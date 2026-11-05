package delhivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func TestLookupPincodeParsesNestedPostalCodeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter_codes"); got != "641664" {
			t.Fatalf("expected filter_codes=641664, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Fatalf("expected authorization header to be set, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"delivery_codes": [
				{
					"postal_code": {
						"pin": "641664",
						"city": "Tiruppur",
						"district": "Tiruppur",
						"state": "Tamil Nadu",
						"state_code": "TN",
						"country": "India",
						"pre_paid": "Y",
						"cash": "N"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:  server.URL,
		APIToken: "test-token",
	})

	result, err := client.LookupPincode(context.Background(), "641664")
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}

	if result.Pincode != "641664" {
		t.Fatalf("expected pincode 641664, got %q", result.Pincode)
	}
	if result.City != "Tiruppur" {
		t.Fatalf("expected city Tiruppur, got %q", result.City)
	}
	if result.District != "Tiruppur" {
		t.Fatalf("expected district Tiruppur, got %q", result.District)
	}
	if result.State != "Tamil Nadu" {
		t.Fatalf("expected state Tamil Nadu, got %q", result.State)
	}
	if result.StateCode != "TN" {
		t.Fatalf("expected state code TN, got %q", result.StateCode)
	}
	if result.Country != "India" {
		t.Fatalf("expected country India, got %q", result.Country)
	}
	if !result.Serviceable {
		t.Fatalf("expected pincode to be marked serviceable")
	}
	if result.COD {
		t.Fatalf("expected cod=false")
	}
	if !result.Prepaid {
		t.Fatalf("expected prepaid=true")
	}
	if len(result.Raw) != 1 {
		t.Fatalf("expected 1 raw candidate, got %d", len(result.Raw))
	}
}

func TestLookupPincodeReturnsErrorWhenNoCandidatesFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"delivery_codes":[]}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:  server.URL,
		APIToken: "test-token",
	})

	_, err := client.LookupPincode(context.Background(), "641664")
	if err == nil {
		t.Fatalf("expected lookup to fail when no pincode data is returned")
	}
}

func TestLookupPincodeMapsStateCodeToStateName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"delivery_codes": [
				{
					"postal_code": {
						"pin": "637001",
						"city": "Namakkal",
						"district": "Namakkal",
						"state_code": "TN",
						"country": "India",
						"cash": "Y",
						"pre_paid": "Y"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:  server.URL,
		APIToken: "test-token",
	})

	result, err := client.LookupPincode(context.Background(), "637001")
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}

	if result.StateCode != "TN" {
		t.Fatalf("expected state code TN, got %q", result.StateCode)
	}
	if result.State != "Tamil Nadu" {
		t.Fatalf("expected state Tamil Nadu from state code map, got %q", result.State)
	}
	if len(result.Raw) != 1 || result.Raw[0].State != "Tamil Nadu" {
		t.Fatalf("expected raw candidate state to also be mapped, got %#v", result.Raw)
	}
}

func TestLookupPincodeMapsOuterStateCodeWhenPostalCodeIsNested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"delivery_codes": [
				{
					"state_code": "TN",
					"cash": "Y",
					"pre_paid": "Y",
					"postal_code": {
						"pin": "641662",
						"city": "Palladam",
						"district": "Coimbatore",
						"country": "India"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:  server.URL,
		APIToken: "test-token",
	})

	result, err := client.LookupPincode(context.Background(), "641662")
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}

	if result.City != "Palladam" {
		t.Fatalf("expected city Palladam, got %q", result.City)
	}
	if result.District != "Coimbatore" {
		t.Fatalf("expected district Coimbatore, got %q", result.District)
	}
	if result.StateCode != "TN" {
		t.Fatalf("expected state code TN, got %q", result.StateCode)
	}
	if result.State != "Tamil Nadu" {
		t.Fatalf("expected state Tamil Nadu, got %q", result.State)
	}
}

// Booking failures must not be persisted as successful shipments.
type bookingTransport func(*http.Request) (*http.Response, error)

func (f bookingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCreateForwardOrderRequiresConfirmedAWB(t *testing.T) {
	ns := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
	nf := func(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }
	order := &models.DirectOrder{OrderID: "DO-2001", OrderStatus: "confirmed", CustomerName: ns("Customer"), Address: ns("Street"), Mobile: ns("9999999999"), Pincode: ns("600001"), City: ns("Chennai"), State: ns("Tamil Nadu"), TotalWeight: nf(500), LengthCM: nf(10), WidthCM: nf(10), HeightCM: nf(5), Items: []models.DirectOrderItem{{Item: ns("Product"), Quantity: 1, HSN: ns("3926"), Weight: nf(500)}}}
	for _, tt := range []struct {
		name              string
		code              int
		body              string
		success, rejected bool
	}{
		{"accepted", 200, `{"success":true,"packages":[{"status":"Success","waybill":"123456","refnum":"DO-2001"}]}`, true, false},
		{"existing package success without top-level success flag", 200, `{"packages":[{"status":"Success","waybill":"123456","refnum":"DO-2001"}]}`, true, false},
		{"package rejected", 200, `{"success":false,"packages":[{"status":"Fail","remarks":["Invalid address"]}]}`, false, true},
		{"unauthorized", 401, `{}`, false, true},
		{"uncertain server response", 503, `{}`, false, false},
		{"unconfirmed", 200, `{"success":false}`, false, false},
		{"explicit rejection", 200, `{"success":false,"packages":[],"error":"Invalid account"}`, false, true},
		{"missing AWB", 200, `{"success":true,"packages":[{"status":"Success"}]}`, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(Config{APIToken: "test-token", SellerGSTIN: "test-gstin", DefaultPickupLocation: "Seller", SellerName: "Existing seller", SellerAddress: "Existing Delhivery seller address"})
			client.httpClient = &http.Client{Transport: bookingTransport(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Token test-token" || r.URL.Path != "/api/cmu/create.json" {
					t.Fatal("wrong Delhivery endpoint/credentials")
				}

				if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Fatal("Delhivery content type changed")
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.PostForm.Get("format") != "json" {
					t.Fatal("Delhivery format changed")
				}
				var payload createOrderPayload
				if err := json.Unmarshal([]byte(r.PostForm.Get("data")), &payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Shipments) != 1 || payload.PickupLocation["name"] != "Seller" {
					t.Fatal("existing Delhivery pickup mapping changed")
				}
				shipment := payload.Shipments[0]
				if shipment.Order != "DO-2001" || shipment.Weight != 500 || shipment.ShipmentLength != 10 || shipment.HSNCode != "3926" || shipment.Name != "Customer" || shipment.Pin != "600001" {
					t.Fatalf("Delhivery shipment mapping changed: %+v", shipment)
				}
				if shipment.SellerName != "Existing seller" || shipment.SellerAdd != "Existing Delhivery seller address" {
					t.Fatal("DTDC changes must not change Delhivery seller details")
				}
				if shipment.PaymentMode != "Pre-paid" || shipment.CODAmount != 0 {
					t.Fatal("prepaid behavior changed")
				}
				return &http.Response{StatusCode: tt.code, Body: io.NopCloser(strings.NewReader(tt.body)), Header: http.Header{}}, nil
			})}
			result, err := client.CreateForwardOrder(context.Background(), order)
			if tt.success {
				if err != nil || result.AWB == nil || *result.AWB != "123456" {
					t.Fatalf("unexpected booking: %v %v", result, err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected failure")
			}
			var rejected *RejectedError
			if errors.As(err, &rejected) != tt.rejected {
				t.Fatalf("wrong retry classification: %v", err)
			}
		})
	}
}

func TestDelhiveryCODAndOrderPickupArePreserved(t *testing.T) {
	ns := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
	nf := func(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }
	order := &models.DirectOrder{OrderID: "DO-COD", OrderStatus: "confirmed", CustomerName: ns("Customer"), Address: ns("Street"), Mobile: ns("9999999999"), Pincode: ns("600001"), City: ns("Chennai"), State: ns("Tamil Nadu"), PickupLocation: ns("Order pickup"), Amount: nf(1500), CODAmount: nf(500), TotalWeight: nf(750), LengthCM: nf(10), WidthCM: nf(10), HeightCM: nf(5), Items: []models.DirectOrderItem{{Item: ns("Product"), Quantity: 1, HSN: ns("3926"), Weight: nf(750)}}}
	client := NewClient(Config{APIToken: "legacy-token", SellerGSTIN: "legacy-gstin", DefaultPickupLocation: "Default legacy pickup"})
	client.httpClient = &http.Client{Transport: bookingTransport(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		var payload createOrderPayload
		if err := json.Unmarshal([]byte(r.PostForm.Get("data")), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.PickupLocation["name"] != "Order pickup" {
			t.Fatal("per-order Delhivery pickup override changed")
		}
		shipment := payload.Shipments[0]
		if shipment.PaymentMode != "COD" || shipment.CODAmount != 500 || shipment.TotalAmount != 1500 || shipment.Weight != 750 {
			t.Fatalf("Delhivery COD/grams mapping changed: %+v", shipment)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"packages":[{"status":"Success","waybill":"COD123","refnum":"DO-COD"}]}`)), Header: http.Header{}}, nil
	})}
	result, err := client.CreateForwardOrder(context.Background(), order)
	if err != nil {
		t.Fatal(err)
	}
	if result.AWB == nil || *result.AWB != "COD123" || result.CourierOrderID == nil || *result.CourierOrderID != "DO-COD" {
		t.Fatal("Delhivery AWB/reference mapping changed")
	}
}
