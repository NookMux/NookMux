package dbmigrate

import (
	"context"
	"errors"
	"github.com/NookMux/NookMux/internal/i18n"
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

	if err := resetPGFamilyTargetSequences(job, targetType, targetMainDB, targetLogDB, needCloseLogDB, params.IncludeLogs); err != nil {
		return err
	}

	setMigrateStep(job, i18n.MsgDBMigrateStepDone)
	appendMigrateLog(job, i18n.MsgDBMigrateLogPreDone)
	return nil
}

func logDBPreMigrateStart(job *DBPreMigrateJob, targetType string, params DBPreMigrateStartParams) {
	appendMigrateLog(job, i18n.MsgDBMigrateLogPreStart, map[string]any{"Source": job.SourceDBType, "Target": targetType})
	appendMigrateLog(job, i18n.MsgDBMigrateLogOptions, map[string]any{"IncludeLogs": params.IncludeLogs, "Force": params.Force})
}

func prepareTargetMainDB(ctx context.Context, params DBPreMigrateStartParams, targetType string) (*gorm.DB, error) {
	targetMainDB, err := openDBByType(params.TargetDSN, targetType)
	if err != nil {
		return nil, i18n.NewError(i18n.MsgDBMigrateConnectTargetMainFail).WithCause(err)
	}
	if err := autoMigrateTargetMainSchema(targetMainDB); err != nil {
		_ = closeGormDB(targetMainDB)
		return nil, i18n.NewError(i18n.MsgDBMigrateSchemaTargetMainFail).WithCause(err)
	}
	if err := ensureTargetMainDBEmptyOrForce(ctx, targetMainDB, params.Force); err != nil {
		_ = closeGormDB(targetMainDB)
		return nil, err
	}
	return targetMainDB, nil
}

// migrateJobRecorder 抽象预迁移与同类型迁移 job 的进度记录能力，使两个
// runner 共享同一份 PostgreSQL 系序列重置与本地化日志实现。
type migrateJobRecorder interface {
	setStep(step string)
	appendLog(line string)
	jobLang() string
}

func langOrDefault(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return i18n.DefaultLang
	}
	return lang
}

func setMigrateStep(job migrateJobRecorder, key string, args ...map[string]any) {
	if job == nil {
		return
	}
	var p map[string]any
	if len(args) > 0 {
		p = args[0]
	}
	job.setStep(i18n.Translate(job.jobLang(), key, p))
}

func appendMigrateLog(job migrateJobRecorder, key string, args ...map[string]any) {
	if job == nil {
		return
	}
	p := map[string]any{"Time": time.Now().Format(time.RFC3339)}
	if len(args) > 0 {
		for k, v := range args[0] {
			p[k] = v
		}
	}
	job.appendLog(i18n.Translate(job.jobLang(), key, p))
}

// resetPGFamilyTargetSequences 对 PostgreSQL 系内核（PG/openGauss/金仓）
// 目标库执行自增序列 setval 重置：其序列不随显式主键插值前进，拷贝迁移后
// 必须修复，否则迁移完成后首次自增写入主键冲突；MySQL AUTO_INCREMENT 与
// DM IDENTITY 自动跟随显式插值，无需处理。预迁移与同类型迁移两条链路
// 共用本实现。
func resetPGFamilyTargetSequences(job migrateJobRecorder, targetType string, mainDB *gorm.DB, logDB *gorm.DB, logDBNeedClose bool, includeLogs bool) error {
	if targetType != infradb.DatabaseTypePostgreSQL && targetType != infradb.DatabaseTypeOpenGauss && targetType != infradb.DatabaseTypeKingbase {
		return nil
	}

	setMigrateStep(job, i18n.MsgDBMigrateStepFixPGSequences)
	updated, err := resetPostgresOwnedSequences(mainDB)
	if err != nil {
		return i18n.NewError(i18n.MsgDBMigrateFixPGSeqMainFail).WithCause(err)
	}
	appendMigrateLog(job, i18n.MsgDBMigrateLogPGSeqMainDone, map[string]any{"Count": updated})

	if includeLogs && logDBNeedClose {
		updated, err := resetPostgresOwnedSequences(logDB)
		if err != nil {
			return i18n.NewError(i18n.MsgDBMigrateFixPGSeqLogFail).WithCause(err)
		}
		appendMigrateLog(job, i18n.MsgDBMigrateLogPGSeqLogDone, map[string]any{"Count": updated})
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
			return i18n.NewError(i18n.MsgDBMigrateCheckEmptyMainFail).WithCause(err)
		}
		if has {
			return i18n.NewError(i18n.MsgDBMigrateTargetMainNotEmpty)
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
		return nil, false, i18n.NewError(i18n.MsgDBMigrateTargetLogTypeMismatch)
	}
	if strings.TrimSpace(targetLogDSN) == strings.TrimSpace(params.TargetDSN) {
		return targetMainDB, false, nil
	}

	appendMigrateLog(job, i18n.MsgDBMigrateLogSeparateLog)
	targetLogDB, err := openDBByType(targetLogDSN, logType)
	if err != nil {
		return nil, false, i18n.NewError(i18n.MsgDBMigrateConnectTargetLogFail).WithCause(err)
	}
	if err := autoMigrateTargetLogSchema(targetLogDB); err != nil {
		_ = closeGormDB(targetLogDB)
		return nil, false, i18n.NewError(i18n.MsgDBMigrateSchemaTargetLogFail).WithCause(err)
	}
	if !params.Force {
		has, err := hasAnyRow(ctx, targetLogDB, &logstore.Log{})
		if err != nil {
			_ = closeGormDB(targetLogDB)
			return nil, false, i18n.NewError(i18n.MsgDBMigrateCheckEmptyLogFail).WithCause(err)
		}
		if has {
			_ = closeGormDB(targetLogDB)
			return nil, false, i18n.NewError(i18n.MsgDBMigrateTargetLogNotEmpty)
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
		return i18n.NewError(i18n.MsgDBMigrateSourceLogDBEmpty)
	}
	if dst == nil {
		return i18n.NewError(i18n.MsgDBMigrateTargetLogDBEmpty)
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
