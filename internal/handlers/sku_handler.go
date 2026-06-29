package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

type SKUHandler struct {
	skuCSVPath string
}

type skuFileMetadata struct {
	UploadedFileName string `json:"uploaded_file_name"`
	UploadedAt       string `json:"uploaded_at"`
}

type scheduleWeightParseItem struct {
	Position    int     `json:"position"`
	SKU         string  `json:"sku"`
	Quantity    int     `json:"quantity"`
	UnitWeight  float64 `json:"unit_weight_kg"`
	TotalWeight float64 `json:"total_weight_kg"`
	Found       bool    `json:"found"`
}

func NewSKUHandler(skuCSVPath string) *SKUHandler {
	if skuCSVPath == "" {
		skuCSVPath = "./SKU_V8.csv"
	}
	return &SKUHandler{
		skuCSVPath: skuCSVPath,
	}
}

func (h *SKUHandler) metadataPath() string {
	return h.skuCSVPath + ".meta.json"
}

func (h *SKUHandler) readMetadata() (*skuFileMetadata, error) {
	data, err := os.ReadFile(h.metadataPath())
	if err != nil {
		return nil, err
	}

	var metadata skuFileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (h *SKUHandler) writeMetadata(metadata *skuFileMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(h.metadataPath(), data, 0o644)
}

func copyFileContents(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		return err
	}

	if err := dstFile.Close(); err != nil {
		return err
	}

	return nil
}

// GetSKUFileInfo returns information about the current SKU CSV file
func (h *SKUHandler) GetSKUFileInfo(c *gin.Context) {
	fileInfo, err := os.Stat(h.skuCSVPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "SKU CSV file not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get file info: %v", err),
		})
		return
	}

	// Get SKU count from mapper
	skuMapper := utils.GetSKUMapper()
	skuCount := skuMapper.GetDataCount()
	metadata, err := h.readMetadata()
	if err != nil && !os.IsNotExist(err) {
		log.Printf("⚠️  Failed to read SKU mapper metadata: %v", err)
	}

	sourceFileName := filepath.Base(h.skuCSVPath)
	uploadedAt := fileInfo.ModTime().Format(time.RFC3339)
	if metadata != nil {
		if strings.TrimSpace(metadata.UploadedFileName) != "" {
			sourceFileName = metadata.UploadedFileName
		}
		if strings.TrimSpace(metadata.UploadedAt) != "" {
			uploadedAt = metadata.UploadedAt
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"file_name":        filepath.Base(h.skuCSVPath),
		"source_file_name": sourceFileName,
		"file_path":        h.skuCSVPath,
		"file_size":        fileInfo.Size(),
		"updated_at":       fileInfo.ModTime().Format(time.RFC3339),
		"uploaded_at":      uploadedAt,
		"sku_count":        skuCount,
		"status":           "active",
	})
}

// DownloadSKUFile allows downloading the current SKU CSV file
func (h *SKUHandler) DownloadSKUFile(c *gin.Context) {
	// Check if file exists
	if _, err := os.Stat(h.skuCSVPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "SKU CSV file not found",
		})
		return
	}

	// Open the file
	file, err := os.Open(h.skuCSVPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to open file: %v", err),
		})
		return
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get file info: %v", err),
		})
		return
	}

	// Set headers for file download
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(h.skuCSVPath)))
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream file to response
	if _, err := io.Copy(c.Writer, file); err != nil {
		log.Printf("Error streaming file: %v", err)
	}
}

// UpdateSKUFile uploads a new SKU CSV file
func (h *SKUHandler) UpdateSKUFile(c *gin.Context) {
	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing 'file' in request. Please upload a CSV file.",
		})
		return
	}

	// Validate file extension
	if filepath.Ext(file.Filename) != ".csv" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Only CSV files are allowed.",
		})
		return
	}

	// Validate file size (max 10MB)
	if file.Size > 10<<20 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File size exceeds maximum limit of 10MB.",
		})
		return
	}

	tempPath := h.skuCSVPath + ".uploading"
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to save file: %v", err),
		})
		return
	}
	defer func() {
		if _, err := os.Stat(tempPath); err == nil {
			_ = os.Remove(tempPath)
		}
	}()

	// Validate the uploaded CSV by loading it before swapping it into place.
	skuMapper := utils.GetSKUMapper()
	parsedData, err := utils.LoadSKUDataFromCSV(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Uploaded file failed validation: %v", err),
			"warning": "The active SKU mapper file was not replaced.",
		})
		return
	}

	backupPath := ""
	if _, err := os.Stat(h.skuCSVPath); err == nil {
		backupPath = h.skuCSVPath + ".backup." + time.Now().Format("20060102_150405")
		if err := copyFileContents(h.skuCSVPath, backupPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create backup of current SKU file: %v", err),
			})
			return
		}
		log.Printf("Created backup: %s", backupPath)
	}

	if err := copyFileContents(tempPath, h.skuCSVPath); err != nil {
		if backupPath != "" {
			_ = copyFileContents(backupPath, h.skuCSVPath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to activate uploaded SKU file: %v", err),
		})
		return
	}

	// Reuse the already-validated parsed data instead of parsing the full CSV again.
	skuMapper.ReplaceData(parsedData)
	if err := h.writeMetadata(&skuFileMetadata{
		UploadedFileName: file.Filename,
		UploadedAt:       time.Now().Format(time.RFC3339),
	}); err != nil {
		log.Printf("⚠️  Failed to write SKU mapper metadata: %v", err)
	}

	log.Printf("✅ SKU CSV file updated: %s (%d bytes, %d SKUs loaded)",
		file.Filename, file.Size, skuMapper.GetDataCount())

	c.JSON(http.StatusOK, gin.H{
		"message":     "SKU CSV file updated successfully",
		"file_name":   file.Filename,
		"file_size":   file.Size,
		"sku_count":   skuMapper.GetDataCount(),
		"uploaded_at": time.Now().Format(time.RFC3339),
	})
}

func (h *SKUHandler) ParseScheduleWeights(c *gin.Context) {
	var req struct {
		RawText string `json:"raw_text"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "raw_text is required",
		})
		return
	}

	rawText := strings.TrimSpace(req.RawText)
	if rawText == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "raw_text is required",
		})
		return
	}

	skuMapper := utils.GetSKUMapper()
	pattern := regexp.MustCompile(`(?is)SKU:\s*([A-Z0-9-]+)\s+Quantity:\s*(\d+)`)
	matches := pattern.FindAllStringSubmatch(rawText, -1)

	items := make([]scheduleWeightParseItem, 0, len(matches))
	totalWeightKg := 0.0
	foundCount := 0

	for index, match := range matches {
		sku := strings.ToUpper(strings.TrimSpace(match[1]))
		quantity, err := strconv.Atoi(strings.TrimSpace(match[2]))
		if err != nil || quantity <= 0 {
			quantity = 1
		}

		item := scheduleWeightParseItem{
			Position: index + 1,
			SKU:      sku,
			Quantity: quantity,
		}

		if skuData, exists := skuMapper.GetBySKU(sku); exists {
			item.Found = true
			item.UnitWeight = skuData.WeightKg
			item.TotalWeight = skuData.WeightKg * float64(quantity)
			totalWeightKg += item.TotalWeight
			foundCount++
		}

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"items":             items,
		"matched_count":     foundCount,
		"unmatched_count":   len(items) - foundCount,
		"total_items":       len(items),
		"total_weight_kg":   totalWeightKg,
		"parser_version":    "amazon_schedule_v1",
		"source_line_count": strings.Count(rawText, "\n") + 1,
	})
}
