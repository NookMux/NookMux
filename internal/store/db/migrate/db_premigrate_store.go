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
	dbPreMigrateJobsMu sync.RWMutex
	dbPreMigrateJobs   = make(map[string]*DBPreMigrateJob)
)

func StartDBPreMigrate(params DBPreMigrateStartParams) (string, error) {
	if strings.TrimSpace(params.TargetDSN) == "" {
		return "", i18n.NewError(i18n.MsgDBMigrateTargetDSNRequired)
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
		return "", i18n.NewError(i18n.MsgDBMigratePreRunning)
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
