package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

type PriorityRuleService struct {
	repo *repository.PriorityRuleRepository
}

func NewPriorityRuleService(repo *repository.PriorityRuleRepository) *PriorityRuleService {
	return &PriorityRuleService{repo: repo}
}

func (s *PriorityRuleService) List(ctx context.Context) ([]models.OrderPriorityRule, error) {
	return s.repo.List(ctx)
}

func (s *PriorityRuleService) UpdateAll(ctx context.Context, req *models.UpdateOrderPriorityRulesRequest, actor string) ([]models.OrderPriorityRule, error) {
	if req == nil || len(req.Rules) == 0 {
		return nil, fmt.Errorf("rules are required")
	}

	seen := make(map[string]struct{}, len(req.Rules))
	for _, rule := range req.Rules {
		key := strings.ToLower(strings.TrimSpace(rule.Key))
		if !isValidPriority(key) {
			return nil, fmt.Errorf("invalid key %q: must be one of p1, p2, p3, p4", rule.Key)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate key %q in payload", key)
		}
		seen[key] = struct{}{}
	}

	return s.repo.UpdateAll(ctx, req.Rules, actor)
}
