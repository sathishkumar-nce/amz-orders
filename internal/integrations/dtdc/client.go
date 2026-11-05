package dtdc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type Config struct {
	BaseURL, TrackingURL, APIKey, TrackingToken, CustomerCode, ServiceType, Commodity string
	Origin                                                                            Address
}
type Address struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Address        string `json:"address_line_1"`
	Pincode        string `json:"pincode"`
	City           string `json:"city"`
	State          string `json:"state"`
	AlternatePhone string `json:"alternate_phone,omitempty"`
	Email          string `json:"email,omitempty"`
	AddressLine2   string `json:"address_line_2,omitempty"`
}
type Piece struct {
	Description string `json:"description"`
	Value       string `json:"declared_value"`
	Weight      string `json:"weight"`
	Length      string `json:"length"`
	Width       string `json:"width"`
	Height      string `json:"height"`
}
type BookingOptions struct {
	ServiceType   string  `json:"service_type_id"`
	Commodity     string  `json:"commodity_id,omitempty"`
	Pieces        []Piece `json:"pieces_detail,omitempty"`
	EwayBill      string  `json:"eway_bill,omitempty"`
	RiskSurcharge bool    `json:"is_risk_surcharge_applicable"`
}
type Booking struct {
	AWB string
	Raw json.RawMessage
}
type Tracking struct {
	StatusFlag bool                `json:"statusFlag"`
	Status     string              `json:"status"`
	Header     map[string]string   `json:"trackHeader"`
	Details    []map[string]string `json:"trackDetails"`
}
type RejectedError struct{ Message string }

func (e *RejectedError) Error() string { return e.Message }

type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://pxapi.dtdc.in"
	}
	if cfg.TrackingURL == "" {
		cfg.TrackingURL = "https://blktracksvc.dtdc.com"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 45 * time.Second}}
}
func (c *Client) MissingConfig() []string {
	missing := []string{}
	for _, v := range []struct{ k, v string }{
		{"DTDC_API_KEY", c.cfg.APIKey}, {"DTDC_CUSTOMER_CODE", c.cfg.CustomerCode},
		{"DTDC_SERVICE_TYPE", c.cfg.ServiceType}, {"DTDC_COMMODITY", c.cfg.Commodity},
		{"DTDC_ORIGIN_NAME", c.cfg.Origin.Name}, {"DTDC_ORIGIN_PHONE", c.cfg.Origin.Phone},
		{"DTDC_ORIGIN_ADDRESS", c.cfg.Origin.Address}, {"DTDC_ORIGIN_PINCODE", c.cfg.Origin.Pincode},
		{"DTDC_ORIGIN_CITY", c.cfg.Origin.City}, {"DTDC_ORIGIN_STATE", c.cfg.Origin.State},
	} {
		if strings.TrimSpace(v.v) == "" {
			missing = append(missing, v.k)
		}
	}
	return missing
}
func (c *Client) Settings() map[string]any {
	return map[string]any{
		"missing_config": c.MissingConfig(), "service_type": c.cfg.ServiceType,
		"commodity": c.cfg.Commodity, "commodity_options": commodityOptions,
		"service_type_options": documentedServiceTypes,
		"tracking_enabled":     c.cfg.TrackingToken != "",
	}
}
func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func (c *Client) BuildBooking(order *models.DirectOrder, opts BookingOptions) (map[string]any, error) {
	missing := []string{}
	for _, key := range c.MissingConfig() {
		if key == "DTDC_SERVICE_TYPE" && strings.TrimSpace(opts.ServiceType) != "" {
			continue
		}
		if key == "DTDC_COMMODITY" && strings.TrimSpace(opts.Commodity) != "" {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("DTDC configuration missing: %s", strings.Join(missing, ", "))
	}

	if !regexp.MustCompile(`^\d{6}$`).MatchString(c.cfg.Origin.Pincode) {
		return nil, fmt.Errorf("DTDC_ORIGIN_PINCODE must contain six digits")
	}
	if !regexp.MustCompile(`^\d{10}$`).MatchString(c.cfg.Origin.Phone) {
		return nil, fmt.Errorf("DTDC_ORIGIN_PHONE must contain ten digits")
	}
	if order.Country.String != "" && !strings.EqualFold(order.Country.String, "India") && !strings.EqualFold(order.Country.String, "IN") {
		return nil, fmt.Errorf("DTDC booking supports domestic Indian destinations only")
	}
	if order.AWB.String != "" || order.CourierOrderID.String != "" {
		return nil, fmt.Errorf("order already has a shipment; cannot book again")
	}
	if order.OrderStatus == "cancelled" {
		return nil, fmt.Errorf("cannot book a cancelled order")
	}
	if order.ShipmentType.String != "" && !strings.EqualFold(order.ShipmentType.String, "forward") {
		return nil, fmt.Errorf("DTDC booking supports forward orders only")
	}
	for _, v := range []struct{ k, v string }{{"customer name", order.CustomerName.String}, {"phone", order.Mobile.String}, {"address", order.Address.String}, {"city", order.City.String}, {"state", order.State.String}} {
		if strings.TrimSpace(v.v) == "" {
			return nil, fmt.Errorf("DTDC requires %s", v.k)
		}
	}
	if !regexp.MustCompile(`^\d{10}$`).MatchString(order.Mobile.String) {
		return nil, fmt.Errorf("DTDC requires a ten-digit destination phone number")
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(order.Pincode.String) {
		return nil, fmt.Errorf("DTDC requires a six-digit destination pincode")
	}
	if order.TotalWeight.Float64 <= 0 || order.LengthCM.Float64 <= 0 || order.WidthCM.Float64 <= 0 || order.HeightCM.Float64 <= 0 || order.Amount.Float64 <= 0 {
		return nil, fmt.Errorf("DTDC requires positive shipment weight, length, width, height and declared amount")
	}
	if order.CODAmount.Float64 < 0 || order.CODAmount.Float64 > order.Amount.Float64 {
		return nil, fmt.Errorf("COD amount must be between zero and declared amount")
	}
	count := order.PackageCount.Int64
	if count < 1 {
		count = 1
	}
	if count > 1 && len(opts.Pieces) != int(count) {
		return nil, fmt.Errorf("DTDC requires dimensions, kg weight and declared value for each of the %d parcels", count)
	}
	if count == 1 && len(opts.Pieces) > 0 {
		return nil, fmt.Errorf("piece details are only required for multiple parcels")
	}
	for _, p := range opts.Pieces {
		if strings.TrimSpace(p.Description) == "" {
			return nil, fmt.Errorf("each parcel needs a description")
		}
		for _, v := range []string{p.Value, p.Weight, p.Length, p.Width, p.Height} {
			n, e := strconv.ParseFloat(v, 64)
			if e != nil || n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("each parcel needs positive value, kg weight and dimensions")
			}
		}
	}
	if opts.EwayBill != "" && !regexp.MustCompile(`^\d{12}$`).MatchString(opts.EwayBill) {
		return nil, fmt.Errorf("e-way bill must contain 12 digits")
	}
	service := strings.TrimSpace(opts.ServiceType)
	if service == "" {
		service = c.cfg.ServiceType
	}
	commodity := strings.TrimSpace(opts.Commodity)
	if commodity == "" {
		commodity = strings.TrimSpace(c.cfg.Commodity)
	}
	shipment := map[string]any{
		"customer_code": c.cfg.CustomerCode, "service_type_id": service, "load_type": "NON-DOCUMENT", "consignment_type": "Forward",
		"description": commodityName(commodity), "commodity_id": commodity, "dimension_unit": "CM", "weight_unit": "KG",
		"length": number(order.LengthCM.Float64), "width": number(order.WidthCM.Float64), "height": number(order.HeightCM.Float64),
		"weight": number(order.TotalWeight.Float64 / 1000), "declared_value": number(order.Amount.Float64), "num_pieces": strconv.FormatInt(count, 10),
		"origin_details": c.cfg.Origin, "destination_details": Address{Name: order.CustomerName.String, Phone: order.Mobile.String, Address: order.Address.String, Pincode: order.Pincode.String, City: order.City.String, State: order.State.String, AlternatePhone: order.AlternateMobile.String, Email: order.Email.String, AddressLine2: order.Landmark.String},
		"customer_reference_number": order.OrderID, "invoice_number": order.OrderID, "reference_number": "",
		"is_risk_surcharge_applicable": opts.RiskSurcharge, "cod_amount": "0", "cod_collection_mode": "",
	}
	if order.CODAmount.Float64 > 0 {
		shipment["cod_amount"] = number(order.CODAmount.Float64)
		shipment["cod_collection_mode"] = "CASH"
	}
	if order.InvoiceDate.Valid {
		shipment["invoice_date"] = order.InvoiceDate.Time.Format("02 Jan 2006")
	}
	if len(opts.Pieces) > 0 {
		shipment["pieces_detail"] = opts.Pieces
	}
	if opts.EwayBill != "" {
		shipment["eway_bill"] = opts.EwayBill
	}
	return map[string]any{"consignments": []any{shipment}}, nil
}
func (c *Client) request(ctx context.Context, method, endpoint, header, token string, body any) ([]byte, error) {
	if token == "" {
		return nil, fmt.Errorf("DTDC credentials are not configured")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set(header, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DTDC request did not complete; verify the shipment before retrying")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("could not read DTDC response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("DTDC returned HTTP %d", resp.StatusCode)
		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, &RejectedError{msg}
		}
		return nil, fmt.Errorf("%s; verify the shipment before retrying", msg)
	}
	return raw, nil
}

const consignmentPath = "/api/customer/integration/consignment/"

func (c *Client) Book(ctx context.Context, payload map[string]any) (*Booking, error) {
	raw, err := c.request(ctx, "POST", strings.TrimRight(c.cfg.BaseURL, "/")+consignmentPath+"softdata", "api-key", c.cfg.APIKey, payload)
	if err != nil {
		return nil, err
	}
	var response struct {
		Status string `json:"status"`
		Data   []struct {
			Success *bool  `json:"success"`
			AWB     string `json:"reference_number"`
			Message string `json:"message"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || len(response.Data) != 1 {
		return nil, fmt.Errorf("invalid DTDC booking response; reconcile before retrying")
	}
	item := response.Data[0]
	if item.Success == nil || (!*item.Success && item.AWB != "") {
		return nil, fmt.Errorf("DTDC booking response is ambiguous; reconcile before retrying")
	}
	if !*item.Success {
		message := strings.TrimSpace(item.Message)
		if message == "" {
			message = "check address, service and account details"
		}
		if len(message) > 500 {
			message = message[:500]
		}
		return nil, &RejectedError{"DTDC rejected the consignment: " + message}
	}
	if response.Status != "OK" || item.AWB == "" {
		return nil, fmt.Errorf("DTDC response did not confirm an AWB; reconcile before retrying")
	}
	return &Booking{item.AWB, raw}, nil
}
func (c *Client) tracking(ctx context.Context, kind, reference string) (*Tracking, error) {
	raw, err := c.request(ctx, "POST", strings.TrimRight(c.cfg.TrackingURL, "/")+"/dtdc-api/rest/JSONCnTrk/getTrackDetails", "x-access-token", c.cfg.TrackingToken, map[string]string{"trkType": kind, "strcnno": reference, "addtnlDtl": "Y"})
	if err != nil {
		return nil, err
	}
	var result Tracking
	if json.Unmarshal(raw, &result) != nil {
		return nil, fmt.Errorf("invalid DTDC tracking response")
	}
	if !result.StatusFlag || result.Status != "SUCCESS" || result.Header["strStatus"] == "" || result.Header["strShipmentNo"] == "" {
		return nil, fmt.Errorf("DTDC tracking is not available yet; the saved status has been retained")
	}
	if account := result.Header["strCNActCustCode"]; account != "" && c.cfg.CustomerCode != "" && !strings.EqualFold(account, c.cfg.CustomerCode) {
		return nil, fmt.Errorf("DTDC tracking returned a different customer account")
	}
	if result.Details == nil {
		result.Details = []map[string]string{}
	}
	return &result, nil
}
func (c *Client) Track(ctx context.Context, awb string) (*Tracking, error) {
	result, err := c.tracking(ctx, "cnno", awb)
	if err != nil {
		return nil, err
	}
	if result.Header["strShipmentNo"] != awb {
		return nil, fmt.Errorf("DTDC tracking returned a different shipment")
	}
	return result, nil
}
func (c *Client) TrackReference(ctx context.Context, reference string) (*Tracking, error) {
	result, err := c.tracking(ctx, "reference", reference)
	if err != nil {
		return nil, err
	}
	if result.Header["strRefNo"] != reference {
		return nil, fmt.Errorf("DTDC tracking did not confirm this order reference; booking remains locked for review")
	}
	return result, nil
}
func (c *Client) Cancel(ctx context.Context, awb string) (json.RawMessage, error) {
	if c.cfg.CustomerCode == "" {
		return nil, fmt.Errorf("DTDC customer code is not configured")
	}
	raw, err := c.request(ctx, "POST", strings.TrimRight(c.cfg.BaseURL, "/")+consignmentPath+"cancel", "api-key", c.cfg.APIKey, map[string]any{"AWBNo": []string{awb}, "customerCode": c.cfg.CustomerCode})
	if err != nil {
		return nil, err
	}
	var result struct {
		Status  string `json:"status"`
		Success bool   `json:"success"`
		Items   []struct {
			Success bool   `json:"success"`
			AWB     string `json:"reference_number"`
		} `json:"successConsignments"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return nil, fmt.Errorf("invalid DTDC cancellation response")
	}
	for _, item := range result.Items {
		if result.Status == "OK" && result.Success && item.Success && item.AWB == awb {
			return raw, nil
		}
	}
	return nil, fmt.Errorf("DTDC did not confirm cancellation; shipment status has been retained")
}
func (c *Client) Label(ctx context.Context, awb, code string) ([]byte, error) {
	switch code {
	case "SHIP_LABEL_A4", "SHIP_LABEL_A6", "SHIP_LABEL_POD", "SHIP_LABEL_4X6", "ROUTE_LABEL_A4", "ROUTE_LABEL_4X4", "INVOICE", "ADDR_LABEL_A4", "ADDR_LABEL_4X2":
	default:
		return nil, fmt.Errorf("unsupported label code")
	}
	query := url.Values{"reference_number": {awb}, "label_code": {code}, "label_format": {"pdf"}}
	raw, err := c.request(ctx, "GET", strings.TrimRight(c.cfg.BaseURL, "/")+consignmentPath+"shippinglabel/stream?"+query.Encode(), "api-key", c.cfg.APIKey, nil)
	if err == nil && !bytes.HasPrefix(raw, []byte("%PDF-")) {
		return nil, fmt.Errorf("DTDC did not return a PDF label")
	}
	return raw, err
}
