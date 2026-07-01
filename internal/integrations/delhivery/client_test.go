package delhivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
