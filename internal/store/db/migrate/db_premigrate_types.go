package dbmigrate

import (
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/i18n"
)

type DBPreMigrateJobStatus string

const (
	DBPreMigrateJobStatusRunning DBPreMigrateJobStatus = "running"
	DBPreMigrateJobStatusSuccess DBPreMigrateJobStatus = "success"
	DBPreMigrateJobStatusFailed  DBPreMigrateJobStatus = "failed"
)

type DBPreMigrateStartParams struct {
	TargetDSN    string
	TargetLogDSN string
	IncludeLogs  bool
	Force        bool
	Lang         string `json:"-"`
}

type DBPreMigrateInfo struct {
	MainDBType       string `json:"main_db_type"`
	LogDBType        string `json:"log_db_type"`
	LogDBIsSeparated bool   `json:"log_db_is_separated"`
}

type DBPreMigrateTableProgress struct {
	Name   string `json:"name"`
	Copied int64  `json:"copied"`
	Total  int64  `json:"total"`
}

type DBPreMigrateJob struct {
	ID           string                      `json:"id"`
	Status       DBPreMigrateJobStatus       `json:"status"`
	StartedAt    int64                       `json:"started_at"`
	FinishedAt   int64                       `json:"finished_at,omitempty"`
	SourceDBType string                      `json:"source_db_type"`
	TargetDBType string                      `json:"target_db_type"`
	IncludeLogs  bool                        `json:"include_logs"`
	Force        bool                        `json:"force"`
	CurrentStep  string                      `json:"current_step"`
	Tables       []DBPreMigrateTableProgress `json:"tables"`
	Logs         []string                    `json:"logs"`
	Error        string                      `json:"error,omitempty"`

	lang string
	mu   sync.Mutex
}

func newDBPreMigrateJob(id string, sourceType string, targetType string, params DBPreMigrateStartParams) *DBPreMigrateJob {
	lang := langOrDefault(params.Lang)
	now := time.Now().Unix()
	return &DBPreMigrateJob{
		ID:           id,
		Status:       DBPreMigrateJobStatusRunning,
		StartedAt:    now,
		SourceDBType: sourceType,
		TargetDBType: targetType,
		IncludeLogs:  params.IncludeLogs,
		Force:        params.Force,
		CurrentStep:  i18n.Translate(lang, i18n.MsgDBMigrateStepInit, nil),
		Tables:       make([]DBPreMigrateTableProgress, 0, 32),
		Logs:         make([]string, 0, 64),
		lang:         lang,
	}
}

func (j *DBPreMigrateJob) jobLang() string {
	return langOrDefault(j.lang)
}

func (j *DBPreMigrateJob) setStep(step string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.CurrentStep = step
}

func (j *DBPreMigrateJob) appendLog(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Logs = append(j.Logs, line)
}

func (j *DBPreMigrateJob) setTableProgress(name string, copied int64, total int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.Tables {
		if j.Tables[i].Name == name {
			j.Tables[i].Copied = copied
			j.Tables[i].Total = total
			return
		}
	}
	j.Tables = append(j.Tables, DBPreMigrateTableProgress{
		Name:   name,
		Copied: copied,
		Total:  total,
	})
}

func (j *DBPreMigrateJob) markFailed(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = DBPreMigrateJobStatusFailed
	j.FinishedAt = time.Now().Unix()
	if err != nil {
		j.Error = i18n.ResolveError(langOrDefault(j.lang), err, "")
	}
}

func (j *DBPreMigrateJob) markSuccess() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = DBPreMigrateJobStatusSuccess
	j.FinishedAt = time.Now().Unix()
}

func (j *DBPreMigrateJob) snapshot() DBPreMigrateJob {
	j.mu.Lock()
	defer j.mu.Unlock()

	tablesCopy := make([]DBPreMigrateTableProgress, len(j.Tables))
	copy(tablesCopy, j.Tables)
	logsCopy := make([]string, len(j.Logs))
	copy(logsCopy, j.Logs)

	return DBPreMigrateJob{
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
