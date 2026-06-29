package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type DBBackupHandler struct {
	service *service.DBBackupService
}

func NewDBBackupHandler(service *service.DBBackupService) *DBBackupHandler {
	return &DBBackupHandler{service: service}
}

func (h *DBBackupHandler) GetStatus(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "db backup service is not available"})
		return
	}

	c.JSON(http.StatusOK, h.service.Status())
}

func (h *DBBackupHandler) RunBackup(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "db backup service is not available"})
		return
	}

	runCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Hour)
	defer cancel()

	result, err := h.service.RunBackup(runCtx)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == service.ErrDBBackupBusy || err == service.ErrDBBackupDisabled {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *DBBackupHandler) DownloadBackup(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "db backup service is not available"})
		return
	}

	fileName := c.Param("file_name")
	filePath, err := h.service.BackupFilePath(fileName)
	if err != nil {
		statusCode := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrDBBackupDisabled):
			statusCode = http.StatusConflict
		case err.Error() == "backup file not found":
			statusCode = http.StatusNotFound
		case err.Error() == "backup file name is required" || err.Error() == "invalid backup file name":
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.FileAttachment(filePath, fileName)
}
