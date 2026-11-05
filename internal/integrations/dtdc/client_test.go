package dtdc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func ns(s string) sql.NullString                                          { return sql.NullString{String: s, Valid: true} }
func nf(n float64) sql.NullFloat64                                        { return sql.NullFloat64{Float64: n, Valid: true} }
func fixture() (*Client, *models.DirectOrder) {
	return NewClient(Config{APIKey: "test-key", TrackingToken: "test-track", CustomerCode: "SELLER", ServiceType: "B2C PREMIUM", Commodity: "Home goods", Origin: Address{Name: "Seller", Phone: "9999999999", Address: "Origin street", Pincode: "600001", City: "Chennai", State: "Tamil Nadu"}}),
		&models.DirectOrder{OrderID: "DO-1001", OrderStatus: "confirmed", CustomerName: ns("Customer"), Mobile: ns("8888888888"), Address: ns("Destination street"), Landmark: ns("Near station"), City: ns("Salem"), State: ns("Tamil Nadu"), Pincode: ns("636010"), TotalWeight: nf(1250), Amount: nf(1500), CODAmount: nf(500), LengthCM: nf(30), WidthCM: nf(20), HeightCM: nf(5), PackageCount: sql.NullInt64{Int64: 1, Valid: true}}
}
func mockResponse(c *Client, code int, body string, inspect func(*http.Request)) {
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if inspect != nil {
			inspect(r)
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
}
func TestBuildBookingUsesSellerAndCustomerWithGramsConversion(t *testing.T) {
	c, o := fixture()
	p, err := c.BuildBooking(o, BookingOptions{ServiceType: "B2C PRIORITY", EwayBill: "123456789012", RiskSurcharge: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	var decoded struct {
		Consignments []map[string]any `json:"consignments"`
	}
	_ = json.Unmarshal(raw, &decoded)
	s := decoded.Consignments[0]
	for key, want := range map[string]any{"weight": "1.25", "weight_unit": "KG", "cod_amount": "500", "cod_collection_mode": "CASH", "service_type_id": "B2C PRIORITY", "customer_reference_number": "DO-1001", "eway_bill": "123456789012", "is_risk_surcharge_applicable": true} {
		if s[key] != want {
			t.Errorf("%s = %v, want %v", key, s[key], want)
		}
	}
	if s["origin_details"].(map[string]any)["name"] != "Seller" || s["destination_details"].(map[string]any)["name"] != "Customer" || s["destination_details"].(map[string]any)["address_line_2"] != "Near station" {
		t.Fatal("seller/customer details not mapped")
	}
	o.CODAmount = nf(0)
	p, err = c.BuildBooking(o, BookingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s = p["consignments"].([]any)[0].(map[string]any)
	if s["cod_amount"] != "0" || s["cod_collection_mode"] != "" {
		t.Fatal("prepaid shipment mapped as COD")
	}
}
func TestBookingValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Client, *models.DirectOrder, *BookingOptions)
	}{
		{"missing seller config", func(c *Client, o *models.DirectOrder, b *BookingOptions) { c.cfg.CustomerCode = "" }},
		{"invalid origin pincode", func(c *Client, o *models.DirectOrder, b *BookingOptions) { c.cfg.Origin.Pincode = "abc" }},
		{"existing AWB", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.AWB = ns("D123") }},
		{"existing carrier reference", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.CourierOrderID = ns("ref") }},
		{"cancelled", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.OrderStatus = "cancelled" }},
		{"reverse", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.ShipmentType = ns("reverse") }},
		{"international", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.Country = ns("US") }},
		{"invalid phone", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.Mobile = ns("abc") }},
		{"invalid pincode", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.Pincode = ns("123") }},
		{"zero weight", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.TotalWeight = nf(0) }},
		{"COD over declared", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.CODAmount = nf(2000) }},
		{"missing multiparcel details", func(c *Client, o *models.DirectOrder, b *BookingOptions) { o.PackageCount.Int64 = 2 }},
		{"invalid eway bill", func(c *Client, o *models.DirectOrder, b *BookingOptions) { b.EwayBill = "123" }},
		{"NaN piece weight", func(c *Client, o *models.DirectOrder, b *BookingOptions) {
			o.PackageCount.Int64 = 2
			b.Pieces = []Piece{{Description: "A", Value: "1", Weight: "NaN", Length: "1", Width: "1", Height: "1"}, {Description: "B", Value: "1", Weight: "1", Length: "1", Width: "1", Height: "1"}}
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, o := fixture()
			b := BookingOptions{}
			tt.change(c, o, &b)
			if _, err := c.BuildBooking(o, b); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	c, o := fixture()
	o.PackageCount.Int64 = 2
	if _, err := c.BuildBooking(o, BookingOptions{Pieces: []Piece{{Description: "A", Value: "750", Weight: "0.5", Length: "1", Width: "1", Height: "1"}, {Description: "B", Value: "750", Weight: "0.75", Length: "1", Width: "1", Height: "1"}}}); err != nil {
		t.Fatal(err)
	}
}
func TestBookConfirmsAWBAndSeparatesRejectedFromAmbiguousResponses(t *testing.T) {
	cases := []struct {
		name              string
		code              int
		body              string
		success, rejected bool
	}{
		{"success", 200, `{"status":"OK","data":[{"success":true,"reference_number":"D123"}]}`, true, false},
		{"rejected", 200, `{"status":"OK","data":[{"success":false,"message":"Not serviceable"}]}`, false, true},
		{"unauthorized", 401, `{}`, false, true},
		{"server error", 503, `{}`, false, false},
		{"invalid json", 200, `oops`, false, false},
		{"missing AWB", 200, `{"status":"OK","data":[{"success":true}]}`, false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := fixture()
			mockResponse(c, tt.code, tt.body, func(r *http.Request) {
				if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/softdata") || r.Header.Get("api-key") != "test-key" {
					t.Fatal("incorrect booking request")
				}
			})
			b, err := c.Book(context.Background(), map[string]any{})
			if tt.success {
				if err != nil || b.AWB != "D123" {
					t.Fatalf("booking = %v, %v", b, err)
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
	c, _ := fixture()
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("connection dropped") })}
	_, err := c.Book(context.Background(), nil)
	var rejected *RejectedError
	if err == nil || errors.As(err, &rejected) {
		t.Fatalf("network failure must be ambiguous: %v", err)
	}
}
func TestTrackRejectsMissingAndMismatchedShipment(t *testing.T) {
	for _, tt := range []struct {
		body  string
		valid bool
	}{
		{`{"statusFlag":true,"status":"SUCCESS","trackHeader":{"strShipmentNo":"D123","strStatus":"Delivered"},"trackDetails":[]}`, true},
		{`{"statusFlag":false,"status":"FAILED"}`, false},
		{`{"statusFlag":true,"status":"SUCCESS","trackHeader":{"strShipmentNo":"OTHER","strStatus":"Delivered"}}`, false},
	} {
		c, _ := fixture()
		mockResponse(c, 200, tt.body, func(r *http.Request) {
			if r.Header.Get("x-access-token") != "test-track" || r.URL.Path != "/dtdc-api/rest/JSONCnTrk/getTrackDetails" {
				t.Fatal("incorrect tracking request")
			}
		})
		_, err := c.Track(context.Background(), "D123")
		if (err == nil) != tt.valid {
			t.Fatalf("unexpected tracking result: %v", err)
		}
	}
}
func TestCancelRequiresMatchingConfirmedAWB(t *testing.T) {
	for _, tt := range []struct {
		body  string
		valid bool
	}{
		{`{"status":"OK","success":true,"successConsignments":[{"success":true,"reference_number":"D123"}]}`, true},
		{`{"status":"OK","success":true,"successConsignments":[{"success":true,"reference_number":"OTHER"}]}`, false},
		{`{"status":"OK","success":false,"successConsignments":[]}`, false},
	} {
		c, _ := fixture()
		mockResponse(c, 200, tt.body, func(r *http.Request) {
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["customerCode"] != "SELLER" || b["AWBNo"].([]any)[0] != "D123" {
				t.Fatal("wrong cancellation payload")
			}
		})
		_, err := c.Cancel(context.Background(), "D123")
		if (err == nil) != tt.valid {
			t.Fatalf("unexpected cancellation result: %v", err)
		}
	}
}
func TestLabelRequiresPDFAndSupportedFormat(t *testing.T) {
	c, _ := fixture()
	mockResponse(c, 200, "%PDF-test", func(r *http.Request) {
		if r.Method != "GET" || r.URL.Query().Get("label_code") != "SHIP_LABEL_4X6" || r.URL.Query().Get("reference_number") != "D123" {
			t.Fatal("incorrect label request")
		}
	})
	if _, err := c.Label(context.Background(), "D123", "SHIP_LABEL_4X6"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Label(context.Background(), "D123", "UNKNOWN"); err == nil {
		t.Fatal("expected unsupported label error")
	}
	mockResponse(c, 200, `{"error":"not found"}`, nil)
	if _, err := c.Label(context.Background(), "D123", "SHIP_LABEL_4X6"); err == nil {
		t.Fatal("JSON must not be downloaded as PDF")
	}
}

func TestBookingDropdownOverridesUseDTDCIDsWithoutChangingAccountConfig(t *testing.T) {
	client, order := fixture()
	client.cfg.ServiceType = ""
	client.cfg.Commodity = ""
	payload, err := client.BuildBooking(order, BookingOptions{ServiceType: "B2C PRIORITY", Commodity: "84"})
	if err != nil {
		t.Fatal(err)
	}
	shipment := payload["consignments"].([]any)[0].(map[string]any)
	if shipment["service_type_id"] != "B2C PRIORITY" || shipment["commodity_id"] != "84" || shipment["description"] != "PLASTIC ITEMS" {
		t.Fatalf("dropdown options not mapped: %v", shipment)
	}
	if client.cfg.ServiceType != "" || client.cfg.Commodity != "" {
		t.Fatal("per-booking choice changed seller defaults")
	}
	client.cfg.CustomerCode = ""
	if _, err := client.BuildBooking(order, BookingOptions{ServiceType: "B2C PRIORITY", Commodity: "84"}); err == nil {
		t.Fatal("dropdown selection must not bypass missing account configuration")
	}
}

func TestSettingsExposeDocumentedDropdownOptionsWithoutCredentials(t *testing.T) {
	client, _ := fixture()
	settings := client.Settings()
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "test-key") || strings.Contains(string(raw), "test-track") {
		t.Fatal("settings exposed credentials")
	}
	options := settings["commodity_options"].([]CommodityOption)
	seen := map[string]bool{}
	for _, option := range options {
		if option.ID == "" || option.Name == "" || seen[option.ID] {
			t.Fatalf("invalid/duplicate commodity option: %v", option)
		}
		seen[option.ID] = true
	}
	if !seen["84"] || !seen["43"] || len(options) != 87 {
		t.Fatal("DTDC catalogue missing expected commodities")
	}
	services := settings["service_type_options"].([]string)
	if len(services) != 2 || services[0] != "B2C PREMIUM" || services[1] != "B2C PRIORITY" {
		t.Fatalf("unexpected service examples: %v", services)
	}
}

func TestAmbiguousBookingResponsesNeverAllowAutomaticRetry(t *testing.T) {
	for _, body := range []string{
		`{"status":"OK","data":[{"reference_number":"D123"}]}`,
		`{"status":"OK","data":[{"success":false,"reference_number":"D123","message":"partial response"}]}`,
		`{"status":"OK","data":[{}]}`,
	} {
		client, _ := fixture()
		mockResponse(client, 200, body, nil)
		_, err := client.Book(context.Background(), map[string]any{})
		var rejected *RejectedError
		if err == nil || errors.As(err, &rejected) {
			t.Fatalf("ambiguous response was treated as safely retryable: %s, %v", body, err)
		}
	}
}

func TestTrackingReferenceRecoveryRequiresExactOrderAndAccount(t *testing.T) {
	for _, tt := range []struct {
		name, reference, account string
		ok                       bool
	}{
		{"matching order", "DO-1001", "SELLER", true},
		{"wrong order", "DO-OTHER", "SELLER", false},
		{"wrong account", "DO-1001", "ANOTHER-SELLER", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := fixture()
			body, _ := json.Marshal(map[string]any{"statusFlag": true, "status": "SUCCESS", "trackHeader": map[string]any{"strShipmentNo": "D123", "strStatus": "Booked", "strRefNo": tt.reference, "strCNActCustCode": tt.account, "strConnectionDateTime": nil}, "trackDetails": nil})
			mockResponse(client, 200, string(body), func(r *http.Request) {
				var payload map[string]string
				if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["trkType"] != "reference" || payload["strcnno"] != "DO-1001" {
					t.Fatal("recovery used incorrect reference request")
				}
			})
			result, err := client.TrackReference(context.Background(), "DO-1001")
			if (err == nil) != tt.ok {
				t.Fatalf("unexpected recovery result: %v", err)
			}
			if tt.ok && result.Details == nil {
				t.Fatal("null scan array must normalize to an empty array")
			}
		})
	}
}
