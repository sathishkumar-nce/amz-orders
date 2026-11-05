package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func (r *DirectOrderRepository) GetCourierSettings(ctx context.Context, defaults models.CourierSettings) (*models.CourierSettings, error) {
	var raw []byte
	result := defaults
	err := r.db.QueryRow(ctx, `SELECT settings,updated_at FROM courier_settings WHERE state_key='global'`).Scan(&raw, &result.UpdatedAt)
	if err == pgx.ErrNoRows {
		raw, err = json.Marshal(defaults)
		if err != nil {
			return nil, err
		}
		if _, err = r.db.Exec(ctx, `INSERT INTO courier_settings(state_key,settings) VALUES ('global',$1::jsonb) ON CONFLICT DO NOTHING`, raw); err != nil {
			return nil, fmt.Errorf("courier settings unavailable; apply the latest database migrations: %w", err)
		}
		err = r.db.QueryRow(ctx, `SELECT settings,updated_at FROM courier_settings WHERE state_key='global'`).Scan(&raw, &result.UpdatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("load courier settings: %w", err)
	}
	updatedAt := result.UpdatedAt
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.UpdatedAt = updatedAt
	return &result, nil
}
func (r *DirectOrderRepository) SaveCourierSettings(ctx context.Context, settings models.CourierSettings) (*models.CourierSettings, error) {
	raw, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `INSERT INTO courier_settings(state_key,settings) VALUES ('global',$1::jsonb)
 ON CONFLICT(state_key) DO UPDATE SET settings=EXCLUDED.settings,updated_at=NOW() RETURNING updated_at`, raw).Scan(&settings.UpdatedAt)
	return &settings, err
}

// Stores both the provider response (for detailed history) and a common summary.
// A row lock prevents an old tracking response from replacing a confirmed cancellation.
func (r *DirectOrderRepository) SaveShipmentTracking(ctx context.Context, id, awb, courier string, snapshot *models.ShipmentTracking, raw json.RawMessage, actor string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var savedAWB, savedCourier, status string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(awb,''),LOWER(COALESCE(courier_type,'')),COALESCE(courier_status,'') FROM direct_orders WHERE order_id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&savedAWB, &savedCourier, &status); err != nil {
		return err
	}
	if savedAWB != awb || savedCourier != courier {
		return ErrShipmentLocked
	}
	if strings.EqualFold(status, "Cancelled") {
		snapshot.Status = "Cancelled"
	}
	summary, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE direct_orders SET courier_status=$2,updated_by=$3,
 courier_payload=COALESCE(courier_payload,'{}'::jsonb)||jsonb_build_object('tracking',$4::jsonb,'tracking_summary',$5::jsonb,'tracking_updated_at',NOW())
 WHERE order_id=$1`, id, snapshot.Status, actor, raw, summary)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
