package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

type AmazonRowHighlightRuleService struct {
	repo *repository.AmazonRowHighlightRuleRepository
}

func NewAmazonRowHighlightRuleService(repo *repository.AmazonRowHighlightRuleRepository) *AmazonRowHighlightRuleService {
	return &AmazonRowHighlightRuleService{repo: repo}
}

var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)

func (s *AmazonRowHighlightRuleService) List(ctx context.Context) ([]models.AmazonRowHighlightRule, error) {
	return s.repo.List(ctx)
}

func (s *AmazonRowHighlightRuleService) UpdateAll(ctx context.Context, rules []models.AmazonRowHighlightRule) error {
	if len(rules) == 0 {
		return fmt.Errorf("at least one row highlight rule is required")
	}

	for _, rule := range rules {
		if err := validateAmazonRowHighlightRule(rule); err != nil {
			return fmt.Errorf("%s: %w", rule.RuleKey, err)
		}
	}

	return s.repo.UpdateAll(ctx, rules)
}

func (s *AmazonRowHighlightRuleService) ResetDefaults(ctx context.Context) error {
	return s.repo.ResetDefaults(ctx)
}

func validateAmazonRowHighlightRule(rule models.AmazonRowHighlightRule) error {
	if strings.TrimSpace(rule.RuleKey) == "" {
		return fmt.Errorf("rule_key is required")
	}
	if strings.TrimSpace(rule.Label) == "" {
		return fmt.Errorf("label is required")
	}

	validFields := map[string]bool{
		"quantity":     true,
		"is_round":     true,
		"priority":     true,
		"payment_done": true,
	}
	if !validFields[rule.FieldName] {
		return fmt.Errorf("field_name must be one of: quantity, is_round, priority, payment_done")
	}

	if rule.Operator != "gt" && rule.Operator != "eq" {
		return fmt.Errorf("operator must be one of: gt, eq")
	}
	if rule.ColorMode != "solid" && rule.ColorMode != "gradient" {
		return fmt.Errorf("color_mode must be one of: solid, gradient")
	}
	if !hexColorPattern.MatchString(strings.TrimSpace(rule.ColorStart)) {
		return fmt.Errorf("color_start must be a hex color like #dbeafe")
	}
	if rule.ColorMode == "gradient" {
		if rule.ColorEnd == nil || !hexColorPattern.MatchString(strings.TrimSpace(*rule.ColorEnd)) {
			return fmt.Errorf("color_end must be a hex color like #fef08a when color_mode is gradient")
		}
	}
	if rule.Operator == "gt" && rule.ThresholdNumber == nil {
		return fmt.Errorf("threshold_number is required for gt rules")
	}
	if rule.Operator == "eq" && (rule.MatchText == nil || strings.TrimSpace(*rule.MatchText) == "") {
		return fmt.Errorf("match_text is required for eq rules")
	}

	return nil
}
