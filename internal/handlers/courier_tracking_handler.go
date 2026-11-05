package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/dtdc"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func (h *DirectOrderHandler) ListCourierShipments(c *gin.Context) {
	filters := parseDirectOrderFilters(c)
	filters.BookedOnly = true
	courier := strings.ToLower(strings.TrimSpace(c.Query("courier")))
	if courier != "" && courier != "all" && courier != "dtdc" && courier != "delhivery" {
		c.JSON(400, gin.H{"error": "Choose DTDC, Delhivery or all couriers"})
		return
	}
	if courier != "all" {
		filters.CourierType = courier
	}
	filters.CourierStatus = strings.TrimSpace(c.Query("courier_status"))
	orders, total, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not load courier shipments"})
		return
	}
	data := make([]*models.DirectOrderResponse, 0, len(orders))
	for i := range orders {
		data = append(data, models.ToDirectOrderResponse(&orders[i]))
	}
	c.JSON(200, gin.H{"data": data, "total": total, "page": filters.Page, "limit": filters.Limit, "total_pages": (total + filters.Limit - 1) / filters.Limit})
}
func trackingTime(date, clock string) string {
	if date == "" {
		return ""
	}
	zone := time.FixedZone("IST", 19800)
	if clock == "" {
		if parsed, err := time.Parse("02012006", date); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	text := date + " " + clock
	for _, format := range []string{"02012006 150405", "02012006 1504", "02012006 15:04:05", "02012006 "} {
		if parsed, err := time.ParseInLocation(format, text, zone); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return strings.TrimSpace(text)
}
func dtdcSummary(result *dtdc.Tracking, awb string) *models.ShipmentTracking {
	header := result.Header
	snapshot := &models.ShipmentTracking{Courier: "dtdc", AWB: awb, Status: header["strStatus"], ReportedStatus: header["strStatus"], Origin: header["strOrigin"], Destination: header["strDestination"], ExpectedDelivery: trackingTime(header["strRevExpectedDeliveryDate"], ""), LastEventAt: trackingTime(header["strStatusTransOn"], header["strStatusTransTime"]), Remarks: header["strRemarks"], TrackingURL: "https://www.dtdc.com/track-your-shipment/?" + url.Values{"awb": {awb}}.Encode(), CheckedAt: time.Now().UTC(), Events: []models.ShipmentTrackingEvent{}}
	if snapshot.ExpectedDelivery == "" {
		snapshot.ExpectedDelivery = trackingTime(header["strExpectedDeliveryDate"], "")
	}
	for _, detail := range result.Details {
		remarks := detail["sTrRemarks"]
		if remarks == "" {
			remarks = detail["strRemarks"]
		}
		snapshot.Events = append(snapshot.Events, models.ShipmentTrackingEvent{Status: detail["strAction"], Location: detail["strOrigin"], Destination: detail["strDestination"], OccurredAt: trackingTime(detail["strActionDate"], detail["strActionTime"]), Remarks: remarks, Code: detail["strCode"]})
	}
	// Latest scans come first in the shared panel and WhatsApp preview.
	sort.SliceStable(snapshot.Events, func(i, j int) bool { return snapshot.Events[i].OccurredAt > snapshot.Events[j].OccurredAt })
	for _, event := range snapshot.Events {
		if event.Location != "" {
			snapshot.CurrentLocation = event.Location
			break
		}
	}
	return snapshot
}
func (h *DirectOrderHandler) TrackShipment(c *gin.Context) {
	order := h.dtdcOrder(c, false)
	if order == nil {
		return
	}
	if order.AWB.String == "" {
		c.JSON(400, gin.H{"error": "This order does not have a tracking ID yet"})
		return
	}
	courier := strings.ToLower(strings.TrimSpace(order.CourierType.String))
	var snapshot *models.ShipmentTracking
	var raw json.RawMessage
	var err error
	switch courier {
	case "dtdc":
		var tracking *dtdc.Tracking
		tracking, err = h.dtdcClient.Track(c.Request.Context(), order.AWB.String)
		if err == nil {
			snapshot = dtdcSummary(tracking, order.AWB.String)
			raw, err = json.Marshal(tracking)
		}
	case "delhivery":
		snapshot, raw, err = h.delhiveryClient.Track(c.Request.Context(), order.AWB.String)
	default:
		c.JSON(400, gin.H{"error": "Live tracking is available for DTDC and Delhivery shipments"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err = h.repo.SaveShipmentTracking(ctx, order.OrderID, order.AWB.String, courier, snapshot, raw, currentActorName(c)); err != nil {
		c.JSON(409, gin.H{"error": "The shipment changed while tracking was refreshed. Reload the order."})
		return
	}
	updated, err := h.repo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Tracking saved, but the order could not be reloaded"})
		return
	}
	c.JSON(200, gin.H{"order": models.ToDirectOrderResponse(updated), "tracking": snapshot})
}
