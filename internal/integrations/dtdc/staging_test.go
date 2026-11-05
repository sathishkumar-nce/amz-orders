package dtdc

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

const stagingBookingURL = "https://alphademodashboardapi.shipsy.io"
const stagingTrackingURL = "https://dtdcstagingapi.dtdc.com/dtdc-tracking-api"

func checkStagingURLs(base, tracking string) error {
	for _, pair := range [][2]string{{base, stagingBookingURL}, {tracking, stagingTrackingURL}} {
		if strings.TrimRight(pair[0], "/") != pair[1] {
			return fmt.Errorf("staging tests refuse endpoint %q; expected %s", pair[0], pair[1])
		}
	}
	return nil
}

func TestStagingURLGuard(t *testing.T) {
	for _, pair := range [][2]string{
		{"https://pxapi.dtdc.in", stagingTrackingURL},
		{stagingBookingURL, "https://blktracksvc.dtdc.com"},
		{stagingBookingURL + ".example.com", stagingTrackingURL},
	} {
		if err := checkStagingURLs(pair[0], pair[1]); err == nil {
			t.Fatal("unsafe endpoint allowed")
		}
	}
	if err := checkStagingURLs(stagingBookingURL, stagingTrackingURL); err != nil {
		t.Fatal(err)
	}
}

// This test never reads the app's .env/.env.docker, never writes to its database,
// and uses the seller's own contact/address as the synthetic test recipient.
func TestStagingLifecycle(t *testing.T) {
	if os.Getenv("DTDC_RUN_STAGING_TESTS") != "1" {
		t.Skip("opt in with DTDC_RUN_STAGING_TESTS=1 and staging account configuration")
	}
	path := filepath.Join("..", "..", "..", ".env.dtdc.staging.local")
	values, err := godotenv.Read(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read staging configuration: %v", err)
	}
	get := func(key string) string {
		if value, exists := os.LookupEnv(key); exists {
			return strings.TrimSpace(value)
		}
		return strings.TrimSpace(values[key])
	}
	base, tracking := get("DTDC_BASE_URL"), get("DTDC_TRACKING_URL")
	if base == "" {
		base = stagingBookingURL
	}
	if tracking == "" {
		tracking = stagingTrackingURL
	}
	if err := checkStagingURLs(base, tracking); err != nil {
		t.Fatal(err)
	}
	client := NewClient(Config{BaseURL: base, TrackingURL: tracking, APIKey: get("DTDC_API_KEY"), TrackingToken: get("DTDC_TRACKING_TOKEN"), CustomerCode: get("DTDC_CUSTOMER_CODE"), ServiceType: get("DTDC_SERVICE_TYPE"), Commodity: get("DTDC_COMMODITY"), Origin: Address{Name: get("DTDC_ORIGIN_NAME"), Phone: get("DTDC_ORIGIN_PHONE"), Address: get("DTDC_ORIGIN_ADDRESS"), Pincode: get("DTDC_ORIGIN_PINCODE"), City: get("DTDC_ORIGIN_CITY"), State: get("DTDC_ORIGIN_STATE")}})
	if missing := client.MissingConfig(); len(missing) > 0 {
		t.Fatalf("staging booking cannot run; missing %s", strings.Join(missing, ", "))
	}
	if client.cfg.TrackingToken == "" {
		t.Fatal("DTDC_TRACKING_TOKEN is required to check staging tracking")
	}
	client.http.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		allowed := map[string]bool{"alphademodashboardapi.shipsy.io": true, "dtdcstagingapi.dtdc.com": true}
		if req.URL.Scheme != "https" || !allowed[req.URL.Host] || len(via) > 3 {
			return fmt.Errorf("staging test refused redirect")
		}
		return nil
	}
	// Check the resulting endpoints explicitly; no production fallback is allowed.
	for _, endpoint := range []string{client.cfg.BaseURL, client.cfg.TrackingURL} {
		if _, err := url.ParseRequestURI(endpoint); err != nil {
			t.Fatal(err)
		}
	}
	text := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
	number := func(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }
	order := &models.DirectOrder{OrderID: fmt.Sprintf("DTDC-STAGE-CHECK-%d", time.Now().UnixNano()), OrderStatus: "confirmed", CustomerName: text("DTDC Staging Integration Test"), Mobile: text(client.cfg.Origin.Phone), Address: text(client.cfg.Origin.Address), Pincode: text(client.cfg.Origin.Pincode), City: text(client.cfg.Origin.City), State: text(client.cfg.Origin.State), Country: text("India"), TotalWeight: number(1000), LengthCM: number(10), WidthCM: number(10), HeightCM: number(5), Amount: number(1), CODAmount: number(0), PackageCount: sql.NullInt64{Int64: 1, Valid: true}}
	payload, err := client.BuildBooking(order, BookingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	booking, err := client.Book(ctx, payload)
	if err != nil {
		t.Fatalf("staging booking failed: %v", err)
	}
	t.Logf("Staging booking accepted, AWB %s, reference %s", booking.AWB, order.OrderID)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := client.Cancel(ctx, booking.AWB); err != nil {
			t.Errorf("staging cancellation failed for AWB %s; cancel it in staging: %v", booking.AWB, err)
		} else {
			t.Logf("Staging cancellation confirmed for %s", booking.AWB)
		}
	})
	label, err := client.Label(ctx, booking.AWB, "SHIP_LABEL_4X6")
	if err != nil {
		t.Errorf("staging label failed: %v", err)
	} else if !bytes.HasPrefix(label, []byte("%PDF-")) {
		t.Error("staging label is not PDF")
	} else {
		t.Log("Staging PDF label verified")
	}
	awb := get("DTDC_STAGE_TRACKING_AWB")
	if awb == "" {
		awb = booking.AWB
	}
	result, err := client.Track(ctx, awb)
	if err != nil {
		if strings.Contains(err.Error(), "not available yet") && get("DTDC_STAGE_TRACKING_AWB") == "" {
			t.Log("Tracking endpoint reached; no scan events yet for the new staging AWB. Set DTDC_STAGE_TRACKING_AWB to verify existing scan history.")
		} else {
			t.Errorf("staging tracking failed: %v", err)
		}
	} else {
		t.Logf("Staging tracking verified for %s: %s (%d scans)", awb, result.Header["strStatus"], len(result.Details))
	}
}
