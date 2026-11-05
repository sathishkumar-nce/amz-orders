package repository

import (
	"errors"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"strings"
	"testing"
)

func TestDTDCShipmentFilterIncludesBookingsAwaitingReviewAndExcludesOtherCouriers(t *testing.T) {
	where, args, _ := buildDirectOrderWhere(models.DirectOrderFilters{DTDCBookedOnly: true, CourierStatus: "Delivered", Search: "DO-42"})
	for _, part := range []string{"deleted_at IS NULL", "courier_type = 'dtdc'", "booking_pending", "booking_review", "courier_status ILIKE $1"} {
		if !strings.Contains(where, part) {
			t.Fatalf("missing %q in %s", part, where)
		}
	}
	if len(args) != 2 || args[0] != "Delivered" || args[1] != "%DO-42%" {
		t.Fatalf("unexpected filter arguments: %v", args)
	}
}
func TestShipmentEditsCannotEraseDTDCTrackingOrPendingBooking(t *testing.T) {
	text := func(v string) *string { return &v }
	for _, tt := range []struct {
		name, courier, awb, status string
		req                        models.UpdateDirectOrderRequest
		locked                     bool
	}{
		{"pending order edits", "dtdc", "", "booking_pending", models.UpdateDirectOrderRequest{Remarks: text("note")}, true},
		{"review edits", "dtdc", "", "booking_review", models.UpdateDirectOrderRequest{CourierType: text("delhivery")}, true},
		{"delhivery pending", "delhivery", "", "booking_pending", models.UpdateDirectOrderRequest{}, true},
		{"AWB overwrite", "dtdc", "D123", "Booked", models.UpdateDirectOrderRequest{AWB: text("D456")}, true},
		{"courier switch", "dtdc", "D123", "Booked", models.UpdateDirectOrderRequest{CourierType: text("delhivery")}, true},
		{"same carrier identity", "dtdc", "D123", "Booked", models.UpdateDirectOrderRequest{AWB: text("D123"), CourierType: text("dtdc")}, false},
		{"ordinary note", "dtdc", "D123", "Booked", models.UpdateDirectOrderRequest{Remarks: text("note")}, false},
		{"unbooked courier switch", "dtdc", "", "booking_rejected", models.UpdateDirectOrderRequest{CourierType: text("delhivery")}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShipmentEdit(tt.courier, tt.awb, tt.status, &tt.req)
			if errors.Is(err, ErrShipmentLocked) != tt.locked {
				t.Fatalf("unexpected edit result: %v", err)
			}
		})
	}
}
