package dbmigrate

import (
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/infra/db"
	"github.com/google/uuid"
	"strings"
	"sync"
)

var (
	dbSameTypeMigrateJobsMu sync.RWMutex
	dbSameTypeMigrateJobs   = make(map[string]*DBSameTypeMigrateJob)
)

func StartDBSameTypeMigrate(params DBSameTypeMigrateStartParams) (string, error) {
	if strings.TrimSpace(params.TargetDSN) == "" {
		return "", i18n.NewError(i18n.MsgDBMigrateTargetDSNRequired)
	}

	// 获取当前主库类型
	sourceType, err := currentMainDBType()
	if err != nil {
		return "", err
	}

	// 解析目标库类型
	targetType, err := db.ParseDBTypeFromDSN(params.TargetDSN)
	if err != nil {
		return "", err
	}

	// 同类型迁移校验：仅允许 PG→PG、MySQL→MySQL
	if err := validateDBSameTypeMigrateDirection(sourceType, targetType); err != nil {
		return "", err
	}

	// 目标库与源库是否相同，在 runner 中通过连接级标识校验（不依赖 DSN 字符串比较）

	dbSameTypeMigrateJobsMu.Lock()
	defer dbSameTypeMigrateJobsMu.Unlock()
	if hasRunningDBSameTypeMigrateJobLocked() {
		return "", i18n.NewError(i18n.MsgDBMigrateSameTypeRunning)
	}

	id := uuid.NewString()
	job := newDBSameTypeMigrateJob(id, sourceType, params)
	dbSameTypeMigrateJobs[id] = job
	go runDBSameTypeMigrateJob(job, params)
	return id, nil
}

func GetDBSameTypeMigrateJob(id string) (DBSameTypeMigrateJob, bool) {
	dbSameTypeMigrateJobsMu.RLock()
	job := dbSameTypeMigrateJobs[id]
	dbSameTypeMigrateJobsMu.RUnlock()
	if job == nil {
		return DBSameTypeMigrateJob{}, false
	}
	return job.snapshot(), true
}

func GetDBSameTypeMigrateInfo() (DBSameTypeMigrateInfo, error) {
	mainType, err := currentMainDBType()
	if err != nil {
		return DBSameTypeMigrateInfo{}, err
	}
	logType, err := currentLogDBType()
	if err != nil {
		return DBSameTypeMigrateInfo{}, err
	}

	logSeparated := strings.TrimSpace(common.GetEnvOrDefaultString("LOG_SQL_DSN", "")) != ""
	return DBSameTypeMigrateInfo{
		MainDBType:       mainType,
		LogDBType:        logType,
		LogDBIsSeparated: logSeparated,
	}, nil
}

func hasRunningDBSameTypeMigrateJobLocked() bool {
	for _, job := range dbSameTypeMigrateJobs {
		if job == nil {
			continue
		}
		s := job.snapshot()
		if s.Status == DBSameTypeMigrateJobStatusRunning {
			return true
		}
	}
	return false
}

func validateDBSameTypeMigrateDirection(sourceType string, targetType string) error {
	if sourceType == "" || targetType == "" {
		return i18n.NewError(i18n.MsgDBMigrateTypeEmpty)
	}
	if sourceType != targetType {
		return i18n.NewError(i18n.MsgDBMigrateSameTypeMismatch)
	}
	switch sourceType {
	case db.DatabaseTypeMySQL, db.DatabaseTypePostgreSQL, db.DatabaseTypeOpenGauss, db.DatabaseTypeDM, db.DatabaseTypeKingbase:
		return nil
	default:
		return i18n.NewError(i18n.MsgDBMigrateSameTypeUnsupported)
	}
}
