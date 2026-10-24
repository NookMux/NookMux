package dbmigrate

import (
	"context"
	"errors"
	"fmt"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/option"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"gorm.io/gorm"
	"strings"
	"time"
)

func runDBPreMigrateJob(job *DBPreMigrateJob, params DBPreMigrateStartParams) {
	ctx := context.Background()
	if err := runDBPreMigrateJobWithContext(ctx, job, params); err != nil {
		job.markFailed(err)
		return
	}
	job.markSuccess()
}

func runDBPreMigrateJobWithContext(ctx context.Context, job *DBPreMigrateJob, params DBPreMigrateStartParams) error {
	if job == nil {
		return errors.New("job 不能为空")
	}

	targetType, err := infradb.ParseDBTypeFromDSN(params.TargetDSN)
	if err != nil {
		return err
	}

	logDBPreMigrateStart(job, targetType, params)

	targetMainDB, err := prepareTargetMainDB(ctx, params, targetType)
	if err != nil {
		return err
	}
	defer func() { _ = closeGormDB(targetMainDB) }()

	if err := migrateMainDBTables(ctx, job, dbstore.DB, targetMainDB, params); err != nil {
		return err
	}

	targetLogDB, needCloseLogDB, err := prepareTargetLogDB(ctx, job, params, targetMainDB, targetType)
	if err != nil {
		return err
	}
	if needCloseLogDB {
		defer func() { _ = closeGormDB(targetLogDB) }()
	}
	if params.IncludeLogs {
		if err := migrateLogTable(ctx, job, dbstore.LOG_DB, targetLogDB, params); err != nil {
			return err
		}
	}

	if err := fixTargetAutoIncrement(job, targetType, targetMainDB, targetLogDB, needCloseLogDB, params.IncludeLogs); err != nil {
		return err
	}

	job.setStep("完成")
	job.appendLog(fmt.Sprintf("[%s] 预迁移完成", time.Now().Format(time.RFC3339)))
	return nil
}

func logDBPreMigrateStart(job *DBPreMigrateJob, targetType string, params DBPreMigrateStartParams) {
	job.appendLog(fmt.Sprintf("[%s] 预迁移启动：%s -> %s", time.Now().Format(time.RFC3339), job.SourceDBType, targetType))
	job.appendLog(fmt.Sprintf("[%s] 迁移日志：%v；覆盖目标库：%v", time.Now().Format(time.RFC3339), params.IncludeLogs, params.Force))
}

func prepareTargetMainDB(ctx context.Context, params DBPreMigrateStartParams, targetType string) (*gorm.DB, error) {
	targetMainDB, err := openDBByType(params.TargetDSN, targetType)
	if err != nil {
		return nil, fmt.Errorf("连接目标主库失败：%w", err)
	}
	if err := autoMigrateTargetMainSchema(targetMainDB); err != nil {
		_ = closeGormDB(targetMainDB)
		return nil, fmt.Errorf("目标主库建表/迁移失败：%w", err)
	}
	if err := ensureTargetMainDBEmptyOrForce(ctx, targetMainDB, params.Force); err != nil {
		_ = closeGormDB(targetMainDB)
		return nil, err
	}
	return targetMainDB, nil
}

func fixTargetAutoIncrement(job *DBPreMigrateJob, targetType string, mainDB *gorm.DB, logDB *gorm.DB, logDBNeedClose bool, includeLogs bool) error {
	if targetType != infradb.DatabaseTypePostgreSQL && targetType != infradb.DatabaseTypeOpenGauss {
		return nil
	}

	job.setStep("修复 PostgreSQL/openGauss 自增序列")
	updated, err := resetPostgresOwnedSequences(mainDB)
	if err != nil {
		return fmt.Errorf("修复 PostgreSQL 自增序列失败：%w", err)
	}
	job.appendLog(fmt.Sprintf("[%s] PostgreSQL 自增序列已修复：%d 个", time.Now().Format(time.RFC3339), updated))

	if includeLogs && logDBNeedClose {
		updated, err := resetPostgresOwnedSequences(logDB)
		if err != nil {
			return fmt.Errorf("修复 PostgreSQL(日志库) 自增序列失败：%w", err)
		}
		job.appendLog(fmt.Sprintf("[%s] PostgreSQL(日志库) 自增序列已修复：%d 个", time.Now().Format(time.RFC3339), updated))
	}
	return nil
}

func ensureTargetMainDBEmptyOrForce(ctx context.Context, target *gorm.DB, force bool) error {
	if force {
		return nil
	}
	checkModels := []any{
		&userstore.User{},
		&optionstore.Option{},
		&channelstore.Channel{},
		&tokenstore.Token{},
	}
	for _, m := range checkModels {
		has, err := hasAnyRow(ctx, target, m)
		if err != nil {
			return fmt.Errorf("检查目标主库是否为空失败：%w", err)
		}
		if has {
			return errors.New("目标主库不是空库：请使用全新的空数据库，或勾选“覆盖目标库（清空后迁移）”")
		}
	}
	return nil
}

func prepareTargetLogDB(ctx context.Context, job *DBPreMigrateJob, params DBPreMigrateStartParams, targetMainDB *gorm.DB, targetType string) (*gorm.DB, bool, error) {
	if !params.IncludeLogs {
		return targetMainDB, false, nil
	}

	targetLogDSN := strings.TrimSpace(params.TargetLogDSN)
	if targetLogDSN == "" {
		targetLogDSN = strings.TrimSpace(params.TargetDSN)
	}
	logType, err := infradb.ParseDBTypeFromDSN(targetLogDSN)
	if err != nil {
		return nil, false, err
	}
	if logType != targetType {
		return nil, false, errors.New("目标日志库类型必须与目标主库一致（当前仅支持同类型迁移）")
	}
	if strings.TrimSpace(targetLogDSN) == strings.TrimSpace(params.TargetDSN) {
		return targetMainDB, false, nil
	}

	job.appendLog(fmt.Sprintf("[%s] 使用独立的目标日志库", time.Now().Format(time.RFC3339)))
	targetLogDB, err := openDBByType(targetLogDSN, logType)
	if err != nil {
		return nil, false, fmt.Errorf("连接目标日志库失败：%w", err)
	}
	if err := autoMigrateTargetLogSchema(targetLogDB); err != nil {
		_ = closeGormDB(targetLogDB)
		return nil, false, fmt.Errorf("目标日志库建表/迁移失败：%w", err)
	}
	if !params.Force {
		has, err := hasAnyRow(ctx, targetLogDB, &logstore.Log{})
		if err != nil {
			_ = closeGormDB(targetLogDB)
			return nil, false, fmt.Errorf("检查目标日志库是否为空失败：%w", err)
		}
		if has {
			_ = closeGormDB(targetLogDB)
			return nil, false, errors.New("目标日志库不是空库：请使用全新的空数据库，或勾选“覆盖目标库（清空后迁移）”")
		}
	}
	return targetLogDB, true, nil
}

func migrateMainDBTables(ctx context.Context, job *DBPreMigrateJob, src *gorm.DB, dst *gorm.DB, params DBPreMigrateStartParams) error {
	for _, step := range dbPreMigrateMainSteps {
		if err := step.Run(ctx, job, src, dst, params); err != nil {
			return err
		}
	}
	return nil
}

func migrateLogTable(ctx context.Context, job *DBPreMigrateJob, src *gorm.DB, dst *gorm.DB, params DBPreMigrateStartParams) error {
	if src == nil {
		return errors.New("源日志库连接为空")
	}
	if dst == nil {
		return errors.New("目标日志库连接为空")
	}
	return dbPreMigrateLogStep.Run(ctx, job, src, dst, params)
}

func hasAnyRow(ctx context.Context, db *gorm.DB, model any) (bool, error) {
	if db == nil || model == nil {
		return false, errors.New("db/model 不能为空")
	}
	rows, err := db.WithContext(ctx).Model(model).Select("1").Limit(1).Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}
