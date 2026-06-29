package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type PriorityRuleHandler struct {
	service      *service.PriorityRuleService
	orderService *service.OrderService
}

func NewPriorityRuleHandler(service *service.PriorityRuleService, orderService *service.OrderService) *PriorityRuleHandler {
	return &PriorityRuleHandler{
		service:      service,
		orderService: orderService,
	}
}

func (h *PriorityRuleHandler) List(c *gin.Context) {
	rules, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func (h *PriorityRuleHandler) UpdateAll(c *gin.Context) {
	var req models.UpdateOrderPriorityRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rules, err := h.service.UpdateAll(c.Request.Context(), &req, currentActorName(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reprioritizedOrders := 0
	if h.orderService != nil {
		reprioritizedOrders, err = h.orderService.RecalculatePriorities(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"rules":                rules,
		"reprioritized_orders": reprioritizedOrders,
	})
}
