package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

// Claim is persisted before contacting DTDC. An ambiguous response must be reconciled,
// never automatically retried: the remote consignment may already exist.
var ErrShipmentLocked = errors.New("shipment is booked or awaiting review; refresh the order and reconcile with the courier before making shipment changes")

func (r *DirectOrderRepository) ClaimShipment(ctx context.Context, id, courier, actor string, version time.Time) error {
	return r.claimShipment(ctx, id, courier, actor, version, nil)
}
func (r *DirectOrderRepository) ClaimDTDCBooking(ctx context.Context, id, actor string, version time.Time, request json.RawMessage) error {
	return r.claimShipment(ctx, id, "dtdc", actor, version, request)
}
func (r *DirectOrderRepository) claimShipment(ctx context.Context, id, courier, actor string, version time.Time, request json.RawMessage) error {
	if courier != "dtdc" && courier != "delhivery" {
		return fmt.Errorf("unsupported courier")
	}
	result, err := r.db.Exec(ctx, `UPDATE direct_orders SET courier_type=$2,courier_status='booking_pending',updated_by=$3,
 service_type=CASE WHEN $5::jsonb IS NOT NULL THEN COALESCE($5::jsonb->'consignments'->0->>'service_type_id',service_type) ELSE service_type END,
 courier_payload=CASE WHEN $5::jsonb IS NOT NULL THEN (COALESCE(courier_payload,'{}'::jsonb)-'booking_error'-'booking_error_updated_at')||jsonb_build_object('booking_request',$5::jsonb) ELSE courier_payload END
 WHERE order_id=$1 AND deleted_at IS NULL AND order_status <> 'cancelled' AND updated_at=$4
 AND COALESCE(awb,'')='' AND COALESCE(courier_order_id,'')=''
 AND COALESCE(courier_status,'') NOT IN ('booking_pending','booking_review')`, id, courier, actor, version, request)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrShipmentLocked
	}
	return nil
}

// Called with the row locked so edits cannot erase an in-flight booking claim.
func validateShipmentEdit(courier, awb, status string, req *models.UpdateDirectOrderRequest) error {
	if status == "booking_pending" || status == "booking_review" {
		return ErrShipmentLocked
	}
	if courier != "dtdc" || awb == "" {
		return nil
	}
	if (req.CourierType != nil && !strings.EqualFold(*req.CourierType, courier)) ||
		(req.CourierName != nil && !strings.EqualFold(*req.CourierName, "DTDC")) ||
		(req.AWB != nil && *req.AWB != awb) {
		return ErrShipmentLocked
	}
	return nil
}

func (r *DirectOrderRepository) SaveDTDCBooking(ctx context.Context, id, awb string, raw json.RawMessage, actor string) error {
	result, err := r.db.Exec(ctx, `UPDATE direct_orders SET courier_type='dtdc',courier_name='DTDC',awb=$2,courier_order_id=$2,
 courier_status='Booked',order_status='forwarded',manifested_at=NOW(),updated_by=$4,
 courier_payload=COALESCE(courier_payload,'{}'::jsonb)||jsonb_build_object('booking',$3::jsonb) WHERE order_id=$1 AND courier_type='dtdc' AND courier_status='booking_pending' AND deleted_at IS NULL`, id, awb, raw, actor)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("booking could not be saved; reconcile DTDC AWB %s", awb)
	}
	return nil
}
func (r *DirectOrderRepository) SetShipmentBookingFailure(ctx context.Context, id string, uncertain bool, message ...string) error {
	status := "booking_rejected"
	if uncertain {
		status = "booking_review"
	}
	detail := ""
	if len(message) > 0 {
		detail = message[0]
	}
	_, err := r.db.Exec(ctx, `UPDATE direct_orders SET courier_status=$2,
 courier_payload=CASE WHEN $3<>'' THEN COALESCE(courier_payload,'{}'::jsonb)||jsonb_build_object('booking_error',$3::text,'booking_error_updated_at',NOW()) ELSE courier_payload END
 WHERE order_id=$1 AND courier_status='booking_pending'`, id, status, detail)
	return err
}
func (r *DirectOrderRepository) SaveDTDCEvent(ctx context.Context, id, awb, status, key string, raw json.RawMessage, actor string) error {
	result, err := r.db.Exec(ctx, `UPDATE direct_orders SET courier_status=$3,updated_by=$6,
 courier_payload=COALESCE(courier_payload,'{}'::jsonb)||jsonb_build_object($4::text,$5::jsonb,$4::text||'_updated_at',NOW())
 WHERE order_id=$1 AND awb=$2 AND courier_type='dtdc' AND deleted_at IS NULL
 AND (LOWER(COALESCE(courier_status,'')) <> 'cancelled' OR LOWER($3)='cancelled')`, id, awb, status, key, raw, actor)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("shipment changed; reload before continuing")
	}
	return nil
}

// A recovery only attaches an AWB after DTDC has confirmed the exact customer reference.
func (r *DirectOrderRepository) ReconcileDTDCBooking(ctx context.Context, id, awb, status string, raw json.RawMessage, actor string, version time.Time) error {
	result, err := r.db.Exec(ctx, `UPDATE direct_orders SET awb=$2,courier_order_id=$2,courier_name='DTDC',
 courier_status=$3,order_status='forwarded',manifested_at=COALESCE(manifested_at,NOW()),updated_by=$5,
 courier_payload=(COALESCE(courier_payload,'{}'::jsonb)-'booking_error'-'booking_error_updated_at')||jsonb_build_object('tracking',$4::jsonb,'tracking_updated_at',NOW(),'reconciled_at',NOW())
 WHERE order_id=$1 AND courier_type='dtdc' AND deleted_at IS NULL AND COALESCE(awb,'')='' AND COALESCE(courier_order_id,'')=''
 AND courier_status IN ('booking_pending','booking_review') AND updated_at=$6`, id, awb, status, raw, actor, version)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrShipmentLocked
	}
	return nil
}
