package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/service"
)

type OrderHandler struct {
	service *service.OrderService
}

func NewOrderHandler(service *service.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func parseQueryTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}

func isDateOnly(value string) bool {
	return len(value) == len("2006-01-02")
}

func parseFilterOperator(value string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "gt", "gte", "lt", "lte", "eq":
		return strings.TrimSpace(strings.ToLower(value)), true
	default:
		return "", false
	}
}

func executiveDashboardLocation() *time.Location {
	return time.FixedZone("IST", 5*60*60+30*60)
}

func resolveExecutiveDateRange(dateRange string, fromDateRaw string, toDateRaw string) (*time.Time, *time.Time, string, error) {
	location := executiveDashboardLocation()
	now := time.Now().In(location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	parseDateOnlyInLocation := func(value string) (time.Time, error) {
		parsed, err := time.ParseInLocation("2006-01-02", value, location)
		if err != nil {
			return time.Time{}, err
		}
		return parsed, nil
	}

	selected := dateRange
	if selected == "" {
		selected = "last_30_days"
	}

	var fromDate time.Time
	var toDate time.Time

	switch selected {
	case "today":
		fromDate = todayStart
		toDate = todayStart.AddDate(0, 0, 1)
	case "yesterday":
		fromDate = todayStart.AddDate(0, 0, -1)
		toDate = todayStart
	case "last_7_days":
		fromDate = todayStart.AddDate(0, 0, -6)
		toDate = todayStart.AddDate(0, 0, 1)
	case "last_30_days":
		fromDate = todayStart.AddDate(0, 0, -29)
		toDate = todayStart.AddDate(0, 0, 1)
	case "this_month":
		fromDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		toDate = todayStart.AddDate(0, 0, 1)
	case "last_month":
		startOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		fromDate = startOfThisMonth.AddDate(0, -1, 0)
		toDate = startOfThisMonth
	case "custom_range":
		selected = "custom_range"
		if fromDateRaw == "" || toDateRaw == "" {
			return nil, nil, "", fmt.Errorf("from_date and to_date are required for custom_range")
		}
		parsedFrom, err := parseDateOnlyInLocation(fromDateRaw)
		if err != nil {
			return nil, nil, "", fmt.Errorf("invalid from_date. Use YYYY-MM-DD")
		}
		parsedTo, err := parseDateOnlyInLocation(toDateRaw)
		if err != nil {
			return nil, nil, "", fmt.Errorf("invalid to_date. Use YYYY-MM-DD")
		}
		fromDate = parsedFrom
		toDate = parsedTo.AddDate(0, 0, 1)
	default:
		return nil, nil, "", fmt.Errorf("invalid date_range")
	}

	if !toDate.After(fromDate) {
		return nil, nil, "", fmt.Errorf("to_date must be on or after from_date")
	}

	return &fromDate, &toDate, selected, nil
}

func applySharedListFilters(c *gin.Context, filters map[string]interface{}) {
	if key := c.Query("search_key"); key != "" {
		if value := c.Query("search_value"); value != "" {
			switch key {
			case "order_id":
				filters["amazon_order_id"] = value
			case "customer":
				filters["customer"] = value
			case "phone":
				filters["mobile"] = value
			case "sku":
				filters["sku"] = value
			case "thickness":
				filters["thickness"] = value
			case "priority":
				filters["priority"] = value
			case "is_round":
				if parsed, err := strconv.ParseBool(value); err == nil {
					filters["round_product"] = parsed
				}
			case "confirmed_date":
				if parsed, err := time.Parse("2006-01-02", value); err == nil {
					filters["confirmed_date_from"] = parsed.UTC().Format(time.RFC3339)
					filters["confirmed_date_to"] = parsed.AddDate(0, 0, 1).UTC().Format(time.RFC3339)
				}
			case "order_status":
				filters["order_status"] = value
			case "default_width_in_inches":
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					filters["default_width_in_inches"] = parsed
					if operator, ok := parseFilterOperator(c.Query("search_operator")); ok {
						filters["default_width_in_inches_operator"] = operator
					} else {
						filters["default_width_in_inches_operator"] = "eq"
					}
				}
			case "default_length_in_inches":
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					filters["default_length_in_inches"] = parsed
					if operator, ok := parseFilterOperator(c.Query("search_operator")); ok {
						filters["default_length_in_inches_operator"] = operator
					} else {
						filters["default_length_in_inches_operator"] = "eq"
					}
				}
			}
		}
	}

	if val := c.Query("order_id"); val != "" {
		filters["amazon_order_id"] = val
	}
	if val := c.Query("customer"); val != "" {
		filters["customer"] = val
	}
	if val := c.Query("mobile"); val != "" {
		filters["mobile"] = val
	} else if val := c.Query("phone"); val != "" {
		filters["mobile"] = val
	}
	if val := c.Query("sku"); val != "" {
		filters["sku"] = val
	}
	if val := c.Query("thickness"); val != "" {
		filters["thickness"] = strings.TrimSpace(val)
	}
	if val := c.Query("quantity"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			filters["quantity"] = parsed
			if operator, ok := parseFilterOperator(c.Query("quantity_operator")); ok {
				filters["quantity_operator"] = operator
			} else {
				filters["quantity_operator"] = "eq"
			}
		}
	}
	if val := c.Query("default_width_in_inches"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			filters["default_width_in_inches"] = parsed
			if operator, ok := parseFilterOperator(c.Query("default_width_in_inches_operator")); ok {
				filters["default_width_in_inches_operator"] = operator
			} else {
				filters["default_width_in_inches_operator"] = "eq"
			}
		}
	}
	if val := c.Query("default_length_in_inches"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			filters["default_length_in_inches"] = parsed
			if operator, ok := parseFilterOperator(c.Query("default_length_in_inches_operator")); ok {
				filters["default_length_in_inches_operator"] = operator
			} else {
				filters["default_length_in_inches_operator"] = "eq"
			}
		}
	}
	if val := c.Query("confirmed_date"); val != "" {
		if parsed, err := time.Parse("2006-01-02", val); err == nil {
			filters["confirmed_date_from"] = parsed.UTC().Format(time.RFC3339)
			filters["confirmed_date_to"] = parsed.AddDate(0, 0, 1).UTC().Format(time.RFC3339)
		}
	}
	if val := c.Query("confirmed_date_from"); val != "" {
		if parsed, err := parseQueryTime(val); err == nil {
			filters["confirmed_date_from"] = parsed.UTC().Format(time.RFC3339)
		}
	}
	if val := c.Query("confirmed_date_to"); val != "" {
		if parsed, err := parseQueryTime(val); err == nil {
			if isDateOnly(val) {
				parsed = parsed.AddDate(0, 0, 1)
			}
			filters["confirmed_date_to"] = parsed.UTC().Format(time.RFC3339)
		}
	}
	if val := c.Query("priority"); val != "" {
		filters["priority"] = val
	}
	if val := c.Query("order_status"); val != "" {
		filters["order_status"] = val
	}
	if val := c.Query("delivery_state"); val != "" {
		filters["delivery_state"] = val
	}
	if val := c.Query("date_from"); val != "" {
		if parsed, err := parseQueryTime(val); err == nil {
			filters["date_from"] = parsed.UTC().Format(time.RFC3339)
		}
	}
	if val := c.Query("date_to"); val != "" {
		if parsed, err := parseQueryTime(val); err == nil {
			if isDateOnly(val) {
				parsed = parsed.AddDate(0, 0, 1)
			}
			filters["date_to"] = parsed.UTC().Format(time.RFC3339)
		}
	}
	if val := c.Query("search"); val != "" {
		filters["search"] = val
	}
	if val := c.Query("round_product"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["round_product"] = parsed
		}
	}
	if val := c.Query("is_round"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["round_product"] = parsed
		}
	}
	if val := c.Query("missing_customer_inputs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["missing_customer_inputs"] = parsed
		}
	}
	if val := c.Query("has_customer_inputs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["has_customer_inputs"] = parsed
		}
	}
	if val := c.Query("return_initiated"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["return_initiated"] = parsed
		}
	}
	if val := c.Query("return_initiated_compromised"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["return_initiated_compromised"] = parsed
		}
	}
	if val := c.Query("other_issues"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["other_issues"] = parsed
		}
	}
	if val := c.Query("safety_claimed"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			filters["safety_claimed"] = parsed
		}
	}
}

// Health returns service health status
func (h *OrderHandler) Health(c *gin.Context) {
	log.Printf("🩺 Health check requested")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ImportFromBaseLinker fetches and imports orders from BaseLinker
func (h *OrderHandler) ImportFromBaseLinker(c *gin.Context) {
	var req struct {
		DateConfirmedFrom int64 `json:"date_confirmed_from"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body, default to 0
		req.DateConfirmedFrom = 0
	}
	log.Printf("📨 API import request received (date_confirmed_from=%d)", req.DateConfirmedFrom)

	totalFetched, totalOrders, totalProducts, err := h.service.ImportFromBaseLinker(c.Request.Context(), req.DateConfirmedFrom)
	if err != nil {
		log.Printf("❌ API import request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ API import request completed: fetched=%d orders=%d products=%d", totalFetched, totalOrders, totalProducts)

	c.JSON(http.StatusOK, gin.H{
		"status":                  "success",
		"total_fetched":           totalFetched,
		"total_orders_upserted":   totalOrders,
		"total_products_upserted": totalProducts,
	})
}

// ImportFromSampleFile imports orders from local JSON file
func (h *OrderHandler) ImportFromSampleFile(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Sample import request rejected: missing file_path")
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_path is required"})
		return
	}
	log.Printf("📨 Sample import request received (file_path=%s)", req.FilePath)

	totalFetched, totalOrders, totalProducts, err := h.service.ImportFromFile(c.Request.Context(), req.FilePath)
	if err != nil {
		log.Printf("❌ Sample import failed for %s: %v", req.FilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ Sample import completed for %s: fetched=%d orders=%d products=%d", req.FilePath, totalFetched, totalOrders, totalProducts)

	c.JSON(http.StatusOK, gin.H{
		"status":                  "success",
		"total_fetched":           totalFetched,
		"total_orders_upserted":   totalOrders,
		"total_products_upserted": totalProducts,
	})
}

// ListOrders returns paginated list of orders
func (h *OrderHandler) ListOrders(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}

	filters := make(map[string]interface{})

	if val := c.Query("amazon_order_id"); val != "" {
		filters["amazon_order_id"] = val
	}
	if val := c.Query("baselinker_order_id"); val != "" {
		if id, err := strconv.ParseInt(val, 10, 64); err == nil {
			filters["baselinker_order_id"] = id
		}
	}
	if val := c.Query("order_status_id"); val != "" {
		if id, err := strconv.ParseInt(val, 10, 64); err == nil {
			filters["order_status_id"] = id
		}
	}
	if val := c.Query("confirmed"); val != "" {
		if confirmed, err := strconv.ParseBool(val); err == nil {
			filters["confirmed"] = confirmed
		}
	}
	if val := c.Query("main_sku"); val != "" {
		filters["sku"] = val
	}
	if val := c.Query("delivery_city"); val != "" {
		filters["delivery_city"] = val
	}
	if val := c.Query("delivery_state"); val != "" {
		filters["delivery_state"] = val
	}
	if val := c.Query("return_status"); val != "" {
		filters["return_status"] = val
	}
	if val := c.Query("issue_status"); val != "" {
		filters["issue_status"] = val
	}
	if val := c.Query("safety_claim"); val != "" {
		filters["safety_claim"] = val
	}
	applySharedListFilters(c, filters)
	log.Printf("📋 List orders requested (page=%d limit=%d filters=%d)", page, limit, len(filters))

	orders, total, err := h.service.ListOrders(c.Request.Context(), filters, page, limit)
	if err != nil {
		log.Printf("❌ List orders failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ List orders completed: returned=%d total=%d", len(orders), total)

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data":        orders,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetOrder returns a single order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	amazonOrderID := c.Param("amazon_order_id")
	log.Printf("🔎 Get order requested (amazon_order_id=%s)", amazonOrderID)

	order, err := h.service.GetOrderByID(c.Request.Context(), amazonOrderID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("ℹ️  Order not found (amazon_order_id=%s)", amazonOrderID)
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		log.Printf("❌ Get order failed (amazon_order_id=%s): %v", amazonOrderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ Get order completed (amazon_order_id=%s)", amazonOrderID)

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) GetOrdersByIDs(c *gin.Context) {
	var req models.GetOrdersByIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.GetOrdersByIDs(c.Request.Context(), req.AmazonOrderIDs)
	if err != nil {
		log.Printf("❌ Get orders by ids failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) GetChangedOrdersByIDs(c *gin.Context) {
	var req models.GetChangedOrdersByIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	since, err := parseQueryTime(req.Since)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since. Use YYYY-MM-DD or RFC3339"})
		return
	}

	response, err := h.service.GetChangedOrdersByIDs(c.Request.Context(), req.AmazonOrderIDs, since)
	if err != nil {
		log.Printf("❌ Get changed orders by ids failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) ListReviewFollowupSettings(c *gin.Context) {
	settings, err := h.service.ListReviewFollowupSettings(c.Request.Context())
	if err != nil {
		log.Printf("❌ List review followup settings failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ReviewFollowupSettingsResponse{Settings: settings})
}

func (h *OrderHandler) UpdateReviewFollowupSettings(c *gin.Context) {
	var req models.UpdateReviewFollowupSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateReviewFollowupSettings(c.Request.Context(), req.Settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := h.service.ListReviewFollowupSettings(c.Request.Context())
	if err != nil {
		log.Printf("❌ Reload review followup settings failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ReviewFollowupSettingsResponse{Settings: settings})
}

func (h *OrderHandler) ResetReviewFollowupSettings(c *gin.Context) {
	if err := h.service.ResetReviewFollowupSettings(c.Request.Context()); err != nil {
		log.Printf("❌ Reset review followup settings failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	settings, err := h.service.ListReviewFollowupSettings(c.Request.Context())
	if err != nil {
		log.Printf("❌ Reload review followup settings failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ReviewFollowupSettingsResponse{Settings: settings})
}

func (h *OrderHandler) ListReviewQueue(c *gin.Context) {
	filters := models.ReviewQueueFilters{
		Thickness:   strings.TrimSpace(c.Query("thickness")),
		SearchKey:   strings.TrimSpace(c.Query("search_key")),
		SearchValue: strings.TrimSpace(c.Query("search_value")),
	}

	for _, rawState := range c.QueryArray("state") {
		for _, part := range strings.Split(rawState, ",") {
			if state := strings.TrimSpace(part); state != "" {
				filters.States = append(filters.States, state)
			}
		}
	}

	if operator, ok := parseFilterOperator(c.DefaultQuery("quantity_operator", "gte")); ok {
		filters.QuantityOperator = operator
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quantity_operator"})
		return
	}
	if raw := strings.TrimSpace(c.Query("quantity")); raw != "" {
		quantity, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quantity"})
			return
		}
		filters.Quantity = &quantity
	}

	if operator, ok := parseFilterOperator(c.DefaultQuery("confidence_operator", "gte")); ok {
		filters.ConfidenceOperator = operator
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confidence_operator"})
		return
	}
	if raw := strings.TrimSpace(c.Query("confidence")); raw != "" {
		confidence, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confidence"})
			return
		}
		if confidence < 0 || confidence > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "confidence must be between 0 and 100"})
			return
		}
		filters.Confidence = &confidence
	}

	if raw := strings.TrimSpace(c.Query("is_round")); raw != "" {
		isRound, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid is_round"})
			return
		}
		filters.IsRound = &isRound
	}

	if raw := strings.TrimSpace(c.Query("special")); raw != "" {
		special, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid special"})
			return
		}
		filters.SpecialOnly = special
	}

	for _, rawStatus := range c.QueryArray("status") {
		for _, part := range strings.Split(rawStatus, ",") {
			if status := strings.TrimSpace(part); status != "" {
				switch status {
				case "not-requested", "requested", "received-review":
				default:
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
					return
				}
				filters.ReviewRequestStatuses = append(filters.ReviewRequestStatuses, status)
			}
		}
	}

	response, err := h.service.ListReviewQueue(c.Request.Context(), filters)
	if err != nil {
		log.Printf("❌ List review queue failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) UpdateReviewRequestStatus(c *gin.Context) {
	var req models.UpdateReviewRequestStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedCount, err := h.service.UpdateReviewRequestStatus(c.Request.Context(), req.AmazonOrderIDs, req.Status, currentActorName(c))
	if err != nil {
		log.Printf("❌ Update review request status failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.UpdateReviewRequestStatusResponse{UpdatedCount: updatedCount})
}

func (h *OrderHandler) GetDashboardAnalytics(c *gin.Context) {
	log.Printf("📊 Dashboard analytics requested")

	chartWindowDays, err := strconv.Atoi(c.DefaultQuery("chart_days", "30"))
	if err != nil || chartWindowDays <= 0 {
		chartWindowDays = 30
	}
	if chartWindowDays > 365 {
		chartWindowDays = 365
	}

	missingRiskWindowDays, err := strconv.Atoi(c.DefaultQuery("missing_risk_days", "7"))
	if err != nil || missingRiskWindowDays <= 0 {
		missingRiskWindowDays = 7
	}
	if missingRiskWindowDays > 60 {
		missingRiskWindowDays = 60
	}

	var dateFromPtr *time.Time
	var dateToPtr *time.Time
	if raw := c.Query("date_from"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_from. Use YYYY-MM-DD or RFC3339"})
			return
		}
		dateFromPtr = &parsed
	}
	if raw := c.Query("date_to"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_to. Use YYYY-MM-DD or RFC3339"})
			return
		}
		dateToPtr = &parsed
	}
	if dateFromPtr != nil && dateToPtr != nil {
		if dateToPtr.Before(*dateFromPtr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_to must be on or after date_from"})
			return
		}
		if dateToPtr.Sub(*dateFromPtr).Hours()/24 > 365 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date range cannot exceed 365 days"})
			return
		}
	}

	analytics, err := h.service.GetDashboardAnalytics(c.Request.Context(), chartWindowDays, missingRiskWindowDays, dateFromPtr, dateToPtr)
	if err != nil {
		log.Printf("❌ Dashboard analytics failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Dashboard analytics completed")
	c.JSON(http.StatusOK, analytics)
}

func (h *OrderHandler) GetExecutiveDashboard(c *gin.Context) {
	log.Printf("📊 Executive dashboard requested")

	fromDate, toDate, dateRange, err := resolveExecutiveDateRange(
		c.DefaultQuery("date_range", "last_30_days"),
		c.Query("from_date"),
		c.Query("to_date"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filters := models.ExecutiveDashboardFilters{
		DateRange:   dateRange,
		FromDate:    fromDate,
		ToDate:      toDate,
		State:       strings.TrimSpace(c.Query("state")),
		City:        strings.TrimSpace(c.Query("city")),
		Thickness:   strings.TrimSpace(c.Query("thickness")),
		OrderStatus: strings.TrimSpace(c.Query("order_status")),
	}

	response, err := h.service.GetExecutiveDashboard(c.Request.Context(), filters)
	if err != nil {
		log.Printf("❌ Executive dashboard failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Executive dashboard completed")
	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) GetReturnsDashboard(c *gin.Context) {
	log.Printf("📊 Return analytics dashboard requested")

	fromDate, toDate, dateRange, err := resolveExecutiveDateRange(
		c.DefaultQuery("date_range", "last_30_days"),
		c.Query("from_date"),
		c.Query("to_date"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var returnInitiated *bool
	if raw := strings.TrimSpace(c.Query("return_initiated")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return_initiated. Use true or false"})
			return
		}
		returnInitiated = &parsed
	}

	var returnInitiatedCompromised *bool
	if raw := strings.TrimSpace(c.Query("return_initiated_compromised")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return_initiated_compromised. Use true or false"})
			return
		}
		returnInitiatedCompromised = &parsed
	}

	filters := models.ReturnsDashboardFilters{
		DateRange:                  dateRange,
		FromDate:                   fromDate,
		ToDate:                     toDate,
		State:                      strings.TrimSpace(c.Query("state")),
		City:                       strings.TrimSpace(c.Query("city")),
		Thickness:                  strings.TrimSpace(c.Query("thickness")),
		OrderStatus:                strings.TrimSpace(c.Query("order_status")),
		ReturnInitiated:            returnInitiated,
		ReturnInitiatedCompromised: returnInitiatedCompromised,
	}

	response, err := h.service.GetReturnsDashboard(c.Request.Context(), filters)
	if err != nil {
		log.Printf("❌ Return analytics dashboard failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Return analytics dashboard completed")
	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) GetSafetyClaimsDashboard(c *gin.Context) {
	log.Printf("📊 Safety claims analytics dashboard requested")

	fromDate, toDate, dateRange, err := resolveExecutiveDateRange(
		c.DefaultQuery("date_range", "last_30_days"),
		c.Query("from_date"),
		c.Query("to_date"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var safetyClaimed *bool
	if raw := strings.TrimSpace(c.Query("safety_claimed")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid safety_claimed. Use true or false"})
			return
		}
		safetyClaimed = &parsed
	}

	filters := models.SafetyClaimsDashboardFilters{
		DateRange:     dateRange,
		FromDate:      fromDate,
		ToDate:        toDate,
		State:         strings.TrimSpace(c.Query("state")),
		City:          strings.TrimSpace(c.Query("city")),
		Thickness:     strings.TrimSpace(c.Query("thickness")),
		OrderStatus:   strings.TrimSpace(c.Query("order_status")),
		SafetyClaimed: safetyClaimed,
	}

	response, err := h.service.GetSafetyClaimsDashboard(c.Request.Context(), filters)
	if err != nil {
		log.Printf("❌ Safety claims analytics dashboard failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Safety claims analytics dashboard completed")
	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) GetRepeatOrderCustomers(c *gin.Context) {
	log.Printf("📊 Repeat customer orders analytics requested")

	var confirmedDateFromPtr *time.Time
	var confirmedDateToPtr *time.Time
	if raw := c.Query("confirmed_date_from"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confirmed_date_from. Use YYYY-MM-DD or RFC3339"})
			return
		}
		confirmedDateFromPtr = &parsed
	}
	if raw := c.Query("confirmed_date_to"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confirmed_date_to. Use YYYY-MM-DD or RFC3339"})
			return
		}
		confirmedDateToPtr = &parsed
	}

	response, err := h.service.GetRepeatCustomers(c.Request.Context(), false, confirmedDateFromPtr, confirmedDateToPtr)
	if err != nil {
		log.Printf("❌ Repeat customer orders analytics failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Repeat customer orders analytics completed")
	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) GetRepeatReturnCustomers(c *gin.Context) {
	log.Printf("📊 Repeat customer returns analytics requested")

	var confirmedDateFromPtr *time.Time
	var confirmedDateToPtr *time.Time
	if raw := c.Query("confirmed_date_from"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confirmed_date_from. Use YYYY-MM-DD or RFC3339"})
			return
		}
		confirmedDateFromPtr = &parsed
	}
	if raw := c.Query("confirmed_date_to"); raw != "" {
		parsed, err := parseQueryTime(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confirmed_date_to. Use YYYY-MM-DD or RFC3339"})
			return
		}
		confirmedDateToPtr = &parsed
	}

	response, err := h.service.GetRepeatCustomers(c.Request.Context(), true, confirmedDateFromPtr, confirmedDateToPtr)
	if err != nil {
		log.Printf("❌ Repeat customer returns analytics failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Repeat customer returns analytics completed")
	c.JSON(http.StatusOK, response)
}

// UpdateManualFields updates manual business fields
func (h *OrderHandler) UpdateManualFields(c *gin.Context) {
	amazonOrderID := c.Param("amazon_order_id")
	log.Printf("🛠️  Manual update requested (amazon_order_id=%s)", amazonOrderID)

	var req models.UpdateManualFieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Manual update payload invalid (amazon_order_id=%s): %v", amazonOrderID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.service.UpdateManualFields(c.Request.Context(), amazonOrderID, &req, currentActorName(c))
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("ℹ️  Manual update target not found (amazon_order_id=%s)", amazonOrderID)
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		log.Printf("❌ Manual update failed (amazon_order_id=%s): %v", amazonOrderID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ Manual update completed (amazon_order_id=%s)", amazonOrderID)

	c.JSON(http.StatusOK, order)
}

// UpdateProductManualFields updates manual fields for one product inside an order.
func (h *OrderHandler) UpdateProductManualFields(c *gin.Context) {
	amazonOrderID := c.Param("amazon_order_id")
	orderProductID, err := strconv.ParseInt(c.Param("order_product_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_product_id"})
		return
	}
	log.Printf("🛠️  Product manual update requested (amazon_order_id=%s order_product_id=%d)", amazonOrderID, orderProductID)

	var req models.UpdateProductManualFieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Product manual update payload invalid (amazon_order_id=%s order_product_id=%d): %v", amazonOrderID, orderProductID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.service.UpdateProductManualFields(c.Request.Context(), amazonOrderID, orderProductID, &req, currentActorName(c))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// ListIssues returns orders with issues
func (h *OrderHandler) ListIssues(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}

	filters := map[string]interface{}{}
	applySharedListFilters(c, filters)
	if _, hasExplicitOtherIssues := filters["other_issues"]; !hasExplicitOtherIssues {
		filters["other_issues_exclude"] = false
	}
	log.Printf("📋 List issues requested (page=%d limit=%d filters=%d)", page, limit, len(filters))

	orders, total, err := h.service.ListOrders(c.Request.Context(), filters, page, limit)
	if err != nil {
		log.Printf("❌ List issues failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ List issues completed: returned=%d total=%d", len(orders), total)

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data":        orders,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// ListReturns returns orders with returns
func (h *OrderHandler) ListReturns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}

	filters := map[string]interface{}{}
	applySharedListFilters(c, filters)
	if _, hasExplicitReturnInitiated := filters["return_initiated"]; !hasExplicitReturnInitiated {
		filters["return_initiated_exclude"] = false
	}
	log.Printf("📋 List returns requested (page=%d limit=%d filters=%d)", page, limit, len(filters))

	orders, total, err := h.service.ListOrders(c.Request.Context(), filters, page, limit)
	if err != nil {
		log.Printf("❌ List returns failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ List returns completed: returned=%d total=%d", len(orders), total)

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data":        orders,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// ListSafetyClaims returns orders with safety claims
func (h *OrderHandler) ListSafetyClaims(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}

	filters := map[string]interface{}{}
	applySharedListFilters(c, filters)
	if _, hasExplicitSafetyClaimed := filters["safety_claimed"]; !hasExplicitSafetyClaimed {
		filters["safety_claimed_exclude"] = false
	}
	log.Printf("📋 List safety claims requested (page=%d limit=%d filters=%d)", page, limit, len(filters))

	orders, total, err := h.service.ListOrders(c.Request.Context(), filters, page, limit)
	if err != nil {
		log.Printf("❌ List safety claims failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ List safety claims completed: returned=%d total=%d", len(orders), total)

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data":        orders,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}
