package delhivery

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTrackingMatchesAWBAndNormalizesProviderDetails(t *testing.T) {
	client := NewClient(Config{APIToken: "tracking-test"})
	client.httpClient = &http.Client{Transport: bookingTransport(func(r *http.Request) (*http.Response, error) {
		if (r.URL.Query().Get("waybill") != "D123" && r.URL.Query().Get("waybill") != "WRONG") || r.Header.Get("Authorization") != "Token tracking-test" {
			t.Fatal("incorrect official tracking request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ShipmentData":[{"Shipment":{"AWB":"D123","Origin":"Namakkal","Destination":"Chennai","ExpectedDeliveryDate":"2026-11-07","Status":{"Status":"In Transit","StatusLocation":"Chennai Hub","StatusDateTime":"2026-11-05T11:30:00","Instructions":"On the way"},"Scans":[{"ScanDetail":{"Scan":"Received","ScannedLocation":"Chennai Hub","ScanDateTime":"2026-11-05T11:30:00"}}]}}]}`)), Header: http.Header{}}, nil
	})}
	snapshot, raw, err := client.Track(context.Background(), "D123")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "In Transit" || snapshot.CurrentLocation != "Chennai Hub" || len(snapshot.Events) != 1 || len(raw) == 0 || !strings.HasPrefix(snapshot.TrackingURL, "https://www.delhivery.com/") {
		t.Fatalf("incomplete tracking snapshot: %+v", snapshot)
	}
	if _, _, err = client.Track(context.Background(), "WRONG"); err == nil {
		t.Fatal("mismatched AWB accepted")
	}
}
func TestTrackingDoesNotInventStatusForEmptyProviderResponse(t *testing.T) {
	client := NewClient(Config{APIToken: "tracking-test"})
	for _, body := range []string{`{}`, `{"ShipmentData":[]}`, `{"ShipmentData":[{"Shipment":{"AWB":"D123"}}]}`} {
		client.httpClient = &http.Client{Transport: bookingTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})}
		if _, _, err := client.Track(context.Background(), "D123"); err == nil {
			t.Fatal("unconfirmed status accepted")
		}
	}
}
