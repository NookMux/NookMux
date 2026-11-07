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

func runDBSameTypeMigrateJob(job *DBSameTypeMigrateJob, params DBSameTypeMigrateStartParams) {
	ctx := context.Background()
	if err := runDBSameTypeMigrateJobWithContext(ctx, job, params); err != nil {
		job.markFailed(err)
		return
	}
	job.markSuccess()
}

func runDBSameTypeMigrateJobWithContext(ctx context.Context, job *DBSameTypeMigrateJob, params DBSameTypeMigrateStartParams) error {
	if job == nil {
		return errors.New("job 不能为空")
	}

	targetType, err := infradb.ParseDBTypeFromDSN(params.TargetDSN)
	if err != nil {
		return err
	}

	job.appendLog(fmt.Sprintf("[%s] 同类型迁移启动：%s -> %s", time.Now().Format(time.RFC3339), job.SourceDBType, targetType))
	job.appendLog(fmt.Sprintf("[%s] 迁移日志：%v；覆盖目标库：%v", time.Now().Format(time.RFC3339), params.IncludeLogs, params.Force))

	// ============ 第一阶段：打开并校验所有目标库（P2: 先校验再写数据） ============

	job.setStep("校验目标数据库连接")

	// 连接目标主库（仅连接，不建表）
	targetMainDB, err := openTargetMainDB(job, params, targetType)
	if err != nil {
		return err
	}
	defer func() { _ = closeGormDB(targetMainDB) }()

	// 校验目标主库 != 源主库（P1: 连接级标识，不依赖 DSN 字符串）
	// 在建表之前校验，避免对源库误执行 AutoMigrate
	if err := checkNotSameDB(job, dbstore.DB, targetMainDB, targetType, "主库"); err != nil {
		return err
	}

	// 校验通过后再建表 + 空库检查
	if err := prepareSameTypeTargetMainDBAfterCheck(ctx, job, targetMainDB, params); err != nil {
		return err
	}

	// 连接目标日志库（可选），同样先校验再建表
	targetLogDB, needCloseLogDB, err := openAndCheckTargetLogDB(ctx, job, params, targetMainDB, targetType)
	if err != nil {
		return err
	}
	if needCloseLogDB {
		defer func() { _ = closeGormDB(targetLogDB) }()
	}

	// ============ 第二阶段：所有校验通过，开始复制数据 ============

	if err := migrateSameTypeMainDBTables(ctx, job, dbstore.DB, targetMainDB, params); err != nil {
		return err
	}

	if params.IncludeLogs {
		if err := migrateSameTypeLogTable(ctx, job, dbstore.LOG_DB, targetLogDB, params); err != nil {
			return err
		}
	}

	// PostgreSQL 序列修复
	if err := resetPGFamilyTargetSequences(job, targetType, targetMainDB, targetLogDB, needCloseLogDB, params.IncludeLogs); err != nil {
		return err
	}

	job.setStep("完成")
	job.appendLog(fmt.Sprintf("[%s] 同类型迁移完成", time.Now().Format(time.RFC3339)))
	return nil
}

// checkNotSameDB 通过连接级数据库标识比较两个连接是否指向同一个库
func checkNotSameDB(job *DBSameTypeMigrateJob, src *gorm.DB, dst *gorm.DB, dbType string, label string) error {
	srcID, srcErr := dbstore.GetDBIdentity(src, dbType)
	dstID, dstErr := dbstore.GetDBIdentity(dst, dbType)
	if srcErr != nil || dstErr != nil {
		return fmt.Errorf("无法校验目标%s与源%s是否相同：源=%v 目标=%v（身份校验失败时拒绝继续，以防止误操作）", label, label, srcErr, dstErr)
	}
	// 源库身份复检：openGauss 的实例指纹取自 postmaster 启动时刻，若实例
	// 在两次探测之间重启/主备切换，前后源指纹会不一致——此时目标指纹同样
	// 取自新周期，"不同指纹"不再能证明是两个库，继续迁移可能覆盖正在服务
	// 的库；一律中止并要求重新发起。
	srcID2, srcErr2 := dbstore.GetDBIdentity(src, dbType)
	if srcErr2 != nil {
		return fmt.Errorf("复检源%s实例标识失败：%v（身份校验失败时拒绝继续，以防止误操作）", label, srcErr2)
	}
	if srcID2 != srcID {
		return fmt.Errorf("源%s实例标识在校验期间发生变化（源=%s 复检=%s，疑似实例重启或主备切换），为避免误判请重新发起迁移", label, srcID.String(), srcID2.String())
	}
	if srcID == dstID {
		return fmt.Errorf("目标%s与当前源%s是同一个数据库（源=%s 目标=%s），禁止迁移以避免数据被覆盖",
			label, label, srcID.String(), dstID.String())
	}
	job.appendLog(fmt.Sprintf("[%s] 目标%s与源%s不是同一个数据库（源=%s 目标=%s），校验通过",
		time.Now().Format(time.RFC3339), label, label, srcID.String(), dstID.String()))
	return nil
}

func openTargetMainDB(job *DBSameTypeMigrateJob, params DBSameTypeMigrateStartParams, targetType string) (*gorm.DB, error) {
	job.appendLog(fmt.Sprintf("[%s] 连接目标主库...", time.Now().Format(time.RFC3339)))
	targetMainDB, err := openDBByType(params.TargetDSN, targetType)
	if err != nil {
		return nil, fmt.Errorf("连接目标主库失败：%w", err)
	}
	return targetMainDB, nil
}

func prepareSameTypeTargetMainDBAfterCheck(ctx context.Context, job *DBSameTypeMigrateJob, targetMainDB *gorm.DB, params DBSameTypeMigrateStartParams) error {
	job.appendLog(fmt.Sprintf("[%s] 目标主库建表/迁移 schema...", time.Now().Format(time.RFC3339)))
	if err := autoMigrateTargetMainSchema(targetMainDB); err != nil {
		return fmt.Errorf("目标主库建表/迁移失败：%w", err)
	}
	if err := ensureSameTypeTargetMainDBEmptyOrForce(ctx, targetMainDB, params.Force); err != nil {
		return err
	}
	job.appendLog(fmt.Sprintf("[%s] 目标主库准备完成", time.Now().Format(time.RFC3339)))
	return nil
}

func ensureSameTypeTargetMainDBEmptyOrForce(ctx context.Context, target *gorm.DB, force bool) error {
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
			return errors.New("目标主库不是空库：请使用全新的空数据库，或勾选「覆盖目标库（清空后迁移）」")
		}
	}
	return nil
}

func openAndCheckTargetLogDB(ctx context.Context, job *DBSameTypeMigrateJob, params DBSameTypeMigrateStartParams, targetMainDB *gorm.DB, targetType string) (*gorm.DB, bool, error) {
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
		return nil, false, errors.New("目标日志库类型必须与目标主库一致")
	}
	if strings.TrimSpace(targetLogDSN) == strings.TrimSpace(params.TargetDSN) {
		// 日志库与主库共用同一个目标库，主库已校验通过
		return targetMainDB, false, nil
	}

	job.appendLog(fmt.Sprintf("[%s] 连接独立的目标日志库...", time.Now().Format(time.RFC3339)))
	targetLogDB, err := openDBByType(targetLogDSN, logType)
	if err != nil {
		return nil, false, fmt.Errorf("连接目标日志库失败：%w", err)
	}

	// 先校验是否同一个库，再建表
	if err := checkNotSameDB(job, dbstore.LOG_DB, targetLogDB, targetType, "日志库"); err != nil {
		_ = closeGormDB(targetLogDB)
		return nil, false, err
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
			return nil, false, errors.New("目标日志库不是空库：请使用全新的空数据库，或勾选「覆盖目标库（清空后迁移）」")
		}
	}
	job.appendLog(fmt.Sprintf("[%s] 目标日志库准备完成", time.Now().Format(time.RFC3339)))
	return targetLogDB, true, nil
}

func migrateSameTypeMainDBTables(ctx context.Context, job *DBSameTypeMigrateJob, src *gorm.DB, dst *gorm.DB, params DBSameTypeMigrateStartParams) error {
	for _, step := range dbPreMigrateMainSteps {
		if err := runSameTypeCopyStep(ctx, job, step, src, dst, params); err != nil {
			return err
		}
	}
	return nil
}

func migrateSameTypeLogTable(ctx context.Context, job *DBSameTypeMigrateJob, src *gorm.DB, dst *gorm.DB, params DBSameTypeMigrateStartParams) error {
	if src == nil {
		return errors.New("源日志库连接为空")
	}
	if dst == nil {
		return errors.New("目标日志库连接为空")
	}
	return runSameTypeCopyStep(ctx, job, dbPreMigrateLogStep, src, dst, params)
}

// runSameTypeCopyStep 通过 dbPreMigrateStep 接口执行表复制，并将进度回调到同类型迁移的 job
func runSameTypeCopyStep(ctx context.Context, job *DBSameTypeMigrateJob, step dbPreMigrateStep, src *gorm.DB, dst *gorm.DB, params DBSameTypeMigrateStartParams) error {
	job.setStep("迁移表：" + step.Name())
	job.appendLog(fmt.Sprintf("[%s] 开始迁移表 %s", time.Now().Format(time.RFC3339), step.Name()))

	// 使用一个临时的 DBPreMigrateJob 来接收 step.Run 的进度回调
	tmpJob := newDBPreMigrateJob("", job.SourceDBType, job.TargetDBType, DBPreMigrateStartParams{Force: params.Force})
	if err := step.Run(ctx, tmpJob, src, dst, DBPreMigrateStartParams{Force: params.Force}); err != nil {
		job.appendLog(fmt.Sprintf("[%s] 迁移表 %s 失败：%v", time.Now().Format(time.RFC3339), step.Name(), err))
		return err
	}

	// 将临时 job 的进度同步到同类型迁移的 job
	snap := tmpJob.snapshot()
	for _, t := range snap.Tables {
		job.setTableProgress(t.Name, t.Copied, t.Total)
	}
	job.appendLog(fmt.Sprintf("[%s] 迁移表 %s 完成", time.Now().Format(time.RFC3339), step.Name()))
	return nil
}
