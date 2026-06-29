package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type ShippingDateFilterHandler struct {
	service *service.ShippingDateFilterService
}

func NewShippingDateFilterHandler(service *service.ShippingDateFilterService) *ShippingDateFilterHandler {
	return &ShippingDateFilterHandler{service: service}
}

func (h *ShippingDateFilterHandler) List(c *gin.Context) {
	settings, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	activeFilterKey, err := h.service.GetActiveFilterKey(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filters": settings, "active_filter_key": activeFilterKey})
}

func (h *ShippingDateFilterHandler) UpdateAll(c *gin.Context) {
	var req models.UpdateShippingDateFilterSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.UpdateAll(c.Request.Context(), req.Filters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	activeFilterKey, err := h.service.GetActiveFilterKey(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filters": settings, "active_filter_key": activeFilterKey})
}

func (h *ShippingDateFilterHandler) ResetDefaults(c *gin.Context) {
	if err := h.service.ResetDefaults(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	activeFilterKey, err := h.service.GetActiveFilterKey(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filters": settings, "active_filter_key": activeFilterKey})
}

func (h *ShippingDateFilterHandler) SetActive(c *gin.Context) {
	var req models.UpdateActiveShippingDateFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SetActiveFilterKey(c.Request.Context(), req.FilterKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filters": settings, "active_filter_key": req.FilterKey})
}
