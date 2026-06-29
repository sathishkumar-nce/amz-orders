package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/delhivery"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

type DirectOrderHandler struct {
	repo            *repository.DirectOrderRepository
	delhiveryClient *delhivery.Client
}

func NewDirectOrderHandler(repo *repository.DirectOrderRepository, delhiveryClient *delhivery.Client) *DirectOrderHandler {
	return &DirectOrderHandler{repo: repo, delhiveryClient: delhiveryClient}
}

func (h *DirectOrderHandler) CreateDirectOrder(c *gin.Context) {
	var req models.CreateDirectOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Direct order create payload invalid: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.OrderStatus == "" {
		req.OrderStatus = "confirmed"
	}
	if req.PaymentStatus == "" {
		req.PaymentStatus = "pending"
	}
	if req.Priority == "" {
		req.Priority = "P4"
	}
	if req.CourierType == nil || *req.CourierType == "" {
		req.CourierType = stringPtr("manual")
	}
	if req.Country == nil || *req.Country == "" {
		req.Country = stringPtr("India")
	}
	if req.ShipmentType == nil || *req.ShipmentType == "" {
		req.ShipmentType = stringPtr("forward")
	}
	if req.ServiceType == nil || *req.ServiceType == "" {
		req.ServiceType = stringPtr("surface")
	}
	if req.PickupLocation == nil || *req.PickupLocation == "" {
		req.PickupLocation = stringPtr("Tekgien Softwares")
	}
	if req.PackageCount == nil || *req.PackageCount <= 0 {
		defaultPackageCount := 1
		req.PackageCount = &defaultPackageCount
	}
	if req.InvoiceDate == nil || *req.InvoiceDate == "" {
		today := time.Now().Format("2006-01-02")
		req.InvoiceDate = &today
	}

	if err := validateDirectOrderPayloadForCreate(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UpdatedBy = stringPtr(currentActorName(c))

	order, err := h.repo.Create(c.Request.Context(), &req)
	if err != nil {
		log.Printf("❌ Direct order create failed (order_id=%s): %v", req.OrderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, models.ToDirectOrderResponse(order))
}

func (h *DirectOrderHandler) GetDirectOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	order, err := h.repo.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ToDirectOrderResponse(order))
}

func (h *DirectOrderHandler) GetNextDirectOrderID(c *gin.Context) {
	nextID, err := h.repo.GetSuggestedNextOrderID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order_id": nextID})
}

func (h *DirectOrderHandler) UpdateDirectOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	var req models.UpdateDirectOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Direct order update payload invalid (order_id=%s): %v", orderID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existingOrder, err := h.repo.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := validateDirectOrderPayloadForUpdate(existingOrder, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UpdatedBy = stringPtr(currentActorName(c))

	order, err := h.repo.Update(c.Request.Context(), orderID, &req)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ToDirectOrderResponse(order))
}

func (h *DirectOrderHandler) DeleteDirectOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	if err := h.repo.SoftDelete(c.Request.Context(), orderID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order deleted successfully"})
}

func (h *DirectOrderHandler) ListDirectOrders(c *gin.Context) {
	filters := parseDirectOrderFilters(c)
	orders, total, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]*models.DirectOrderResponse, 0, len(orders))
	for i := range orders {
		order := orders[i]
		responses = append(responses, models.ToDirectOrderResponse(&order))
	}

	totalPages := (total + filters.Limit - 1) / filters.Limit
	c.JSON(http.StatusOK, gin.H{
		"data":        responses,
		"page":        filters.Page,
		"limit":       filters.Limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

func (h *DirectOrderHandler) SearchDirectOrders(c *gin.Context) {
	h.ListDirectOrders(c)
}

func (h *DirectOrderHandler) ExportDirectOrdersCSV(c *gin.Context) {
	filters := parseDirectOrderFilters(c)
	orders, err := h.repo.ExportToCSV(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("direct_orders_%s.csv", time.Now().Format("2006-01-02_15-04-05"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	header := []string{
		"Order ID", "Source", "Order Status", "Payment Status", "Priority",
		"Customer Name", "Mobile", "Alternate Mobile", "Email", "Alternate Email",
		"Address", "City", "State", "Country", "Pincode", "Landmark",
		"Courier Type", "Courier Name", "AWB", "Courier Status", "Pickup Location",
		"Invoice Date", "Amount", "Advance Amount", "COD Amount",
		"Package Count", "Total Weight", "Length CM", "Width CM", "Height CM",
		"Items", "Remarks", "Issues", "Created At", "Updated At",
	}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write CSV header"})
		return
	}

	for _, order := range orders {
		itemsStr := ""
		for i, item := range order.Items {
			if i > 0 {
				itemsStr += "; "
			}
			itemStr := fmt.Sprintf("Item: %s, Qty: %d", getStringValue(item.Item), item.Quantity)
			if item.SKU.Valid {
				itemStr += fmt.Sprintf(", SKU: %s", item.SKU.String)
			}
			if item.Dimension.Valid {
				itemStr += fmt.Sprintf(", Dim: %s", item.Dimension.String)
			}
			if item.Weight.Valid {
				itemStr += fmt.Sprintf(", Weight: %.3f", item.Weight.Float64)
			}
			if item.Amount.Valid {
				itemStr += fmt.Sprintf(", Amount: %.2f", item.Amount.Float64)
			}
			itemsStr += itemStr
		}

		row := []string{
			order.OrderID,
			getStringValue(order.Source),
			order.OrderStatus,
			order.PaymentStatus,
			order.Priority,
			getStringValue(order.CustomerName),
			getStringValue(order.Mobile),
			getStringValue(order.AlternateMobile),
			getStringValue(order.Email),
			getStringValue(order.AlternateEmail),
			getStringValue(order.Address),
			getStringValue(order.City),
			getStringValue(order.State),
			getStringValue(order.Country),
			getStringValue(order.Pincode),
			getStringValue(order.Landmark),
			getStringValue(order.CourierType),
			getStringValue(order.CourierName),
			getStringValue(order.AWB),
			getStringValue(order.CourierStatus),
			getStringValue(order.PickupLocation),
			getTimeValue(order.InvoiceDate),
			getFloatValue(order.Amount),
			getFloatValue(order.AdvanceAmount),
			getFloatValue(order.CODAmount),
			getIntValue(order.PackageCount),
			getFloatValue(order.TotalWeight),
			getFloatValue(order.LengthCM),
			getFloatValue(order.WidthCM),
			getFloatValue(order.HeightCM),
			itemsStr,
			getStringValue(order.Remarks),
			getStringValue(order.Issues),
			order.CreatedAt.Format(time.RFC3339),
			order.UpdatedAt.Format(time.RFC3339),
		}

		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write CSV row"})
			return
		}
	}
}

func (h *DirectOrderHandler) CreateDelhiveryForwardOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	order, err := h.repo.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.delhiveryClient == nil || !h.delhiveryClient.Enabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Delhivery integration is not configured. Set DELHIVERY_API_TOKEN in the backend environment and restart the service"})
		return
	}

	result, err := h.delhiveryClient.CreateForwardOrder(c.Request.Context(), order)
	if err != nil {
		log.Printf("❌ Delhivery forward order creation failed (order_id=%s): %v", orderID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedOrder, err := h.repo.SaveDelhiveryForwardOrder(c.Request.Context(), orderID, *result, currentActorName(c))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Delhivery forward order created successfully",
		"order":   models.ToDirectOrderResponse(updatedOrder),
	})
}

func (h *DirectOrderHandler) CreateDelhiveryForwardOrdersBulk(c *gin.Context) {
	var req models.DirectOrderBulkForwardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one order_id is required"})
		return
	}

	if h.delhiveryClient == nil || !h.delhiveryClient.Enabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Delhivery integration is not configured. Set DELHIVERY_API_TOKEN in the backend environment and restart the service"})
		return
	}

	ctx := c.Request.Context()
	actor := currentActorName(c)
	seen := make(map[string]struct{}, len(req.OrderIDs))
	successes := make([]models.DirectOrderBulkForwardItem, 0, len(req.OrderIDs))
	failures := make([]models.DirectOrderBulkForwardItem, 0)

	for _, rawOrderID := range req.OrderIDs {
		orderID := strings.TrimSpace(rawOrderID)
		if orderID == "" {
			continue
		}
		if _, exists := seen[orderID]; exists {
			continue
		}
		seen[orderID] = struct{}{}

		order, err := h.repo.GetByOrderID(ctx, orderID)
		if err != nil {
			failures = append(failures, models.DirectOrderBulkForwardItem{
				OrderID: orderID,
				Error:   resolveDirectOrderError(err),
			})
			continue
		}

		result, err := h.delhiveryClient.CreateForwardOrder(ctx, order)
		if err != nil {
			failures = append(failures, models.DirectOrderBulkForwardItem{
				OrderID: orderID,
				Error:   err.Error(),
			})
			continue
		}

		updatedOrder, err := h.repo.SaveDelhiveryForwardOrder(ctx, orderID, *result, actor)
		if err != nil {
			failures = append(failures, models.DirectOrderBulkForwardItem{
				OrderID: orderID,
				Error:   resolveDirectOrderError(err),
			})
			continue
		}

		successes = append(successes, models.DirectOrderBulkForwardItem{
			OrderID: orderID,
			Order:   models.ToDirectOrderResponse(updatedOrder),
		})
	}

	response := models.DirectOrderBulkForwardResponse{
		Message:      "Bulk Delhivery forward order creation completed",
		Requested:    len(seen),
		SuccessCount: len(successes),
		FailureCount: len(failures),
		Successes:    successes,
		Failures:     failures,
	}

	c.JSON(http.StatusOK, response)
}

func parseDirectOrderFilters(c *gin.Context) models.DirectOrderFilters {
	filters := models.DirectOrderFilters{
		OrderID:       c.Query("order_id"),
		Search:        c.Query("search"),
		AWB:           c.Query("awb"),
		OrderStatus:   c.Query("order_status"),
		PaymentStatus: c.Query("payment_status"),
		Priority:      c.Query("priority"),
		Source:        c.Query("source"),
		Mobile:        c.Query("mobile"),
		CustomerName:  c.Query("customer_name"),
		Item:          c.Query("item"),
		Pincode:       c.Query("pincode"),
		DateExact:     c.Query("date_exact"),
		DateFrom:      c.Query("date_from"),
		DateTo:        c.Query("date_to"),
	}
	if val := c.Query("quantity"); val != "" {
		if quantity, err := strconv.Atoi(val); err == nil {
			filters.Quantity = quantity
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	filters.Page = page
	filters.Limit = limit
	return filters
}

func resolveDirectOrderError(err error) string {
	if err == sql.ErrNoRows {
		return "order not found"
	}
	return err.Error()
}

var directOrderMobilePattern = regexp.MustCompile(`^\d{10}$`)
var directOrderPincodePattern = regexp.MustCompile(`^\d{6}$`)

func validateDirectOrderPayloadForCreate(req *models.CreateDirectOrderRequest) error {
	issues := validateDirectOrderFields(
		req.OrderStatus,
		req.PaymentStatus,
		req.Amount,
		req.AdvanceAmount,
		req.CODAmount,
		req.Mobile,
		req.AlternateMobile,
		req.Pincode,
		req.TotalWeight,
		req.LengthCM,
		req.WidthCM,
		req.HeightCM,
		req.Issues,
		req.Items,
	)
	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "\n"))
	}
	return nil
}

func validateDirectOrderPayloadForUpdate(existing *models.DirectOrder, req *models.UpdateDirectOrderRequest) error {
	orderStatus := existing.OrderStatus
	if req.OrderStatus != nil {
		orderStatus = strings.TrimSpace(*req.OrderStatus)
	}

	paymentStatus := existing.PaymentStatus
	if req.PaymentStatus != nil {
		paymentStatus = strings.TrimSpace(*req.PaymentStatus)
	}

	amount := nullFloatToPtr(existing.Amount)
	if req.Amount != nil {
		amount = req.Amount
	}

	advanceAmount := nullFloatToPtr(existing.AdvanceAmount)
	if req.AdvanceAmount != nil {
		advanceAmount = req.AdvanceAmount
	}

	codAmount := nullFloatToPtr(existing.CODAmount)
	if req.CODAmount != nil {
		codAmount = req.CODAmount
	}

	mobile := nullStringToPtr(existing.Mobile)
	if req.Mobile != nil {
		mobile = req.Mobile
	}

	alternateMobile := nullStringToPtr(existing.AlternateMobile)
	if req.AlternateMobile != nil {
		alternateMobile = req.AlternateMobile
	}

	pincode := nullStringToPtr(existing.Pincode)
	if req.Pincode != nil {
		pincode = req.Pincode
	}

	totalWeight := nullFloatToPtr(existing.TotalWeight)
	if req.TotalWeight != nil {
		totalWeight = req.TotalWeight
	}

	lengthCM := nullFloatToPtr(existing.LengthCM)
	if req.LengthCM != nil {
		lengthCM = req.LengthCM
	}

	widthCM := nullFloatToPtr(existing.WidthCM)
	if req.WidthCM != nil {
		widthCM = req.WidthCM
	}

	heightCM := nullFloatToPtr(existing.HeightCM)
	if req.HeightCM != nil {
		heightCM = req.HeightCM
	}

	issuesField := nullStringToPtr(existing.Issues)
	if req.Issues != nil {
		issuesField = req.Issues
	}

	items := make([]models.CreateDirectOrderItemRequest, 0, len(existing.Items))
	for _, item := range existing.Items {
		items = append(items, models.CreateDirectOrderItemRequest{
			Item:      nullStringToPtr(item.Item),
			Quantity:  item.Quantity,
			Dimension: nullStringToPtr(item.Dimension),
			Thickness: nullStringToPtr(item.Thickness),
			Weight:    nullFloatToPtr(item.Weight),
			Amount:    nullFloatToPtr(item.Amount),
			Remark:    nullStringToPtr(item.Remark),
			SKU:       nullStringToPtr(item.SKU),
			HSN:       nullStringToPtr(item.HSN),
			UnitPrice: nullFloatToPtr(item.UnitPrice),
			TaxRate:   nullFloatToPtr(item.TaxRate),
		})
	}
	if req.Items != nil {
		items = *req.Items
	}

	issues := validateDirectOrderFields(
		orderStatus,
		paymentStatus,
		amount,
		advanceAmount,
		codAmount,
		mobile,
		alternateMobile,
		pincode,
		totalWeight,
		lengthCM,
		widthCM,
		heightCM,
		issuesField,
		items,
	)
	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "\n"))
	}
	return nil
}

func validateDirectOrderFields(
	orderStatus string,
	paymentStatus string,
	amount *float64,
	advanceAmount *float64,
	codAmount *float64,
	mobile *string,
	alternateMobile *string,
	pincode *string,
	totalWeight *float64,
	lengthCM *float64,
	widthCM *float64,
	heightCM *float64,
	issues *string,
	items []models.CreateDirectOrderItemRequest,
) []string {
	var validationIssues []string

	trimmedPaymentStatus := strings.TrimSpace(strings.ToLower(paymentStatus))
	codValue := valueOrZero(codAmount)
	advanceValue := valueOrZero(advanceAmount)
	amountValue := valueOrZero(amount)

	switch trimmedPaymentStatus {
	case "paid-full":
		if codValue != 0 {
			validationIssues = append(validationIssues, "COD amount must be 0 when payment status is paid-full")
		}
	case "paid-advance":
		if codValue <= 0 {
			validationIssues = append(validationIssues, "COD amount must be greater than 0 when payment status is paid-advance")
		}
	}

	if amount != nil && *amount < 0 {
		validationIssues = append(validationIssues, "Total amount cannot be negative")
	}
	if advanceAmount != nil && *advanceAmount < 0 {
		validationIssues = append(validationIssues, "Advance amount cannot be negative")
	}
	if codAmount != nil && *codAmount < 0 {
		validationIssues = append(validationIssues, "COD amount cannot be negative")
	}
	if amount != nil && advanceValue > amountValue {
		validationIssues = append(validationIssues, "Advance amount cannot be greater than total amount")
	}
	if amount != nil && codValue > amountValue {
		validationIssues = append(validationIssues, "COD amount cannot be greater than total amount")
	}

	if mobile != nil && strings.TrimSpace(*mobile) != "" && !directOrderMobilePattern.MatchString(strings.TrimSpace(*mobile)) {
		validationIssues = append(validationIssues, "Mobile number must be exactly 10 digits")
	}
	if alternateMobile != nil && strings.TrimSpace(*alternateMobile) != "" && !directOrderMobilePattern.MatchString(strings.TrimSpace(*alternateMobile)) {
		validationIssues = append(validationIssues, "Alternate mobile number must be exactly 10 digits")
	}
	if pincode != nil && strings.TrimSpace(*pincode) != "" && !directOrderPincodePattern.MatchString(strings.TrimSpace(*pincode)) {
		validationIssues = append(validationIssues, "Pincode must be exactly 6 digits")
	}

	if totalWeight != nil && *totalWeight < 0 {
		validationIssues = append(validationIssues, "Order weight cannot be negative")
	}
	if lengthCM != nil && *lengthCM < 0 {
		validationIssues = append(validationIssues, "Length cannot be negative")
	}
	if widthCM != nil && *widthCM < 0 {
		validationIssues = append(validationIssues, "Width cannot be negative")
	}
	if heightCM != nil && *heightCM < 0 {
		validationIssues = append(validationIssues, "Height cannot be negative")
	}

	if strings.TrimSpace(orderStatus) == "other-issues" && (issues == nil || strings.TrimSpace(*issues) == "") {
		validationIssues = append(validationIssues, "Issues field is required when order status is other-issues")
	}

	for index, item := range items {
		itemNumber := index + 1
		if item.Quantity < 0 {
			validationIssues = append(validationIssues, fmt.Sprintf("Item %d quantity cannot be negative", itemNumber))
		}
		if item.Weight != nil && *item.Weight < 0 {
			validationIssues = append(validationIssues, fmt.Sprintf("Item %d weight cannot be negative", itemNumber))
		}
		if item.Amount != nil && *item.Amount < 0 {
			validationIssues = append(validationIssues, fmt.Sprintf("Item %d amount cannot be negative", itemNumber))
		}
		if item.UnitPrice != nil && *item.UnitPrice < 0 {
			validationIssues = append(validationIssues, fmt.Sprintf("Item %d unit price cannot be negative", itemNumber))
		}
		if item.TaxRate != nil && *item.TaxRate < 0 {
			validationIssues = append(validationIssues, fmt.Sprintf("Item %d tax rate cannot be negative", itemNumber))
		}
	}

	return validationIssues
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullStringToPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	return &trimmed
}

func nullFloatToPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func getStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func getFloatValue(nf sql.NullFloat64) string {
	if nf.Valid {
		return fmt.Sprintf("%.2f", nf.Float64)
	}
	return ""
}

func getIntValue(ni sql.NullInt64) string {
	if ni.Valid {
		return strconv.FormatInt(ni.Int64, 10)
	}
	return ""
}

func getTimeValue(nt sql.NullTime) string {
	if nt.Valid {
		return nt.Time.Format("2006-01-02")
	}
	return ""
}

func stringPtr(value string) *string {
	return &value
}
