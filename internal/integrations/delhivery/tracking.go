package delhivery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func stringField(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func (c *Client) Track(ctx context.Context, awb string) (*models.ShipmentTracking, json.RawMessage, error) {
	if !c.Enabled() {
		return nil, nil, fmt.Errorf("Delhivery tracking credentials are not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/packages/json/?"+url.Values{"waybill": {awb}}.Encode(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Token "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Delhivery tracking request did not complete")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("could not read Delhivery tracking response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("Delhivery tracking returned HTTP %d; saved status retained", response.StatusCode)
	}
	var result struct {
		ShipmentData []struct {
			Shipment map[string]any `json:"Shipment"`
		} `json:"ShipmentData"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.ShipmentData) != 1 {
		return nil, nil, fmt.Errorf("Delhivery tracking is unavailable for this AWB; saved status retained")
	}
	shipment := result.ShipmentData[0].Shipment
	if stringField(shipment, "AWB", "Waybill") != awb {
		return nil, nil, fmt.Errorf("Delhivery returned a different shipment")
	}
	status, _ := shipment["Status"].(map[string]any)
	if stringField(status, "Status") == "" {
		return nil, nil, fmt.Errorf("Delhivery did not return a shipment status; saved status retained")
	}
	snapshot := &models.ShipmentTracking{Courier: "delhivery", AWB: awb, Status: stringField(status, "Status"), ReportedStatus: stringField(status, "Status"), CurrentLocation: stringField(status, "StatusLocation"), LastEventAt: stringField(status, "StatusDateTime"), Remarks: stringField(status, "Instructions"), Origin: stringField(shipment, "Origin"), Destination: stringField(shipment, "Destination"), ExpectedDelivery: stringField(shipment, "ExpectedDeliveryDate", "Expected Delivery Date"), TrackingURL: "https://www.delhivery.com/track-v2/package/" + url.PathEscape(awb), CheckedAt: time.Now().UTC(), Events: []models.ShipmentTrackingEvent{}}
	if scans, ok := shipment["Scans"].([]any); ok {
		for _, scan := range scans {
			entry, ok := scan.(map[string]any)
			if !ok {
				continue
			}
			detail, ok := entry["ScanDetail"].(map[string]any)
			if !ok {
				detail = entry
			}
			snapshot.Events = append(snapshot.Events, models.ShipmentTrackingEvent{Status: stringField(detail, "Scan", "Status"), Location: stringField(detail, "ScannedLocation", "StatusLocation", "Location"), OccurredAt: stringField(detail, "ScanDateTime", "StatusDateTime"), Remarks: stringField(detail, "Instructions"), Code: stringField(detail, "StatusCode")})
		}
	}
	return snapshot, raw, nil
}
