package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestApplySharedListFiltersParsesDimensionOperators(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?default_width_in_inches=24.5&default_width_in_inches_operator=lte&default_length_in_inches=36&default_length_in_inches_operator=gt", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["default_width_in_inches"].(float64); !ok || got != 24.5 {
		t.Fatalf("expected default_width_in_inches=24.5, got %#v", filters["default_width_in_inches"])
	}
	if got, ok := filters["default_width_in_inches_operator"].(string); !ok || got != "lte" {
		t.Fatalf("expected default_width_in_inches_operator=lte, got %#v", filters["default_width_in_inches_operator"])
	}
	if got, ok := filters["default_length_in_inches"].(float64); !ok || got != 36 {
		t.Fatalf("expected default_length_in_inches=36, got %#v", filters["default_length_in_inches"])
	}
	if got, ok := filters["default_length_in_inches_operator"].(string); !ok || got != "gt" {
		t.Fatalf("expected default_length_in_inches_operator=gt, got %#v", filters["default_length_in_inches_operator"])
	}
}

func TestApplySharedListFiltersFallsBackToEqForInvalidOperator(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?default_width_in_inches=12&default_width_in_inches_operator=weird", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["default_width_in_inches_operator"].(string); !ok || got != "eq" {
		t.Fatalf("expected invalid operator to fall back to eq, got %#v", filters["default_width_in_inches_operator"])
	}
}

func TestApplySharedListFiltersParsesRFC3339DateRange(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?confirmed_date_from=2026-06-29T10:15:00Z&confirmed_date_to=2026-06-30T08:00:00Z", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["confirmed_date_from"].(string); !ok || got != "2026-06-29T10:15:00Z" {
		t.Fatalf("expected confirmed_date_from to be preserved as RFC3339 UTC, got %#v", filters["confirmed_date_from"])
	}
	if got, ok := filters["confirmed_date_to"].(string); !ok || got != "2026-06-30T08:00:00Z" {
		t.Fatalf("expected confirmed_date_to to be preserved as RFC3339 UTC, got %#v", filters["confirmed_date_to"])
	}
}

func TestApplySharedListFiltersParsesDimensionSearchKey(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?search_key=default_width_in_inches&search_value=18.75&search_operator=gte", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["default_width_in_inches"].(float64); !ok || got != 18.75 {
		t.Fatalf("expected width search key to parse numeric value, got %#v", filters["default_width_in_inches"])
	}
	if got, ok := filters["default_width_in_inches_operator"].(string); !ok || got != "gte" {
		t.Fatalf("expected width search key to preserve operator, got %#v", filters["default_width_in_inches_operator"])
	}
}

func TestApplySharedListFiltersParsesDirectThickness(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?thickness=%201.5mm%20", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["thickness"].(string); !ok || got != "1.5mm" {
		t.Fatalf("expected direct thickness filter to be parsed and trimmed, got %#v", filters["thickness"])
	}
}

func TestApplySharedListFiltersParsesIsRoundAlias(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?is_round=true", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["round_product"].(bool); !ok || !got {
		t.Fatalf("expected is_round alias to set round_product=true, got %#v", filters["round_product"])
	}
}

func TestApplySharedListFiltersParsesHasCustomerInputs(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/orders?has_customer_inputs=true", nil)

	filters := map[string]interface{}{}
	applySharedListFilters(ctx, filters)

	if got, ok := filters["has_customer_inputs"].(bool); !ok || !got {
		t.Fatalf("expected has_customer_inputs=true, got %#v", filters["has_customer_inputs"])
	}
}

func TestApplyReturnsDefaultFiltersDefaultsToReturnInitiated(t *testing.T) {
	filters := map[string]interface{}{}
	applyReturnsDefaultFilters(filters, false)

	if got, ok := filters["return_initiated"].(bool); !ok || !got {
		t.Fatalf("expected returns default to return_initiated=true, got %#v", filters["return_initiated"])
	}
}

func TestApplyReturnsDefaultFiltersKeepsOrderIDSearchUnrestricted(t *testing.T) {
	filters := map[string]interface{}{
		"amazon_order_id": "407-1234567-1234567",
	}
	applyReturnsDefaultFilters(filters, false)

	if _, ok := filters["return_initiated"]; ok {
		t.Fatalf("expected order ID search to skip return_initiated default, got %#v", filters["return_initiated"])
	}
}

func TestApplyReturnsDefaultFiltersPreservesExplicitReturnInitiated(t *testing.T) {
	filters := map[string]interface{}{
		"return_initiated": false,
	}
	applyReturnsDefaultFilters(filters, false)

	if got, ok := filters["return_initiated"].(bool); !ok || got {
		t.Fatalf("expected explicit return_initiated=false to be preserved, got %#v", filters["return_initiated"])
	}
}

func TestApplyReturnsDefaultFiltersKeepsSearchUnrestricted(t *testing.T) {
	filters := map[string]interface{}{
		"sku": "MRC-MR-0246",
	}
	applyReturnsDefaultFilters(filters, true)

	if _, ok := filters["return_initiated"]; ok {
		t.Fatalf("expected search to skip return_initiated default, got %#v", filters["return_initiated"])
	}
}

func TestApplySafetyClaimsDefaultFiltersDefaultsToReturnedOrders(t *testing.T) {
	filters := map[string]interface{}{}
	applySafetyClaimsDefaultFilters(filters)

	if got, ok := filters["order_status"].(string); !ok || got != "returned" {
		t.Fatalf("expected safety claims default to order_status=returned, got %#v", filters["order_status"])
	}
}

func TestApplySafetyClaimsDefaultFiltersKeepsOrderIDSearchUnrestricted(t *testing.T) {
	filters := map[string]interface{}{
		"amazon_order_id": "407-1234567-1234567",
	}
	applySafetyClaimsDefaultFilters(filters)

	if _, ok := filters["order_status"]; ok {
		t.Fatalf("expected order ID search to skip order_status default, got %#v", filters["order_status"])
	}
}

func TestApplySafetyClaimsDefaultFiltersPreservesExplicitOrderStatus(t *testing.T) {
	filters := map[string]interface{}{
		"order_status": "manufactured",
	}
	applySafetyClaimsDefaultFilters(filters)

	if got, ok := filters["order_status"].(string); !ok || got != "manufactured" {
		t.Fatalf("expected explicit order_status=manufactured to be preserved, got %#v", filters["order_status"])
	}
}
