package dbmigrate

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/infra/db"
	"github.com/google/uuid"
	"strings"
	"sync"
)

var (
	dbPreMigrateJobsMu sync.RWMutex
	dbPreMigrateJobs   = make(map[string]*DBPreMigrateJob)
)

func StartDBPreMigrate(params DBPreMigrateStartParams) (string, error) {
	if strings.TrimSpace(params.TargetDSN) == "" {
		return "", errors.New("目标数据库 DSN 不能为空")
	}
	sourceType, err := currentMainDBType()
	if err != nil {
		return "", err
	}
	targetType, err := db.ParseDBTypeFromDSN(params.TargetDSN)
	if err != nil {
		return "", err
	}
	if err := validateDBPreMigrateDirection(sourceType, targetType); err != nil {
		return "", err
	}

	dbPreMigrateJobsMu.Lock()
	defer dbPreMigrateJobsMu.Unlock()
	if hasRunningDBPreMigrateJobLocked() {
		return "", errors.New("已有数据库预迁移任务在运行，请等待其完成后再启动新的任务")
	}

	id := uuid.NewString()
	job := newDBPreMigrateJob(id, sourceType, targetType, params)
	dbPreMigrateJobs[id] = job
	go runDBPreMigrateJob(job, params)
	return id, nil
}

func GetDBPreMigrateJob(id string) (DBPreMigrateJob, bool) {
	dbPreMigrateJobsMu.RLock()
	job := dbPreMigrateJobs[id]
	dbPreMigrateJobsMu.RUnlock()
	if job == nil {
		return DBPreMigrateJob{}, false
	}
	return job.snapshot(), true
}

func GetDBPreMigrateInfo() (DBPreMigrateInfo, error) {
	mainType, err := currentMainDBType()
	if err != nil {
		return DBPreMigrateInfo{}, err
	}
	logType, err := currentLogDBType()
	if err != nil {
		return DBPreMigrateInfo{}, err
	}

	logSeparated := strings.TrimSpace(common.GetEnvOrDefaultString("LOG_SQL_DSN", "")) != ""
	return DBPreMigrateInfo{
		MainDBType:       mainType,
		LogDBType:        logType,
		LogDBIsSeparated: logSeparated,
	}, nil
}

func hasRunningDBPreMigrateJobLocked() bool {
	for _, job := range dbPreMigrateJobs {
		if job == nil {
			continue
		}
		s := job.snapshot()
		if s.Status == DBPreMigrateJobStatusRunning {
			return true
		}
	}
	return false
}
