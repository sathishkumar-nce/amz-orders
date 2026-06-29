package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type ShippingDateFilterRepository struct {
	pool *pgxpool.Pool
}

func NewShippingDateFilterRepository(pool *pgxpool.Pool) *ShippingDateFilterRepository {
	return &ShippingDateFilterRepository{pool: pool}
}

func DefaultShippingDateFilterSettings() []models.ShippingDateFilterSetting {
	return []models.ShippingDateFilterSetting{
		{FilterKey: "prime_ship_today", Label: "Prime - Ship Today", StartDayOffset: -1, StartHour: 13, StartMinute: 50, EndDayOffset: 0, EndHour: 13, EndMinute: 50, IsRangeEnabled: true},
		{FilterKey: "prime_ship_tomorrow", Label: "Prime - Ship Tomorrow", StartDayOffset: 0, StartHour: 13, StartMinute: 50, EndDayOffset: 1, EndHour: 13, EndMinute: 50, IsRangeEnabled: true},
		{FilterKey: "one_day_handle_ship_today", Label: "1 Day Handle - Ship Today", StartDayOffset: -2, StartHour: 10, StartMinute: 0, EndDayOffset: -1, EndHour: 10, EndMinute: 0, IsRangeEnabled: true},
		{FilterKey: "one_day_handle_ship_tomorrow", Label: "1 Day Handle - Ship Tomorrow", StartDayOffset: -1, StartHour: 10, StartMinute: 0, EndDayOffset: 0, EndHour: 10, EndMinute: 0, IsRangeEnabled: true},
		{FilterKey: "custom", Label: "Custom", StartDayOffset: 0, StartHour: 0, StartMinute: 0, EndDayOffset: 1, EndHour: 0, EndMinute: 0, IsRangeEnabled: true},
		{FilterKey: "all_date", Label: "All Date", StartDayOffset: 0, StartHour: 0, StartMinute: 0, EndDayOffset: 0, EndHour: 0, EndMinute: 0, IsRangeEnabled: false},
	}
}

func (r *ShippingDateFilterRepository) List(ctx context.Context) ([]models.ShippingDateFilterSetting, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT filter_key, label, start_day_offset, start_hour, start_minute, end_day_offset, end_hour, end_minute, is_range_enabled
		FROM shipping_date_filter_settings
		ORDER BY CASE filter_key
			WHEN 'prime_ship_today' THEN 1
			WHEN 'prime_ship_tomorrow' THEN 2
			WHEN 'one_day_handle_ship_today' THEN 3
			WHEN 'one_day_handle_ship_tomorrow' THEN 4
			WHEN 'custom' THEN 5
			WHEN 'all_date' THEN 6
			ELSE 99
		END, filter_key
	`)
	if err != nil {
		return nil, fmt.Errorf("list shipping date filter settings: %w", err)
	}
	defer rows.Close()

	settings := make([]models.ShippingDateFilterSetting, 0)
	for rows.Next() {
		var item models.ShippingDateFilterSetting
		if err := rows.Scan(
			&item.FilterKey,
			&item.Label,
			&item.StartDayOffset,
			&item.StartHour,
			&item.StartMinute,
			&item.EndDayOffset,
			&item.EndHour,
			&item.EndMinute,
			&item.IsRangeEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan shipping date filter setting: %w", err)
		}
		settings = append(settings, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shipping date filter settings: %w", err)
	}
	return settings, nil
}

func (r *ShippingDateFilterRepository) UpdateAll(ctx context.Context, filters []models.ShippingDateFilterSetting) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin shipping date filter settings update: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, filter := range filters {
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipping_date_filter_settings (
				filter_key, label, start_day_offset, start_hour, start_minute, end_day_offset, end_hour, end_minute, updated_at
				, is_range_enabled
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),$9)
			ON CONFLICT (filter_key) DO UPDATE SET
				label = EXCLUDED.label,
				start_day_offset = EXCLUDED.start_day_offset,
				start_hour = EXCLUDED.start_hour,
				start_minute = EXCLUDED.start_minute,
				end_day_offset = EXCLUDED.end_day_offset,
				end_hour = EXCLUDED.end_hour,
				end_minute = EXCLUDED.end_minute,
				is_range_enabled = EXCLUDED.is_range_enabled,
				updated_at = NOW()
		`, filter.FilterKey, filter.Label, filter.StartDayOffset, filter.StartHour, filter.StartMinute, filter.EndDayOffset, filter.EndHour, filter.EndMinute, filter.IsRangeEnabled); err != nil {
			return fmt.Errorf("upsert shipping date filter setting %s: %w", filter.FilterKey, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit shipping date filter settings update: %w", err)
	}
	return nil
}

func (r *ShippingDateFilterRepository) ResetDefaults(ctx context.Context) error {
	return r.UpdateAll(ctx, DefaultShippingDateFilterSettings())
}

func (r *ShippingDateFilterRepository) GetActiveFilterKey(ctx context.Context) (string, error) {
	var activeKey string
	if err := r.pool.QueryRow(ctx, `
		SELECT active_filter_key
		FROM shipping_date_filter_state
		WHERE state_key = 'global'
	`).Scan(&activeKey); err != nil {
		return "", fmt.Errorf("get active shipping date filter key: %w", err)
	}
	return activeKey, nil
}

func (r *ShippingDateFilterRepository) SetActiveFilterKey(ctx context.Context, filterKey string) error {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO shipping_date_filter_state (state_key, active_filter_key, updated_at)
		VALUES ('global', $1, NOW())
		ON CONFLICT (state_key) DO UPDATE SET
			active_filter_key = EXCLUDED.active_filter_key,
			updated_at = NOW()
	`, filterKey); err != nil {
		return fmt.Errorf("set active shipping date filter key: %w", err)
	}

	return nil
}
