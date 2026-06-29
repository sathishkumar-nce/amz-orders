package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

var (
	ErrDBBackupDisabled = errors.New("db backup service is not configured")
	ErrDBBackupBusy     = errors.New("db backup is already running")
)

type DBBackupService struct {
	databaseURL    string
	pgDumpPath     string
	localBackupDir string
	mu             sync.Mutex
	running        bool
	lastBackup     *models.DBBackupRunSummary
}

func NewDBBackupService(
	databaseURL string,
	pgDumpPath string,
	localBackupDir string,
) *DBBackupService {
	return &DBBackupService{
		databaseURL:    databaseURL,
		pgDumpPath:     pgDumpPath,
		localBackupDir: localBackupDir,
	}
}

func (s *DBBackupService) Enabled() bool {
	return s != nil && s.databaseURL != "" && s.pgDumpPath != "" && s.localBackupDir != ""
}

func (s *DBBackupService) Status() *models.DBBackupStatusResponse {
	if s == nil {
		return &models.DBBackupStatusResponse{
			Enabled: false,
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var lastBackup *models.DBBackupRunSummary
	if s.lastBackup != nil {
		copyValue := *s.lastBackup
		lastBackup = &copyValue
	}

	return &models.DBBackupStatusResponse{
		Enabled:              s.Enabled(),
		Running:              s.running,
		LocalBackupDirectory: s.localBackupDir,
		LastBackup:           lastBackup,
	}
}

func (s *DBBackupService) RunBackup(ctx context.Context) (*models.DBBackupRunResponse, error) {
	if !s.Enabled() {
		return nil, ErrDBBackupDisabled
	}

	startedAt := time.Now()

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrDBBackupBusy
	}
	s.running = true
	s.lastBackup = &models.DBBackupRunSummary{
		StartedAt: &startedAt,
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if err := os.MkdirAll(s.localBackupDir, 0o755); err != nil {
		s.markFailed(startedAt, "", err)
		return nil, fmt.Errorf("create local backup dir: %w", err)
	}

	fileName := fmt.Sprintf("amz_orders_%s.sql", startedAt.Format("20060102_150405"))
	localPath := filepath.Join(s.localBackupDir, fileName)

	args := []string{
		"--format=plain",
		"--no-owner",
		"--no-privileges",
		"--file", localPath,
		s.databaseURL,
	}
	cmd := exec.CommandContext(ctx, s.pgDumpPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		runErr := fmt.Errorf("pg_dump failed: %w (%s)", err, string(output))
		s.markFailed(startedAt, fileName, runErr)
		return nil, runErr
	}
	completedAt := time.Now()
	s.markSucceeded(startedAt, completedAt, fileName)

	log.Printf("✅ Local DB backup created: %s", fileName)
	return &models.DBBackupRunResponse{
		FileName:    fileName,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}, nil
}

func (s *DBBackupService) BackupFilePath(fileName string) (string, error) {
	if s == nil || !s.Enabled() {
		return "", ErrDBBackupDisabled
	}
	if fileName == "" {
		return "", fmt.Errorf("backup file name is required")
	}
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || fileName != filepath.Base(fileName) {
		return "", fmt.Errorf("invalid backup file name")
	}

	fullPath := filepath.Join(s.localBackupDir, fileName)
	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("backup file not found")
		}
		return "", fmt.Errorf("inspect backup file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("backup file path is a directory")
	}

	return fullPath, nil
}

func (s *DBBackupService) markFailed(startedAt time.Time, fileName string, err error) {
	completedAt := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastBackup = &models.DBBackupRunSummary{
		FileName:    fileName,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Error:       err.Error(),
	}
}

func (s *DBBackupService) markSucceeded(startedAt, completedAt time.Time, fileName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastBackup = &models.DBBackupRunSummary{
		FileName:    fileName,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
	}
}
