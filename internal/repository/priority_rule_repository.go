package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type PriorityRuleRepository struct {
	pool *pgxpool.Pool
}

var priorityKeyOrder = []string{"p1", "p2", "p3", "p4"}

func NewPriorityRuleRepository(pool *pgxpool.Pool) *PriorityRuleRepository {
	return &PriorityRuleRepository{pool: pool}
}

func normalizePriorityKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func normalizeSKUValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		sku := strings.ToUpper(strings.TrimSpace(value))
		if sku == "" {
			continue
		}
		if _, exists := seen[sku]; exists {
			continue
		}
		seen[sku] = struct{}{}
		normalized = append(normalized, sku)
	}
	sort.Strings(normalized)
	return normalized
}

func prioritySortIndex(key string) int {
	for index, candidate := range priorityKeyOrder {
		if key == candidate {
			return index
		}
	}
	return len(priorityKeyOrder)
}

func sortPriorityRules(rules []models.OrderPriorityRule) {
	sort.Slice(rules, func(i, j int) bool {
		return prioritySortIndex(rules[i].Key) < prioritySortIndex(rules[j].Key)
	})
}

func (r *PriorityRuleRepository) List(ctx context.Context) ([]models.OrderPriorityRule, error) {
	const query = `
		SELECT priority_key, sku_values, COALESCE(updated_by, ''), updated_at
		FROM order_priority_rules
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query priority rules: %w", err)
	}
	defer rows.Close()

	ruleByKey := make(map[string]models.OrderPriorityRule, len(priorityKeyOrder))
	for rows.Next() {
		var rule models.OrderPriorityRule
		if err := rows.Scan(&rule.Key, &rule.SKUValues, &rule.UpdatedBy, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan priority rule: %w", err)
		}
		rule.Key = normalizePriorityKey(rule.Key)
		rule.SKUValues = normalizeSKUValues(rule.SKUValues)
		ruleByKey[rule.Key] = rule
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate priority rules: %w", err)
	}

	rules := make([]models.OrderPriorityRule, 0, len(priorityKeyOrder))
	for _, key := range priorityKeyOrder {
		rule, exists := ruleByKey[key]
		if !exists {
			rule = models.OrderPriorityRule{Key: key, SKUValues: []string{}}
		}
		rules = append(rules, rule)
	}
	sortPriorityRules(rules)
	return rules, nil
}

func (r *PriorityRuleRepository) UpdateAll(ctx context.Context, rules []models.OrderPriorityRuleInput, actor string) ([]models.OrderPriorityRule, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin priority rule update tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO order_priority_rules (priority_key, sku_values, updated_by, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), NOW())
		ON CONFLICT (priority_key) DO UPDATE
		SET sku_values = EXCLUDED.sku_values,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`

	for _, rule := range rules {
		key := normalizePriorityKey(rule.Key)
		skus := normalizeSKUValues(rule.SKUValues)
		if _, err := tx.Exec(ctx, query, key, skus, actor); err != nil {
			return nil, fmt.Errorf("upsert priority rule %s: %w", key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit priority rule update tx: %w", err)
	}

	return r.List(ctx)
}

func (r *PriorityRuleRepository) GetPrioritySKUMap(ctx context.Context) (map[string]map[string]struct{}, error) {
	rules, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]map[string]struct{}, len(rules))
	for _, rule := range rules {
		skuSet := make(map[string]struct{}, len(rule.SKUValues))
		for _, sku := range rule.SKUValues {
			skuSet[sku] = struct{}{}
		}
		result[rule.Key] = skuSet
	}

	return result, nil
}
