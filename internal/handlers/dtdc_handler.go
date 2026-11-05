package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/dtdc"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func (h *DirectOrderHandler) SetDTDCClient(client *dtdc.Client) { h.dtdcClient = client }
func (h *DirectOrderHandler) DTDCSettings(c *gin.Context) {
	settings, err := h.courierSettings(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	data := h.dtdcClient.Settings()
	data["service_type"] = settings.DTDCDefaultService
	data["commodity"] = settings.DTDCDefaultCommodity
	options := []string{}
	for _, mode := range settings.DTDCModes {
		if mode.Enabled {
			options = append(options, mode.Code)
		}
	}
	data["service_type_options"] = options
	data["service_modes"] = settings.DTDCModes
	data["auto_download_labels"] = settings.AutoDownloadDTDCLabels
	data["label_code"] = settings.DTDCLabelCode
	missing := []string{}
	for _, key := range h.dtdcClient.MissingConfig() {
		if key == "DTDC_SERVICE_TYPE" && settings.DTDCDefaultService != "" {
			continue
		}
		if key == "DTDC_COMMODITY" && settings.DTDCDefaultCommodity != "" {
			continue
		}
		missing = append(missing, key)
	}
	data["missing_config"] = missing
	c.JSON(200, data)
}
func (h *DirectOrderHandler) ListDTDCShipments(c *gin.Context) {
	filters := parseDirectOrderFilters(c)
	filters.DTDCBookedOnly = true
	filters.CourierStatus = c.Query("courier_status")
	orders, total, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not load DTDC shipments"})
		return
	}
	data := make([]*models.DirectOrderResponse, 0, len(orders))
	for i := range orders {
		data = append(data, models.ToDirectOrderResponse(&orders[i]))
	}
	c.JSON(200, gin.H{"data": data, "total": total, "page": filters.Page, "limit": filters.Limit, "total_pages": (total + filters.Limit - 1) / filters.Limit})
}
func (h *DirectOrderHandler) dtdcOrder(c *gin.Context, booked bool) *models.DirectOrder {
	order, err := h.repo.GetByOrderID(c.Request.Context(), c.Param("order_id"))
	if err != nil {
		status := 500
		if errors.Is(err, sql.ErrNoRows) {
			status = 404
		}
		c.JSON(status, gin.H{"error": "Order could not be loaded"})
		return nil
	}
	if booked && (!strings.EqualFold(order.CourierType.String, "dtdc") || order.AWB.String == "") {
		c.JSON(400, gin.H{"error": "Order has no DTDC AWB"})
		return nil
	}
	return order
}
func (h *DirectOrderHandler) BookDTDC(c *gin.Context) {
	var opts dtdc.BookingOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(400, gin.H{"error": "Invalid DTDC booking options"})
		return
	}
	order := h.dtdcOrder(c, false)
	if order == nil {
		return
	}
	settings, err := h.courierSettings(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(opts.ServiceType) == "" {
		opts.ServiceType = settings.DTDCDefaultService
	}
	if strings.TrimSpace(opts.Commodity) == "" {
		opts.Commodity = settings.DTDCDefaultCommodity
	}
	enabled := false
	for _, mode := range settings.DTDCModes {
		if mode.Code == opts.ServiceType && mode.Enabled {
			enabled = true
			break
		}
	}
	if !enabled {
		c.JSON(400, gin.H{"error": "Enable this DTDC service in Courier Settings before booking"})
		return
	}
	payload, err := h.dtdcClient.BuildBooking(order, opts)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	request, err := json.Marshal(payload)
	if err != nil {
		c.JSON(400, gin.H{"error": "Could not prepare the booking request"})
		return
	}
	if err = h.repo.ClaimDTDCBooking(c.Request.Context(), order.OrderID, currentActorName(c), order.UpdatedAt, request); err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	result, err := h.dtdcClient.Book(c.Request.Context(), payload)
	// Finish persistence even when the browser disconnects during the carrier request.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err != nil {
		var rejected *dtdc.RejectedError
		uncertain := !errors.As(err, &rejected)
		saveErr := h.repo.SetShipmentBookingFailure(ctx, order.OrderID, uncertain, err.Error())
		message := err.Error()
		if uncertain || saveErr != nil {
			message += ". Booking is locked for review; check DTDC using order reference " + order.OrderID + " before any retry"
		}
		c.JSON(502, gin.H{"error": message})
		return
	}
	if err = h.repo.SaveDTDCBooking(ctx, order.OrderID, result.AWB, result.Raw, currentActorName(c)); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("DTDC booked AWB %s but local save failed. Do not rebook; reconcile this AWB with order %s", result.AWB, order.OrderID)})
		return
	}
	h.dtdcResponse(c, order.OrderID, "DTDC shipment booked successfully")
}
func (h *DirectOrderHandler) dtdcResponse(c *gin.Context, id, message string) {
	order, err := h.repo.GetByOrderID(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": "Carrier operation completed but order reload failed; refresh the page"})
		return
	}
	c.JSON(200, gin.H{"message": message, "order": models.ToDirectOrderResponse(order)})
}
func (h *DirectOrderHandler) TrackDTDC(c *gin.Context) {
	order := h.dtdcOrder(c, true)
	if order == nil {
		return
	}
	result, err := h.dtdcClient.Track(c.Request.Context(), order.AWB.String)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	raw, _ := json.Marshal(result)
	status := result.Header["strStatus"]
	// DTDC scan headers can lag its cancellation API. Keep a confirmed cancellation.
	if strings.EqualFold(order.CourierStatus.String, "Cancelled") {
		status = "Cancelled"
	}
	if err = h.repo.SaveDTDCEvent(c.Request.Context(), order.OrderID, order.AWB.String, status, "tracking", raw, currentActorName(c)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.dtdcResponse(c, order.OrderID, "Tracking refreshed")
}
func (h *DirectOrderHandler) CancelDTDC(c *gin.Context) {
	order := h.dtdcOrder(c, true)
	if order == nil {
		return
	}
	if strings.EqualFold(order.CourierStatus.String, "Cancelled") {
		h.dtdcResponse(c, order.OrderID, "Shipment already cancelled")
		return
	}
	raw, err := h.dtdcClient.Cancel(c.Request.Context(), order.AWB.String)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err = h.repo.SaveDTDCEvent(ctx, order.OrderID, order.AWB.String, "Cancelled", "cancellation", raw, currentActorName(c)); err != nil {
		c.JSON(500, gin.H{"error": "DTDC confirmed cancellation but local save failed; reconcile before further actions"})
		return
	}
	h.dtdcResponse(c, order.OrderID, "DTDC shipment cancelled")
}
func (h *DirectOrderHandler) LabelDTDC(c *gin.Context) {
	order := h.dtdcOrder(c, true)
	if order == nil {
		return
	}
	raw, err := h.dtdcClient.Label(c.Request.Context(), order.AWB.String, c.DefaultQuery("label_code", "SHIP_LABEL_4X6"))
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=\"dtdc-label.pdf\"")
	c.Data(http.StatusOK, "application/pdf", raw)
}

func (h *DirectOrderHandler) ReconcileDTDC(c *gin.Context) {
	order := h.dtdcOrder(c, false)
	if order == nil {
		return
	}
	if !strings.EqualFold(order.CourierType.String, "dtdc") || order.AWB.String != "" || (order.CourierStatus.String != "booking_pending" && order.CourierStatus.String != "booking_review") {
		c.JSON(http.StatusConflict, gin.H{"error": "This order has no unresolved DTDC booking"})
		return
	}
	if order.CourierStatus.String == "booking_pending" && time.Since(order.UpdatedAt) < 2*time.Minute {
		c.JSON(http.StatusConflict, gin.H{"error": "Booking is still in progress. Wait two minutes before checking DTDC."})
		return
	}
	result, err := h.dtdcClient.TrackReference(c.Request.Context(), order.OrderID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error() + ". Booking remains locked; no new shipment was requested."})
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not save tracking response"})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err = h.repo.ReconcileDTDCBooking(ctx, order.OrderID, result.Header["strShipmentNo"], result.Header["strStatus"], raw, currentActorName(c), order.UpdatedAt); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Shipment could not be reconciled; reload the order before trying again"})
		return
	}
	h.dtdcResponse(c, order.OrderID, "Existing DTDC shipment recovered; no new booking was created")
}
