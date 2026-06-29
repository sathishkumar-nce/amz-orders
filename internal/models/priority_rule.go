package models

import "time"

type OrderPriorityRule struct {
	Key       string    `json:"key"`
	SKUValues []string  `json:"sku_values"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateOrderPriorityRulesRequest struct {
	Rules []OrderPriorityRuleInput `json:"rules" binding:"required"`
}

type OrderPriorityRuleInput struct {
	Key       string   `json:"key" binding:"required"`
	SKUValues []string `json:"sku_values"`
}
