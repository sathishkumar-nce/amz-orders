package models

import "time"

type DBBackupRunResponse struct {
	FileName    string    `json:"file_name"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

type DBBackupStatusResponse struct {
	Enabled              bool                `json:"enabled"`
	Running              bool                `json:"running"`
	LocalBackupDirectory string              `json:"local_backup_directory"`
	LastBackup           *DBBackupRunSummary `json:"last_backup,omitempty"`
}

type DBBackupRunSummary struct {
	FileName    string     `json:"file_name"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}
