package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type InteraktSettingsHandler struct {
	service *service.InteraktSettingsService
}

func NewInteraktSettingsHandler(service *service.InteraktSettingsService) *InteraktSettingsHandler {
	return &InteraktSettingsHandler{service: service}
}

func (h *InteraktSettingsHandler) Get(c *gin.Context) {
	settings, err := h.service.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *InteraktSettingsHandler) Update(c *gin.Context) {
	var req models.UpdateInteraktSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.service.Update(c.Request.Context(), models.InteraktSettings{
		Enabled:      req.Enabled,
		Mode:         req.Mode,
		TemplateName: req.TemplateName,
		TestNumber:   req.TestNumber,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}
