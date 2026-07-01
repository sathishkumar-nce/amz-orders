package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

const interaktSettingsStateKey = "global"

type InteraktSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewInteraktSettingsRepository(pool *pgxpool.Pool) *InteraktSettingsRepository {
	return &InteraktSettingsRepository{pool: pool}
}

func (r *InteraktSettingsRepository) Get(ctx context.Context, defaults models.InteraktSettings) (*models.InteraktSettings, error) {
	var settings models.InteraktSettings
	err := r.pool.QueryRow(ctx, `
		SELECT enabled, mode, template_name, test_number
		FROM interakt_settings
		WHERE state_key = $1
	`, interaktSettingsStateKey).Scan(&settings.Enabled, &settings.Mode, &settings.TemplateName, &settings.TestNumber)
	if err == nil {
		return &settings, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("get interakt settings: %w", err)
	}

	if err := r.Update(ctx, defaults); err != nil {
		return nil, err
	}
	return &defaults, nil
}

func (r *InteraktSettingsRepository) Update(ctx context.Context, settings models.InteraktSettings) error {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO interakt_settings (state_key, enabled, mode, template_name, test_number, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (state_key) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			mode = EXCLUDED.mode,
			template_name = EXCLUDED.template_name,
			test_number = EXCLUDED.test_number,
			updated_at = NOW()
	`, interaktSettingsStateKey, settings.Enabled, settings.Mode, settings.TemplateName, settings.TestNumber); err != nil {
		return fmt.Errorf("update interakt settings: %w", err)
	}
	return nil
}
