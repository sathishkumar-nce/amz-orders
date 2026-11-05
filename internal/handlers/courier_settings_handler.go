package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func (h *DirectOrderHandler) courierSettings(ctx context.Context) (*models.CourierSettings, error) {
	cfg := h.dtdcClient.Settings()
	service, _ := cfg["service_type"].(string)
	service = strings.ToUpper(strings.TrimSpace(service))
	if service == "" {
		service = "B2C SMART EXPRESS"
	}
	commodity, _ := cfg["commodity"].(string)
	if commodity == "" {
		commodity = "84"
	}
	modes := []models.CourierServiceMode{}
	seen := map[string]bool{}
	for _, code := range []string{service, "B2C SMART EXPRESS", "B2C EXPRESS", "B2B PREMIUM", "B2C PREMIUM", "B2C PRIORITY"} {
		code = strings.TrimSpace(code)
		if !seen[code] {
			modes = append(modes, models.CourierServiceMode{Code: code, Label: code, Enabled: true})
			seen[code] = true
		}
	}
	return h.repo.GetCourierSettings(ctx, models.CourierSettings{DTDCModes: modes, DTDCDefaultService: service, DTDCDefaultCommodity: commodity, AutoDownloadDTDCLabels: true, DTDCLabelCode: "SHIP_LABEL_4X6"})
}
func validateCourierSettings(settings *models.CourierSettings) error {
	if len(settings.DTDCModes) == 0 || len(settings.DTDCModes) > 100 {
		return fmt.Errorf("configure between 1 and 100 DTDC service modes")
	}
	seen := map[string]bool{}
	defaultEnabled := false
	settings.DTDCDefaultService = strings.ToUpper(strings.TrimSpace(settings.DTDCDefaultService))
	for i := range settings.DTDCModes {
		mode := &settings.DTDCModes[i]
		mode.Code = strings.ToUpper(strings.TrimSpace(mode.Code))
		mode.Label = strings.TrimSpace(mode.Label)
		if mode.Code == "" || len(mode.Code) > 30 || strings.ContainsAny(mode.Code, "\r\n\x00") {
			return fmt.Errorf("each service code must contain 1–30 characters on one line")
		}
		if seen[mode.Code] {
			return fmt.Errorf("duplicate service code: %s", mode.Code)
		}
		seen[mode.Code] = true
		if mode.Label == "" {
			mode.Label = mode.Code
		}
		if len(mode.Label) > 100 {
			return fmt.Errorf("service labels must be 100 characters or shorter")
		}
		if mode.Code == settings.DTDCDefaultService && mode.Enabled {
			defaultEnabled = true
		}
	}
	if !defaultEnabled {
		return fmt.Errorf("the default DTDC service must be enabled")
	}
	settings.DTDCDefaultCommodity = strings.TrimSpace(settings.DTDCDefaultCommodity)
	if settings.DTDCDefaultCommodity == "" || len(settings.DTDCDefaultCommodity) > 100 {
		return fmt.Errorf("a default commodity ID or name is required (maximum 100 characters)")
	}
	switch settings.DTDCLabelCode {
	case "SHIP_LABEL_4X6", "SHIP_LABEL_A4", "SHIP_LABEL_A6":
	default:
		return fmt.Errorf("select a supported DTDC label format")
	}
	return nil
}
func (h *DirectOrderHandler) GetCourierSettings(c *gin.Context) {
	settings, err := h.courierSettings(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, settings)
}
func (h *DirectOrderHandler) UpdateCourierSettings(c *gin.Context) {
	var settings models.CourierSettings
	if c.ShouldBindJSON(&settings) != nil {
		c.JSON(400, gin.H{"error": "Invalid courier settings"})
		return
	}
	if err := validateCourierSettings(&settings); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	saved, err := h.repo.SaveCourierSettings(c.Request.Context(), settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save courier settings"})
		return
	}
	c.JSON(200, saved)
}
