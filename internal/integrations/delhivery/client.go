package delhivery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type Config struct {
	BaseURL               string
	APIToken              string
	ClientName            string
	DefaultPickupLocation string
	SellerName            string
	SellerAddress         string
	SellerGSTIN           string
	ClientGSTIN           string
}

type Client struct {
	baseURL    string
	apiToken   string
	config     Config
	httpClient *http.Client
	stateNames map[string]string
}

type createOrderPayload struct {
	Shipments      []shipmentPayload `json:"shipments"`
	PickupLocation map[string]string `json:"pickup_location,omitempty"`
}

type shipmentPayload struct {
	Name           string  `json:"name"`
	Add            string  `json:"add"`
	City           string  `json:"city"`
	State          string  `json:"state"`
	Country        string  `json:"country"`
	Pin            string  `json:"pin"`
	Phone          string  `json:"phone"`
	Order          string  `json:"order"`
	PaymentMode    string  `json:"payment_mode"`
	ProductsDesc   string  `json:"products_desc"`
	Quantity       int     `json:"quantity"`
	CODAmount      float64 `json:"cod_amount,omitempty"`
	TotalAmount    float64 `json:"total_amount,omitempty"`
	OrderDate      string  `json:"order_date"`
	SellerName     string  `json:"seller_name,omitempty"`
	SellerAdd      string  `json:"seller_add,omitempty"`
	SellerInv      string  `json:"seller_inv,omitempty"`
	HSNCode        string  `json:"hsn_code,omitempty"`
	SellerGSTTIN   string  `json:"seller_gst_tin,omitempty"`
	ClientGSTTIN   string  `json:"client_gst_tin,omitempty"`
	ShipmentWidth  float64 `json:"shipment_width,omitempty"`
	ShipmentHeight float64 `json:"shipment_height,omitempty"`
	ShipmentLength float64 `json:"shipment_length,omitempty"`
	Weight         float64 `json:"weight,omitempty"`
}

type createOrderResponse struct {
	Success  bool `json:"success"`
	Packages []struct {
		Status   string   `json:"status"`
		Waybill  string   `json:"waybill"`
		Refnum   string   `json:"refnum"`
		Remarks  []string `json:"remarks"`
		Client   string   `json:"client"`
		Sortcode string   `json:"sort_code"`
	} `json:"packages"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type pincodeLookupResponse struct {
	DeliveryCodes []json.RawMessage `json:"delivery_codes"`
	PostalCodes   []json.RawMessage `json:"postal_codes"`
	PinCodes      []json.RawMessage `json:"pin_codes"`
	Data          []json.RawMessage `json:"data"`
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://track.delhivery.com"
	}

	return &Client{
		baseURL:  baseURL,
		apiToken: strings.TrimSpace(cfg.APIToken),
		config:   cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stateNames: loadStateCodeMap(),
	}
}

func (c *Client) Enabled() bool {
	return c.apiToken != ""
}

func (c *Client) LookupPincode(ctx context.Context, pincode string) (*models.DirectOrderPincodeLookupResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Delhivery integration is not configured. Set DELHIVERY_API_TOKEN in the backend environment and restart the service")
	}

	trimmedPincode := strings.TrimSpace(pincode)
	if trimmedPincode == "" {
		return nil, fmt.Errorf("pincode is required")
	}

	endpoint := fmt.Sprintf("%s/c/api/pin-codes/json/?filter_codes=%s", c.baseURL, url.QueryEscape(trimmedPincode))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Delhivery pincode lookup failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded pincodeLookupResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode Delhivery pincode response: %w", err)
	}

	entries := decoded.DeliveryCodes
	if len(entries) == 0 {
		entries = decoded.PostalCodes
	}
	if len(entries) == 0 {
		entries = decoded.PinCodes
	}
	if len(entries) == 0 {
		entries = decoded.Data
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no Delhivery pincode data found for %s", trimmedPincode)
	}

	candidates := make([]models.DirectOrderPincodeLookupCandidate, 0, len(entries))
	for _, entry := range entries {
		candidate, ok := parsePincodeCandidate(entry)
		if !ok {
			continue
		}
		if candidate.Pincode == "" {
			candidate.Pincode = trimmedPincode
		}
		c.fillStateName(&candidate)
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no Delhivery pincode data found for %s", trimmedPincode)
	}

	selected := candidates[0]
	return &models.DirectOrderPincodeLookupResult{
		Pincode:     selected.Pincode,
		City:        selected.City,
		District:    selected.District,
		State:       selected.State,
		StateCode:   selected.StateCode,
		Country:     fallback(selected.Country, "India"),
		Serviceable: selected.Serviceable,
		COD:         selected.COD,
		Prepaid:     selected.Prepaid,
		Raw:         candidates,
	}, nil
}

func (c *Client) CreateForwardOrder(ctx context.Context, order *models.DirectOrder) (*models.DelhiveryForwardOrderResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Delhivery token is not configured. Set DELHIVERY_API_TOKEN and restart the backend")
	}
	if strings.TrimSpace(c.config.SellerGSTIN) == "" {
		return nil, fmt.Errorf("Delhivery seller GSTIN is required. Set DELHIVERY_SELLER_GSTIN in the backend environment and restart the service")
	}

	pickupLocation := strings.TrimSpace(valueOrDefault(order.PickupLocation, c.config.DefaultPickupLocation))
	if pickupLocation == "" {
		return nil, fmt.Errorf("pickup location is required before sending an order to Delhivery")
	}
	shipment, err := c.buildShipment(order)
	if err != nil {
		return nil, err
	}

	payload := createOrderPayload{
		Shipments:      []shipmentPayload{shipment},
		PickupLocation: map[string]string{"name": pickupLocation},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("format", "json")
	form.Set("data", string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/cmu/create.json", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.apiToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Delhivery create order failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded createOrderResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode Delhivery response: %w", err)
	}

	if !decoded.Success && len(decoded.Packages) == 0 && decoded.Error != "" {
		return nil, fmt.Errorf("Delhivery create order failed: %s", decoded.Error)
	}

	result := &models.DelhiveryForwardOrderResult{
		CourierName:    "Delhivery",
		CourierPayload: json.RawMessage(raw),
	}

	if len(decoded.Packages) > 0 {
		first := decoded.Packages[0]
		if first.Waybill != "" {
			result.AWB = &first.Waybill
		}
		if first.Refnum != "" {
			result.CourierOrderID = &first.Refnum
		}
		if first.Status != "" {
			result.CourierStatus = &first.Status
		} else if len(first.Remarks) > 0 {
			status := strings.Join(first.Remarks, "; ")
			result.CourierStatus = &status
		}
		now := time.Now()
		result.ManifestedAt = &now
	}

	if result.CourierOrderID == nil {
		ref := order.OrderID
		result.CourierOrderID = &ref
	}
	if result.CourierStatus == nil {
		status := "submitted"
		result.CourierStatus = &status
	}

	return result, nil
}

func (c *Client) buildShipment(order *models.DirectOrder) (shipmentPayload, error) {
	if strings.TrimSpace(order.OrderID) == "" {
		return shipmentPayload{}, fmt.Errorf("order_id is required")
	}
	var missingFields []string
	if !order.CustomerName.Valid || strings.TrimSpace(order.CustomerName.String) == "" {
		missingFields = append(missingFields, "Customer Name")
	}
	if !order.Address.Valid || strings.TrimSpace(order.Address.String) == "" {
		missingFields = append(missingFields, "Address")
	}
	if !order.Pincode.Valid || strings.TrimSpace(order.Pincode.String) == "" {
		missingFields = append(missingFields, "Pincode")
	}
	if !order.Mobile.Valid || strings.TrimSpace(order.Mobile.String) == "" {
		missingFields = append(missingFields, "Mobile")
	}
	if !order.City.Valid || strings.TrimSpace(order.City.String) == "" {
		missingFields = append(missingFields, "City")
	}
	if !order.State.Valid || strings.TrimSpace(order.State.String) == "" {
		missingFields = append(missingFields, "State")
	}

	quantity := 0
	var productParts []string
	var hsnCodes []string
	totalWeight := 0.0

	if len(order.Items) == 0 {
		missingFields = append(missingFields, "At least one item")
	}

	for _, item := range order.Items {
		if item.Quantity > 0 {
			quantity += item.Quantity
		}
		name := strings.TrimSpace(nullString(item.Item))
		if name != "" {
			productParts = append(productParts, name)
		}
		if item.HSN.Valid && strings.TrimSpace(item.HSN.String) != "" {
			hsnCodes = append(hsnCodes, item.HSN.String)
		}
		if item.Weight.Valid {
			totalWeight += item.Weight.Float64 * float64(max(item.Quantity, 1))
		}
	}
	if quantity == 0 {
		quantity = 1
	}
	if len(hsnCodes) == 0 {
		missingFields = append(missingFields, "At least one item HSN code")
	}

	weight := totalWeight
	if order.TotalWeight.Valid && order.TotalWeight.Float64 > 0 {
		weight = order.TotalWeight.Float64
	}
	if weight <= 0 {
		missingFields = append(missingFields, "Total Weight (grams)")
	}
	if !order.LengthCM.Valid || order.LengthCM.Float64 <= 0 {
		missingFields = append(missingFields, "Length")
	}
	if !order.WidthCM.Valid || order.WidthCM.Float64 <= 0 {
		missingFields = append(missingFields, "Width")
	}
	if !order.HeightCM.Valid || order.HeightCM.Float64 <= 0 {
		missingFields = append(missingFields, "Height")
	}

	if len(missingFields) > 0 {
		return shipmentPayload{}, fmt.Errorf(
			"Missing required fields for Delhivery forward order:\n- %s",
			strings.Join(missingFields, "\n- "),
		)
	}

	paymentMode := "Pre-paid"
	codAmount := 0.0
	if order.CODAmount.Valid && order.CODAmount.Float64 > 0 {
		paymentMode = "COD"
		codAmount = order.CODAmount.Float64
	}

	totalAmount := 0.0
	if order.Amount.Valid {
		totalAmount = order.Amount.Float64
	}

	orderDate := order.CreatedAt.Format("2006-01-02")
	if orderDate == "" {
		orderDate = time.Now().Format("2006-01-02")
	}

	hsnCode := ""
	if len(hsnCodes) > 0 {
		hsnCode = strings.Join(hsnCodes, ",")
	}

	sellerName := strings.TrimSpace(c.config.SellerName)
	if sellerName == "" {
		sellerName = strings.TrimSpace(c.config.ClientName)
	}

	return shipmentPayload{
		Name:           order.CustomerName.String,
		Add:            order.Address.String,
		City:           order.City.String,
		State:          order.State.String,
		Country:        valueOrDefault(order.Country, "India"),
		Pin:            order.Pincode.String,
		Phone:          order.Mobile.String,
		Order:          order.OrderID,
		PaymentMode:    paymentMode,
		ProductsDesc:   fallback(strings.Join(productParts, ", "), "Direct order"),
		Quantity:       quantity,
		CODAmount:      codAmount,
		TotalAmount:    totalAmount,
		OrderDate:      orderDate,
		SellerName:     sellerName,
		SellerAdd:      strings.TrimSpace(c.config.SellerAddress),
		SellerInv:      order.OrderID,
		HSNCode:        hsnCode,
		SellerGSTTIN:   strings.TrimSpace(c.config.SellerGSTIN),
		ClientGSTTIN:   strings.TrimSpace(c.config.ClientGSTIN),
		ShipmentWidth:  positiveOrZero(order.WidthCM),
		ShipmentHeight: positiveOrZero(order.HeightCM),
		ShipmentLength: positiveOrZero(order.LengthCM),
		Weight:         weight,
	}, nil
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func positiveOrZero(value sql.NullFloat64) float64 {
	if !value.Valid || value.Float64 <= 0 {
		return 0
	}
	return value.Float64
}

func valueOrDefault(value sql.NullString, fallback string) string {
	if value.Valid && strings.TrimSpace(value.String) != "" {
		return strings.TrimSpace(value.String)
	}
	return fallback
}

func fallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func max(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func (c *Client) fillStateName(candidate *models.DirectOrderPincodeLookupCandidate) {
	if candidate == nil {
		return
	}
	if strings.TrimSpace(candidate.State) != "" {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(candidate.StateCode))
	if code == "" {
		return
	}
	if stateName, ok := c.stateNames[code]; ok {
		candidate.State = stateName
	}
}

func parsePincodeCandidate(raw json.RawMessage) (models.DirectOrderPincodeLookupCandidate, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return models.DirectOrderPincodeLookupCandidate{}, false
	}

	normalized := payload
	if nested, ok := payload["postal_code"].(map[string]interface{}); ok && len(nested) > 0 {
		normalized = mergeStringMap(payload, nested)
	}

	candidate := models.DirectOrderPincodeLookupCandidate{
		Pincode:     firstNonEmptyString(normalized, "pin", "pincode", "postal_code", "postcode"),
		City:        firstNonEmptyString(normalized, "city", "city_name", "taluk", "location"),
		District:    firstNonEmptyString(normalized, "district", "district_name"),
		State:       firstNonEmptyString(normalized, "state", "state_name"),
		StateCode:   firstNonEmptyString(normalized, "state_code", "st"),
		Country:     firstNonEmptyString(normalized, "country", "country_name"),
		Serviceable: firstTruthy(normalized, "serviceable", "is_serviceable"),
		COD:         firstTruthy(normalized, "cod", "cash", "cash_on_delivery"),
		Prepaid:     firstTruthy(normalized, "prepaid", "pre_paid"),
	}

	if !candidate.Serviceable {
		// Delhivery commonly signals serviceability through COD/prepaid availability.
		candidate.Serviceable = candidate.COD || candidate.Prepaid || len(normalized) > 0
	}

	return candidate, candidate.Pincode != "" || candidate.City != "" || candidate.State != "" || candidate.District != ""
}

func firstNonEmptyString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		case float64:
			if value == float64(int64(value)) {
				return strconv.FormatInt(int64(value), 10)
			}
			return strings.TrimSpace(strconv.FormatFloat(value, 'f', -1, 64))
		case json.Number:
			return strings.TrimSpace(value.String())
		}
	}
	return ""
}

func firstTruthy(values map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case bool:
			return value
		case string:
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "y", "yes", "true", "1":
				return true
			case "n", "no", "false", "0":
				return false
			}
		case float64:
			return value != 0
		case json.Number:
			parsed, err := value.Int64()
			return err == nil && parsed != 0
		}
	}
	return false
}

func mergeStringMap(base map[string]interface{}, override map[string]interface{}) map[string]interface{} {
	if len(base) == 0 {
		return override
	}
	merged := make(map[string]interface{}, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func loadStateCodeMap() map[string]string {
	paths := []string{
		"state_codes.json",
		"../state_codes.json",
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		paths = append(paths, filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "state_codes.json"))
	}

	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var stateNames map[string]string
		if err := json.Unmarshal(raw, &stateNames); err != nil {
			continue
		}

		normalized := make(map[string]string, len(stateNames))
		for code, name := range stateNames {
			trimmedCode := strings.ToUpper(strings.TrimSpace(code))
			trimmedName := strings.TrimSpace(name)
			if trimmedCode == "" || trimmedName == "" {
				continue
			}
			normalized[trimmedCode] = trimmedName
		}
		if len(normalized) > 0 {
			return normalized
		}
	}

	return map[string]string{}
}
