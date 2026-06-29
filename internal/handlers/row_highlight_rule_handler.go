package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type AmazonRowHighlightRuleHandler struct {
	service *service.AmazonRowHighlightRuleService
}

func NewAmazonRowHighlightRuleHandler(service *service.AmazonRowHighlightRuleService) *AmazonRowHighlightRuleHandler {
	return &AmazonRowHighlightRuleHandler{service: service}
}

func (h *AmazonRowHighlightRuleHandler) List(c *gin.Context) {
	rules, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func (h *AmazonRowHighlightRuleHandler) UpdateAll(c *gin.Context) {
	var req models.UpdateAmazonRowHighlightRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.UpdateAll(c.Request.Context(), req.Rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rules, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func (h *AmazonRowHighlightRuleHandler) ResetDefaults(c *gin.Context) {
	if err := h.service.ResetDefaults(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rules, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}
