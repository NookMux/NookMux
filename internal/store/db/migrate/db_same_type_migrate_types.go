package dbmigrate

import (
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
)

type DBSameTypeMigrateJobStatus string

const (
	DBSameTypeMigrateJobStatusRunning DBSameTypeMigrateJobStatus = "running"
	DBSameTypeMigrateJobStatusSuccess DBSameTypeMigrateJobStatus = "success"
	DBSameTypeMigrateJobStatusFailed  DBSameTypeMigrateJobStatus = "failed"
)

type DBSameTypeMigrateStartParams struct {
	TargetDSN    string
	TargetLogDSN string
	IncludeLogs  bool
	Force        bool
	Lang         string `json:"-"`
}

type DBSameTypeMigrateInfo struct {
	MainDBType       string `json:"main_db_type"`
	LogDBType        string `json:"log_db_type"`
	LogDBIsSeparated bool   `json:"log_db_is_separated"`
}

type DBSameTypeMigrateTableProgress struct {
	Name   string `json:"name"`
	Copied int64  `json:"copied"`
	Total  int64  `json:"total"`
}

type DBSameTypeMigrateJob struct {
	ID           string                           `json:"id"`
	Status       DBSameTypeMigrateJobStatus       `json:"status"`
	StartedAt    int64                            `json:"started_at"`
	FinishedAt   int64                            `json:"finished_at,omitempty"`
	SourceDBType string                           `json:"source_db_type"`
	TargetDBType string                           `json:"target_db_type"`
	IncludeLogs  bool                             `json:"include_logs"`
	Force        bool                             `json:"force"`
	CurrentStep  string                           `json:"current_step"`
	Tables       []DBSameTypeMigrateTableProgress `json:"tables"`
	Logs         []string                         `json:"logs"`
	Error        string                           `json:"error,omitempty"`

	lang string
	mu   sync.Mutex
}

func newDBSameTypeMigrateJob(id string, sourceType string, params DBSameTypeMigrateStartParams) *DBSameTypeMigrateJob {
	lang := langOrDefault(params.Lang)
	return &DBSameTypeMigrateJob{
		ID:           id,
		Status:       DBSameTypeMigrateJobStatusRunning,
		StartedAt:    time.Now().Unix(),
		SourceDBType: sourceType,
		TargetDBType: sourceType,
		IncludeLogs:  params.IncludeLogs,
		Force:        params.Force,
		CurrentStep:  i18n.Translate(lang, i18n.MsgDBMigrateStepInit, nil),
		Tables:       make([]DBSameTypeMigrateTableProgress, 0, 32),
		Logs:         make([]string, 0, 64),
		lang:         lang,
	}
}

func (j *DBSameTypeMigrateJob) jobLang() string {
	return langOrDefault(j.lang)
}

func (j *DBSameTypeMigrateJob) setStep(step string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.CurrentStep = step
}

func (j *DBSameTypeMigrateJob) appendLog(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Logs = append(j.Logs, line)
}

func (j *DBSameTypeMigrateJob) setTableProgress(name string, copied int64, total int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.Tables {
		if j.Tables[i].Name == name {
			j.Tables[i].Copied = copied
			j.Tables[i].Total = total
			return
		}
	}
	j.Tables = append(j.Tables, DBSameTypeMigrateTableProgress{
		Name:   name,
		Copied: copied,
		Total:  total,
	})
}

func (j *DBSameTypeMigrateJob) markFailed(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = DBSameTypeMigrateJobStatusFailed
	j.FinishedAt = time.Now().Unix()
	if err != nil {
		j.Error = i18n.ResolveError(langOrDefault(j.lang), err, "")
	}
}

func (j *DBSameTypeMigrateJob) markSuccess() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = DBSameTypeMigrateJobStatusSuccess
	j.FinishedAt = time.Now().Unix()
}

func (j *DBSameTypeMigrateJob) snapshot() DBSameTypeMigrateJob {
	j.mu.Lock()
	defer j.mu.Unlock()

	tablesCopy := make([]DBSameTypeMigrateTableProgress, len(j.Tables))
	copy(tablesCopy, j.Tables)
	logsCopy := make([]string, len(j.Logs))
	copy(logsCopy, j.Logs)

	return DBSameTypeMigrateJob{
		ID:           j.ID,
		Status:       j.Status,
		StartedAt:    j.StartedAt,
		FinishedAt:   j.FinishedAt,
		SourceDBType: j.SourceDBType,
		TargetDBType: j.TargetDBType,
		IncludeLogs:  j.IncludeLogs,
		Force:        j.Force,
		CurrentStep:  j.CurrentStep,
		Tables:       tablesCopy,
		Logs:         logsCopy,
		Error:        j.Error,
	}
}
