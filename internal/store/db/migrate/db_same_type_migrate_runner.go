package dbmigrate

import (
	"context"
	"errors"
	"github.com/NookMux/NookMux/internal/i18n"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/log"
	"gorm.io/gorm"
	"strings"
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

	appendMigrateLog(job, i18n.MsgDBMigrateLogSameStart, map[string]any{"Source": job.SourceDBType, "Target": targetType})
	appendMigrateLog(job, i18n.MsgDBMigrateLogOptions, map[string]any{"IncludeLogs": params.IncludeLogs, "Force": params.Force})

	// ============ 第一阶段：打开并校验所有目标库（P2: 先校验再写数据） ============

	setMigrateStep(job, i18n.MsgDBMigrateStepValidateTargetConn)

	// 连接目标主库（仅连接，不建表）
	targetMainDB, err := openTargetMainDB(job, params, targetType)
	if err != nil {
		return err
	}
	defer func() { _ = closeGormDB(targetMainDB) }()

	// 校验目标主库 != 源主库（P1: 连接级标识，不依赖 DSN 字符串）
	// 在建表之前校验，避免对源库误执行 AutoMigrate
	if err := checkNotSameDB(job, dbstore.DB, targetMainDB, targetType, i18n.MsgDBMigrateMainLabel); err != nil {
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

	setMigrateStep(job, i18n.MsgDBMigrateStepDone)
	appendMigrateLog(job, i18n.MsgDBMigrateLogSameDone)
	return nil
}

// checkNotSameDB 通过连接级数据库标识比较两个连接是否指向同一个库
func checkNotSameDB(job *DBSameTypeMigrateJob, src *gorm.DB, dst *gorm.DB, dbType string, labelKey string) error {
	label := i18n.Translate(job.jobLang(), labelKey, nil)
	srcID, srcErr := dbstore.GetDBIdentity(src, dbType)
	dstID, dstErr := dbstore.GetDBIdentity(dst, dbType)
	if srcErr != nil || dstErr != nil {
		return i18n.NewError(i18n.MsgDBMigrateVerifySameDBFail, map[string]any{
			"Label":     label,
			"SourceErr": srcErr,
			"TargetErr": dstErr,
		})
	}
	// 源库身份复检：openGauss 的实例指纹取自 postmaster 启动时刻，若实例
	// 在两次探测之间重启/主备切换，前后源指纹会不一致——此时目标指纹同样
	// 取自新周期，"不同指纹"不再能证明是两个库，继续迁移可能覆盖正在服务
	// 的库；一律中止并要求重新发起。
	srcID2, srcErr2 := dbstore.GetDBIdentity(src, dbType)
	if srcErr2 != nil {
		return i18n.NewError(i18n.MsgDBMigrateRecheckSrcIDFail, map[string]any{"Label": label}).WithCause(srcErr2)
	}
	if srcID2 != srcID {
		return i18n.NewError(i18n.MsgDBMigrateSrcIDChanged, map[string]any{
			"Label":     label,
			"InitialID": srcID.String(),
			"RecheckID": srcID2.String(),
		})
	}
	if srcID == dstID {
		return i18n.NewError(i18n.MsgDBMigrateSameDB, map[string]any{
			"Label":    label,
			"SourceID": srcID.String(),
			"TargetID": dstID.String(),
		})
	}
	appendMigrateLog(job, i18n.MsgDBMigrateLogNotSameDB, map[string]any{
		"Label":    label,
		"SourceID": srcID.String(),
		"TargetID": dstID.String(),
	})
	return nil
}

func openTargetMainDB(job *DBSameTypeMigrateJob, params DBSameTypeMigrateStartParams, targetType string) (*gorm.DB, error) {
	appendMigrateLog(job, i18n.MsgDBMigrateLogConnMain)
	targetMainDB, err := openDBByType(params.TargetDSN, targetType)
	if err != nil {
		return nil, i18n.NewError(i18n.MsgDBMigrateConnectTargetMainFail).WithCause(err)
	}
	return targetMainDB, nil
}

func prepareSameTypeTargetMainDBAfterCheck(ctx context.Context, job *DBSameTypeMigrateJob, targetMainDB *gorm.DB, params DBSameTypeMigrateStartParams) error {
	appendMigrateLog(job, i18n.MsgDBMigrateLogSchemaMain)
	if err := autoMigrateTargetMainSchema(targetMainDB); err != nil {
		return i18n.NewError(i18n.MsgDBMigrateSchemaTargetMainFail).WithCause(err)
	}
	if err := ensureTargetMainDBEmptyOrForce(ctx, targetMainDB, params.Force); err != nil {
		return err
	}
	appendMigrateLog(job, i18n.MsgDBMigrateLogMainReady)
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
		return nil, false, i18n.NewError(i18n.MsgDBMigrateTargetLogTypeMismatch)
	}
	if strings.TrimSpace(targetLogDSN) == strings.TrimSpace(params.TargetDSN) {
		// 日志库与主库共用同一个目标库，主库已校验通过
		return targetMainDB, false, nil
	}

	appendMigrateLog(job, i18n.MsgDBMigrateLogConnLog)
	targetLogDB, err := openDBByType(targetLogDSN, logType)
	if err != nil {
		return nil, false, i18n.NewError(i18n.MsgDBMigrateConnectTargetLogFail).WithCause(err)
	}

	// 先校验是否同一个库，再建表
	if err := checkNotSameDB(job, dbstore.LOG_DB, targetLogDB, targetType, i18n.MsgDBMigrateLogLabel); err != nil {
		_ = closeGormDB(targetLogDB)
		return nil, false, err
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
	appendMigrateLog(job, i18n.MsgDBMigrateLogLogReady)
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
		return i18n.NewError(i18n.MsgDBMigrateSourceLogDBEmpty)
	}
	if dst == nil {
		return i18n.NewError(i18n.MsgDBMigrateTargetLogDBEmpty)
	}
	return runSameTypeCopyStep(ctx, job, dbPreMigrateLogStep, src, dst, params)
}

// runSameTypeCopyStep 通过 dbPreMigrateStep 接口执行表复制，并将进度回调到同类型迁移的 job
func runSameTypeCopyStep(ctx context.Context, job *DBSameTypeMigrateJob, step dbPreMigrateStep, src *gorm.DB, dst *gorm.DB, params DBSameTypeMigrateStartParams) error {
	setMigrateStep(job, i18n.MsgDBMigrateStepMigratingTable, map[string]any{"Table": step.Name()})
	appendMigrateLog(job, i18n.MsgDBMigrateLogTableStart, map[string]any{"Table": step.Name()})

	// 使用一个临时的 DBPreMigrateJob 来接收 step.Run 的进度回调
	tmpJob := newDBPreMigrateJob("", job.SourceDBType, job.TargetDBType, DBPreMigrateStartParams{Force: params.Force, Lang: job.jobLang()})
	if err := step.Run(ctx, tmpJob, src, dst, DBPreMigrateStartParams{Force: params.Force, Lang: job.jobLang()}); err != nil {
		appendMigrateLog(job, i18n.MsgDBMigrateLogTableFailed, map[string]any{
			"Table": step.Name(),
			"Error": i18n.ResolveError(job.jobLang(), err, ""),
		})
		return err
	}

	// 将临时 job 的进度同步到同类型迁移的 job
	snap := tmpJob.snapshot()
	for _, t := range snap.Tables {
		job.setTableProgress(t.Name, t.Copied, t.Total)
	}
	appendMigrateLog(job, i18n.MsgDBMigrateLogTableDone, map[string]any{"Table": step.Name()})
	return nil
}
