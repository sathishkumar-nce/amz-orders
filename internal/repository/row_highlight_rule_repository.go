package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type AmazonRowHighlightRuleRepository struct {
	pool *pgxpool.Pool
}

func NewAmazonRowHighlightRuleRepository(pool *pgxpool.Pool) *AmazonRowHighlightRuleRepository {
	return &AmazonRowHighlightRuleRepository{pool: pool}
}

func DefaultAmazonRowHighlightRules() []models.AmazonRowHighlightRule {
	quantityThreshold := 1.0
	paymentThreshold := 4000.0
	trueValue := "true"
	p1 := "p1"
	p2 := "p2"
	p3 := "p3"
	return []models.AmazonRowHighlightRule{
		{RuleKey: "priority_p1", Label: "Priority P1", FieldName: "priority", Operator: "eq", MatchText: &p1, ColorMode: "gradient", ColorStart: "#fecaca", ColorEnd: stringPtr("#fef08a"), SortOrder: 10, Enabled: true},
		{RuleKey: "priority_p2", Label: "Priority P2", FieldName: "priority", Operator: "eq", MatchText: &p2, ColorMode: "gradient", ColorStart: "#fed7aa", ColorEnd: stringPtr("#fef9c3"), SortOrder: 20, Enabled: true},
		{RuleKey: "priority_p3", Label: "Priority P3", FieldName: "priority", Operator: "eq", MatchText: &p3, ColorMode: "gradient", ColorStart: "#fbcfe8", ColorEnd: stringPtr("#fef9c3"), SortOrder: 30, Enabled: true},
		{RuleKey: "payment_done_high", Label: "Payment Done Above", FieldName: "payment_done", Operator: "gt", ThresholdNumber: &paymentThreshold, ColorMode: "solid", ColorStart: "#dcfce7", SortOrder: 40, Enabled: true},
		{RuleKey: "quantity_above_one", Label: "Quantity Above One", FieldName: "quantity", Operator: "gt", ThresholdNumber: &quantityThreshold, ColorMode: "solid", ColorStart: "#dbeafe", SortOrder: 50, Enabled: true},
		{RuleKey: "is_round_true", Label: "Round Product", FieldName: "is_round", Operator: "eq", MatchText: &trueValue, ColorMode: "solid", ColorStart: "#ede9fe", SortOrder: 60, Enabled: true},
	}
}

func (r *AmazonRowHighlightRuleRepository) List(ctx context.Context) ([]models.AmazonRowHighlightRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rule_key, label, field_name, operator, threshold_number, match_text, color_mode, color_start, color_end, sort_order, enabled
		FROM amazon_row_highlight_rules
		ORDER BY sort_order ASC, rule_key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list amazon row highlight rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.AmazonRowHighlightRule, 0)
	for rows.Next() {
		var item models.AmazonRowHighlightRule
		if err := rows.Scan(
			&item.RuleKey,
			&item.Label,
			&item.FieldName,
			&item.Operator,
			&item.ThresholdNumber,
			&item.MatchText,
			&item.ColorMode,
			&item.ColorStart,
			&item.ColorEnd,
			&item.SortOrder,
			&item.Enabled,
		); err != nil {
			return nil, fmt.Errorf("scan amazon row highlight rule: %w", err)
		}
		rules = append(rules, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate amazon row highlight rules: %w", err)
	}
	return rules, nil
}

func (r *AmazonRowHighlightRuleRepository) UpdateAll(ctx context.Context, rules []models.AmazonRowHighlightRule) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin amazon row highlight rules update: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, rule := range rules {
		if _, err := tx.Exec(ctx, `
			INSERT INTO amazon_row_highlight_rules (
				rule_key, label, field_name, operator, threshold_number, match_text, color_mode, color_start, color_end, sort_order, enabled, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
			ON CONFLICT (rule_key) DO UPDATE SET
				label = EXCLUDED.label,
				field_name = EXCLUDED.field_name,
				operator = EXCLUDED.operator,
				threshold_number = EXCLUDED.threshold_number,
				match_text = EXCLUDED.match_text,
				color_mode = EXCLUDED.color_mode,
				color_start = EXCLUDED.color_start,
				color_end = EXCLUDED.color_end,
				sort_order = EXCLUDED.sort_order,
				enabled = EXCLUDED.enabled,
				updated_at = NOW()
		`, rule.RuleKey, rule.Label, rule.FieldName, rule.Operator, rule.ThresholdNumber, rule.MatchText, rule.ColorMode, rule.ColorStart, rule.ColorEnd, rule.SortOrder, rule.Enabled); err != nil {
			return fmt.Errorf("upsert amazon row highlight rule %s: %w", rule.RuleKey, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit amazon row highlight rules update: %w", err)
	}
	return nil
}

func (r *AmazonRowHighlightRuleRepository) ResetDefaults(ctx context.Context) error {
	return r.UpdateAll(ctx, DefaultAmazonRowHighlightRules())
}

func stringPtr(value string) *string {
	return &value
}
