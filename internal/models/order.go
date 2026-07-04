package models

import (
	"database/sql"
	"time"
)

// AmazonOrder represents the amazon_orders table
type AmazonOrder struct {
	AmazonOrderID          string          `json:"amazon_order_id"`
	BaseLinkerOrderID      int64           `json:"baselinker_order_id"`
	ShopOrderID            int64           `json:"shop_order_id"`
	OrderSource            sql.NullString  `json:"order_source,omitempty"`
	OrderSourceID          sql.NullInt64   `json:"order_source_id,omitempty"`
	OrderSourceInfo        sql.NullString  `json:"order_source_info,omitempty"`
	OrderStatusID          int64           `json:"order_status_id"`
	Confirmed              bool            `json:"confirmed"`
	DateConfirmed          sql.NullTime    `json:"date_confirmed,omitempty"`
	DateAdd                sql.NullTime    `json:"date_add,omitempty"`
	DateInStatus           sql.NullTime    `json:"date_in_status,omitempty"`
	UserLogin              sql.NullString  `json:"user_login,omitempty"`
	Phone                  sql.NullString  `json:"phone,omitempty"`
	Email                  sql.NullString  `json:"email,omitempty"`
	UserComments           sql.NullString  `json:"user_comments,omitempty"`
	AdminComments          sql.NullString  `json:"admin_comments,omitempty"`
	Currency               sql.NullString  `json:"currency,omitempty"`
	PaymentMethod          sql.NullString  `json:"payment_method,omitempty"`
	PaymentMethodCOD       sql.NullString  `json:"payment_method_cod,omitempty"`
	PaymentDone            float64         `json:"payment_done"`
	DeliveryMethodID       sql.NullInt64   `json:"delivery_method_id,omitempty"`
	DeliveryMethod         sql.NullString  `json:"delivery_method,omitempty"`
	DeliveryPrice          float64         `json:"delivery_price"`
	DeliveryPackageModule  sql.NullString  `json:"delivery_package_module,omitempty"`
	DeliveryPackageNr      sql.NullString  `json:"delivery_package_nr,omitempty"`
	DeliveryFullname       sql.NullString  `json:"delivery_fullname,omitempty"`
	DeliveryCompany        sql.NullString  `json:"delivery_company,omitempty"`
	DeliveryAddress        sql.NullString  `json:"delivery_address,omitempty"`
	DeliveryCity           sql.NullString  `json:"delivery_city,omitempty"`
	DeliveryState          sql.NullString  `json:"delivery_state,omitempty"`
	DeliveryPostcode       sql.NullString  `json:"delivery_postcode,omitempty"`
	DeliveryCountryCode    sql.NullString  `json:"delivery_country_code,omitempty"`
	DeliveryCountry        sql.NullString  `json:"delivery_country,omitempty"`
	DeliveryPointID        sql.NullString  `json:"delivery_point_id,omitempty"`
	DeliveryPointName      sql.NullString  `json:"delivery_point_name,omitempty"`
	DeliveryPointAddress   sql.NullString  `json:"delivery_point_address,omitempty"`
	DeliveryPointPostcode  sql.NullString  `json:"delivery_point_postcode,omitempty"`
	DeliveryPointCity      sql.NullString  `json:"delivery_point_city,omitempty"`
	InvoiceFullname        sql.NullString  `json:"invoice_fullname,omitempty"`
	InvoiceCompany         sql.NullString  `json:"invoice_company,omitempty"`
	InvoiceNip             sql.NullString  `json:"invoice_nip,omitempty"`
	InvoiceAddress         sql.NullString  `json:"invoice_address,omitempty"`
	InvoiceCity            sql.NullString  `json:"invoice_city,omitempty"`
	InvoiceState           sql.NullString  `json:"invoice_state,omitempty"`
	InvoicePostcode        sql.NullString  `json:"invoice_postcode,omitempty"`
	InvoiceCountryCode     sql.NullString  `json:"invoice_country_code,omitempty"`
	InvoiceCountry         sql.NullString  `json:"invoice_country,omitempty"`
	WantInvoice            sql.NullString  `json:"want_invoice,omitempty"`
	ExtraField1            sql.NullString  `json:"extra_field_1,omitempty"`
	ExtraField2            sql.NullString  `json:"extra_field_2,omitempty"`
	OrderPage              sql.NullString  `json:"order_page,omitempty"`
	PickState              int             `json:"pick_state"`
	PackState              int             `json:"pack_state"`
	Star                   int             `json:"star"`
	CRMClientID            int64           `json:"crm_client_id"`
	MainOrderProductID     sql.NullInt64   `json:"main_order_product_id,omitempty"`
	MainProductName        sql.NullString  `json:"main_product_name,omitempty"`
	MainSKU                sql.NullString  `json:"main_sku,omitempty"`
	MainASIN               sql.NullString  `json:"main_asin,omitempty"`
	MainPriceBrutto        sql.NullFloat64 `json:"main_price_brutto,omitempty"`
	MainTaxRate            sql.NullFloat64 `json:"main_tax_rate,omitempty"`
	MainQuantity           sql.NullFloat64 `json:"main_quantity,omitempty"`
	DefaultWidthInInches   sql.NullFloat64 `json:"default_width_in_inches,omitempty"`
	DefaultLengthInInches  sql.NullFloat64 `json:"default_length_in_inches,omitempty"`
	CustomerWidthInInches  sql.NullFloat64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInInches sql.NullFloat64 `json:"customer_length_in_inches,omitempty"`
	DefaultWidthInMM       sql.NullFloat64 `json:"default_width_in_mm,omitempty"`
	DefaultLengthInMM      sql.NullFloat64 `json:"default_length_in_mm,omitempty"`
	CustomerWidthInMM      sql.NullFloat64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthInMM     sql.NullFloat64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes   sql.NullString  `json:"corner_radius_and_notes,omitempty"`
	InternalNotes          sql.NullString  `json:"internal_notes,omitempty"`
	Priority               string          `json:"priority"`
	OrderStatus            string          `json:"order_status"`
	OrderStatusUpdatedAt   sql.NullTime    `json:"order_status_updated_at,omitempty"`
	IsRound                bool            `json:"is_round"`
	UpdatedBy              sql.NullString  `json:"updated_by,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	Products               []OrderProduct  `json:"products,omitempty"`
}

// OrderProduct represents the amazon_order_products table
type OrderProduct struct {
	OrderProductID                      int64           `json:"order_product_id"`
	AmazonOrderID                       string          `json:"amazon_order_id"`
	Storage                             sql.NullString  `json:"storage,omitempty"`
	StorageID                           sql.NullInt64   `json:"storage_id,omitempty"`
	ProductID                           sql.NullString  `json:"product_id,omitempty"`
	VariantID                           sql.NullString  `json:"variant_id,omitempty"`
	Name                                sql.NullString  `json:"name,omitempty"`
	Attributes                          sql.NullString  `json:"attributes,omitempty"`
	SKU                                 sql.NullString  `json:"sku,omitempty"`
	EAN                                 sql.NullString  `json:"ean,omitempty"`
	Location                            sql.NullString  `json:"location,omitempty"`
	WarehouseID                         sql.NullInt64   `json:"warehouse_id,omitempty"`
	AuctionID                           sql.NullString  `json:"auction_id,omitempty"`
	PriceBrutto                         sql.NullFloat64 `json:"price_brutto,omitempty"`
	TaxRate                             sql.NullFloat64 `json:"tax_rate,omitempty"`
	Quantity                            sql.NullFloat64 `json:"quantity,omitempty"`
	DefaultWidthInInches                sql.NullFloat64 `json:"default_width_in_inches,omitempty"`
	DefaultLengthInInches               sql.NullFloat64 `json:"default_length_in_inches,omitempty"`
	CustomerWidthInInches               sql.NullFloat64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInInches              sql.NullFloat64 `json:"customer_length_in_inches,omitempty"`
	DefaultWidthInMM                    sql.NullFloat64 `json:"default_width_in_mm,omitempty"`
	DefaultLengthInMM                   sql.NullFloat64 `json:"default_length_in_mm,omitempty"`
	CustomerWidthInMM                   sql.NullFloat64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthInMM                  sql.NullFloat64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes                sql.NullString  `json:"corner_radius_and_notes,omitempty"`
	SafetyClaimed                       sql.NullBool    `json:"safety_claimed,omitempty"`
	SafetyClaimedUpdatedAt              sql.NullTime    `json:"safety_claimed_updated_at,omitempty"`
	SafetyClaimIssues                   sql.NullString  `json:"safety_claim_issues,omitempty"`
	ReturnInitiated                     sql.NullBool    `json:"return_initiated,omitempty"`
	ReturnInitiatedUpdatedAt            sql.NullTime    `json:"return_initiated_updated_at,omitempty"`
	ReturnInitiatedReason               sql.NullString  `json:"return_initiated_reason,omitempty"`
	ReturnInitiatedFollowupAction       sql.NullString  `json:"return_initiated_followup_action,omitempty"`
	ReturnInitiatedCompromised          sql.NullBool    `json:"return_initiated_compromised,omitempty"`
	ReturnInitiatedCompromisedReason    sql.NullString  `json:"return_initiated_compromised_reason,omitempty"`
	ReturnInitiatedCompromisedUpdatedAt sql.NullTime    `json:"return_initiated_compromised_updated_at,omitempty"`
	OtherIssues                         sql.NullBool    `json:"other_issues,omitempty"`
	OtherIssuesReason                   sql.NullString  `json:"other_issues_reason,omitempty"`
	OtherIssueUpdatedAt                 sql.NullTime    `json:"other_issue_updated_at,omitempty"`
	IsRound                             bool            `json:"is_round"`
	Thickness                           sql.NullString  `json:"thickness,omitempty"`
	Weight                              sql.NullFloat64 `json:"weight,omitempty"`
	BundleID                            sql.NullInt64   `json:"bundle_id,omitempty"`
	IsDiscountLine                      bool            `json:"is_discount_line"`
	UpdatedBy                           sql.NullString  `json:"updated_by,omitempty"`
	CreatedAt                           time.Time       `json:"created_at"`
	UpdatedAt                           time.Time       `json:"updated_at"`
}

// UpdateManualFieldsRequest represents manual field updates
type UpdateManualFieldsRequest struct {
	DefaultWidthInInches   *float64 `json:"default_width_in_inches,omitempty"`
	DefaultLengthInInches  *float64 `json:"default_length_in_inches,omitempty"`
	CustomerWidthInInches  *float64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInInches *float64 `json:"customer_length_in_inches,omitempty"`
	DefaultWidthInMM       *float64 `json:"default_width_in_mm,omitempty"`
	DefaultLengthInMM      *float64 `json:"default_length_in_mm,omitempty"`
	CustomerWidthInMM      *float64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthInMM     *float64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes   *string  `json:"corner_radius_and_notes,omitempty"`
	InternalNotes          *string  `json:"internal_notes,omitempty"`
	Priority               *string  `json:"priority,omitempty"`
	OrderStatus            *string  `json:"order_status,omitempty"`
	IsRound                *bool    `json:"is_round,omitempty"`
}

// UpdateProductManualFieldsRequest represents manual product-level field updates
type UpdateProductManualFieldsRequest struct {
	DefaultWidthInInches             *float64 `json:"default_width_in_inches,omitempty"`
	DefaultLengthInInches            *float64 `json:"default_length_in_inches,omitempty"`
	CustomerWidthInInches            *float64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInInches           *float64 `json:"customer_length_in_inches,omitempty"`
	DefaultWidthInMM                 *float64 `json:"default_width_in_mm,omitempty"`
	DefaultLengthInMM                *float64 `json:"default_length_in_mm,omitempty"`
	CustomerWidthInMM                *float64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthInMM               *float64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes             *string  `json:"corner_radius_and_notes,omitempty"`
	SafetyClaimed                    *bool    `json:"safety_claimed,omitempty"`
	SafetyClaimIssues                *string  `json:"safety_claim_issues,omitempty"`
	ReturnInitiated                  *bool    `json:"return_initiated,omitempty"`
	ReturnInitiatedReason            *string  `json:"return_initiated_reason,omitempty"`
	ReturnInitiatedFollowupAction    *string  `json:"return_initiated_followup_action,omitempty"`
	ReturnInitiatedCompromised       *bool    `json:"return_initiated_compromised,omitempty"`
	ReturnInitiatedCompromisedReason *string  `json:"return_initiated_compromised_reason,omitempty"`
	OtherIssues                      *bool    `json:"other_issues,omitempty"`
	OtherIssuesReason                *string  `json:"other_issues_reason,omitempty"`
	IsRound                          *bool    `json:"is_round,omitempty"`
}

type GetOrdersByIDsRequest struct {
	AmazonOrderIDs []string `json:"amazon_order_ids" binding:"required,min=1"`
}

type OrderedAmazonOrderResult struct {
	RequestedAmazonOrderID string       `json:"requested_amazon_order_id"`
	Found                  bool         `json:"found"`
	Order                  *AmazonOrder `json:"order,omitempty"`
}

type GetOrdersByIDsResponse struct {
	Results               []OrderedAmazonOrderResult `json:"results"`
	MissingAmazonOrderIDs []string                   `json:"missing_amazon_order_ids"`
}

type AnalyticsPeriodStat struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Total      int     `json:"total,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

type AnalyticsTimePoint struct {
	Date  string  `json:"date"`
	Label string  `json:"label"`
	Count float64 `json:"count"`
}

type DashboardAnalytics struct {
	GeneratedAt                time.Time             `json:"generated_at"`
	ChartWindowDays            int                   `json:"chart_window_days"`
	MissingRiskWindowDays      int                   `json:"missing_risk_window_days"`
	RangeStart                 string                `json:"range_start,omitempty"`
	RangeEnd                   string                `json:"range_end,omitempty"`
	OrdersReceived             []AnalyticsPeriodStat `json:"orders_received"`
	ReturnsUpdated             []AnalyticsPeriodStat `json:"returns_updated"`
	SafetyClaimsUpdated        []AnalyticsPeriodStat `json:"safety_claims_updated"`
	IssuesUpdated              []AnalyticsPeriodStat `json:"issues_updated"`
	OrdersReceivedDaily        []AnalyticsTimePoint  `json:"orders_received_daily"`
	ReturnsUpdatedDaily        []AnalyticsTimePoint  `json:"returns_updated_daily"`
	SafetyClaimsUpdatedDaily   []AnalyticsTimePoint  `json:"safety_claims_updated_daily"`
	IssuesUpdatedDaily         []AnalyticsTimePoint  `json:"issues_updated_daily"`
	CustomerInputCoverageDaily []AnalyticsTimePoint  `json:"customer_input_coverage_daily"`
	MissingCustomerDetails     []AnalyticsPeriodStat `json:"missing_customer_details"`
	MissingDetailsRiskDaily    []AnalyticsPeriodStat `json:"missing_details_risk_daily"`
	OrdersByLocation           []AnalyticsChartSlice `json:"orders_by_location"`
	ThicknessDistribution      []AnalyticsChartSlice `json:"thickness_distribution"`
	MostOrderedSKUs            []AnalyticsChartSlice `json:"most_ordered_skus"`
	ReturnsByLocation          []AnalyticsChartSlice `json:"returns_by_location"`
}

type AnalyticsChartSlice struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ExecutiveDashboardFilters struct {
	DateRange   string     `json:"date_range,omitempty"`
	FromDate    *time.Time `json:"from_date,omitempty"`
	ToDate      *time.Time `json:"to_date,omitempty"`
	State       string     `json:"state,omitempty"`
	City        string     `json:"city,omitempty"`
	Thickness   string     `json:"thickness,omitempty"`
	OrderStatus string     `json:"order_status,omitempty"`
}

type ExecutiveDashboardSummary struct {
	TotalOrders         int     `json:"total_orders"`
	ManufacturedOrders  int     `json:"manufactured_orders"`
	CancelledOrders     int     `json:"cancelled_orders"`
	ReturnedOrders      int     `json:"returned_orders"`
	ReturnRate          float64 `json:"return_rate"`
	SafetyClaims        int     `json:"safety_claims"`
	OtherIssues         int     `json:"other_issues"`
	OpenReturns         int     `json:"open_returns"`
	PendingSafetyClaims int     `json:"pending_safety_claims"`
	PendingOtherIssues  int     `json:"pending_other_issues"`
}

type ExecutiveDashboardRecentActivityRow struct {
	AmazonOrderID   string     `json:"amazon_order_id"`
	ConfirmedDate   *time.Time `json:"confirmed_date,omitempty"`
	Customer        string     `json:"customer"`
	State           string     `json:"state"`
	Thickness       string     `json:"thickness"`
	OrderStatus     string     `json:"order_status"`
	ReturnInitiated bool       `json:"return_initiated"`
	SafetyClaimed   bool       `json:"safety_claimed"`
	OtherIssue      bool       `json:"other_issue"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ExecutiveDashboardResponse struct {
	GeneratedAt           time.Time                             `json:"generated_at"`
	DateRange             string                                `json:"date_range"`
	RangeStart            string                                `json:"range_start"`
	RangeEnd              string                                `json:"range_end"`
	AvailableStates       []string                              `json:"available_states"`
	AvailableCities       []string                              `json:"available_cities"`
	Summary               ExecutiveDashboardSummary             `json:"summary"`
	OrdersTrend           []AnalyticsTimePoint                  `json:"orders_trend"`
	ReturnsTrend          []AnalyticsTimePoint                  `json:"returns_trend"`
	CustomerInputGapTrend []AnalyticsTimePoint                  `json:"customer_input_gap_trend"`
	IssueDistribution     []AnalyticsChartSlice                 `json:"issue_distribution"`
	OrdersByThickness     []AnalyticsChartSlice                 `json:"orders_by_thickness"`
	OrdersByState         []AnalyticsChartSlice                 `json:"orders_by_state"`
	OrdersBySKU           []AnalyticsChartSlice                 `json:"orders_by_sku"`
	OrdersByPriceBand     []AnalyticsChartSlice                 `json:"orders_by_price_band"`
	RecentActivity        []ExecutiveDashboardRecentActivityRow `json:"recent_activity"`
}

type ReturnsDashboardFilters struct {
	DateRange                  string     `json:"date_range,omitempty"`
	FromDate                   *time.Time `json:"from_date,omitempty"`
	ToDate                     *time.Time `json:"to_date,omitempty"`
	State                      string     `json:"state,omitempty"`
	City                       string     `json:"city,omitempty"`
	Thickness                  string     `json:"thickness,omitempty"`
	OrderStatus                string     `json:"order_status,omitempty"`
	ReturnInitiated            *bool      `json:"return_initiated,omitempty"`
	ReturnInitiatedCompromised *bool      `json:"return_initiated_compromised,omitempty"`
}

type ReturnsDashboardSummary struct {
	TotalOrders          int     `json:"total_orders"`
	ReturnsInitiated     int     `json:"returns_initiated"`
	ReturnRate           float64 `json:"return_rate"`
	ReturnedOrders       int     `json:"returned_orders"`
	ReturnsCompromised   int     `json:"returns_compromised"`
	CompromiseRate       float64 `json:"compromise_rate"`
	PendingReturns       int     `json:"pending_returns"`
	AverageReturnsPerDay float64 `json:"average_returns_per_day"`
}

type ReturnsDashboardThicknessRow struct {
	Thickness  string  `json:"thickness"`
	Orders     int     `json:"orders"`
	Returns    int     `json:"returns"`
	ReturnRate float64 `json:"return_rate"`
}

type ReturnsDashboardStateRow struct {
	State      string  `json:"state"`
	Orders     int     `json:"orders"`
	Returns    int     `json:"returns"`
	ReturnRate float64 `json:"return_rate"`
}

type ReturnsDashboardPendingRow struct {
	AmazonOrderID  string     `json:"amazon_order_id"`
	ConfirmedDate  *time.Time `json:"confirmed_date,omitempty"`
	Customer       string     `json:"customer"`
	Phone          string     `json:"phone"`
	State          string     `json:"state"`
	City           string     `json:"city"`
	Thickness      string     `json:"thickness"`
	Quantity       float64    `json:"quantity"`
	ReturnReason   string     `json:"return_reason"`
	FollowupAction string     `json:"followup_action"`
	Compromised    bool       `json:"compromised"`
	OrderStatus    string     `json:"order_status"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ReturnsDashboardDetailRow struct {
	AmazonOrderID  string     `json:"amazon_order_id"`
	ConfirmedDate  *time.Time `json:"confirmed_date,omitempty"`
	Product        string     `json:"product"`
	Customer       string     `json:"customer"`
	Phone          string     `json:"phone"`
	State          string     `json:"state"`
	City           string     `json:"city"`
	Thickness      string     `json:"thickness"`
	Quantity       float64    `json:"quantity"`
	ReturnReason   string     `json:"return_reason"`
	FollowupAction string     `json:"followup_action"`
	Compromised    bool       `json:"compromised"`
	OrderStatus    string     `json:"order_status"`
	EventAt        *time.Time `json:"event_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ReturnsDashboardTopProductRow struct {
	Product    string  `json:"product"`
	Orders     int     `json:"orders"`
	Returns    int     `json:"returns"`
	ReturnRate float64 `json:"return_rate"`
}

type ReturnsDashboardResponse struct {
	GeneratedAt          time.Time                       `json:"generated_at"`
	DateRange            string                          `json:"date_range"`
	RangeStart           string                          `json:"range_start"`
	RangeEnd             string                          `json:"range_end"`
	AvailableStates      []string                        `json:"available_states"`
	AvailableCities      []string                        `json:"available_cities"`
	Summary              ReturnsDashboardSummary         `json:"summary"`
	ReturnsTrendDaily    []AnalyticsTimePoint            `json:"returns_trend_daily"`
	ReturnsTrendWeekly   []AnalyticsTimePoint            `json:"returns_trend_weekly"`
	ReturnsTrendMonthly  []AnalyticsTimePoint            `json:"returns_trend_monthly"`
	ThicknessPerformance []ReturnsDashboardThicknessRow  `json:"thickness_performance"`
	StatePerformance     []ReturnsDashboardStateRow      `json:"state_performance"`
	TopReturnCities      []AnalyticsChartSlice           `json:"top_return_cities"`
	ReturnReasons        []AnalyticsChartSlice           `json:"return_reasons"`
	FollowupActions      []AnalyticsChartSlice           `json:"followup_actions"`
	CompromisedBreakdown []AnalyticsChartSlice           `json:"compromised_breakdown"`
	ReturnsByOrderStatus []AnalyticsChartSlice           `json:"returns_by_order_status"`
	PendingReturns       []ReturnsDashboardPendingRow    `json:"pending_returns"`
	ReturnOrderDetails   []ReturnsDashboardDetailRow     `json:"return_order_details"`
	TopReturningProducts []ReturnsDashboardTopProductRow `json:"top_returning_products"`
}

type SafetyClaimsDashboardFilters struct {
	DateRange     string     `json:"date_range,omitempty"`
	FromDate      *time.Time `json:"from_date,omitempty"`
	ToDate        *time.Time `json:"to_date,omitempty"`
	State         string     `json:"state,omitempty"`
	City          string     `json:"city,omitempty"`
	Thickness     string     `json:"thickness,omitempty"`
	OrderStatus   string     `json:"order_status,omitempty"`
	SafetyClaimed *bool      `json:"safety_claimed,omitempty"`
}

type SafetyClaimsDashboardSummary struct {
	TotalOrders               int     `json:"total_orders"`
	SafetyClaimedOrders       int     `json:"safety_claimed_orders"`
	SafetyClaimRate           float64 `json:"safety_claim_rate"`
	ReturnedOrders            int     `json:"returned_orders"`
	ReturnedOrdersWithClaims  int     `json:"returned_orders_with_safety_claims"`
	SafetyClaimConversionRate float64 `json:"safety_claim_conversion_rate"`
	PendingSafetyClaims       int     `json:"pending_safety_claims"`
}

type SafetyClaimsDashboardInsight struct {
	HighestClaimState     string `json:"highest_claim_state"`
	HighestClaimThickness string `json:"highest_claim_thickness"`
	HighestClaimProduct   string `json:"highest_claim_product"`
	HighestClaimDayOfWeek string `json:"highest_claim_day_of_week"`
}

type SafetyClaimsDashboardThicknessRow struct {
	Thickness string  `json:"thickness"`
	Orders    int     `json:"orders"`
	Claims    int     `json:"claims"`
	ClaimRate float64 `json:"claim_rate"`
}

type SafetyClaimsDashboardStateRow struct {
	State     string  `json:"state"`
	Orders    int     `json:"orders"`
	Claims    int     `json:"claims"`
	ClaimRate float64 `json:"claim_rate"`
}

type SafetyClaimsDashboardCaseRow struct {
	AmazonOrderID     string     `json:"amazon_order_id"`
	ConfirmedDate     *time.Time `json:"confirmed_date,omitempty"`
	Customer          string     `json:"customer"`
	Phone             string     `json:"phone"`
	State             string     `json:"state"`
	City              string     `json:"city"`
	Thickness         string     `json:"thickness"`
	Product           string     `json:"product"`
	OrderStatus       string     `json:"order_status"`
	SafetyClaimIssues string     `json:"safety_claim_issues"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SafetyClaimsDashboardDetailRow struct {
	AmazonOrderID     string     `json:"amazon_order_id"`
	ConfirmedDate     *time.Time `json:"confirmed_date,omitempty"`
	Customer          string     `json:"customer"`
	Phone             string     `json:"phone"`
	State             string     `json:"state"`
	City              string     `json:"city"`
	Thickness         string     `json:"thickness"`
	Product           string     `json:"product"`
	OrderStatus       string     `json:"order_status"`
	SafetyClaimed     bool       `json:"safety_claimed"`
	SafetyClaimIssues string     `json:"safety_claim_issues"`
	EventAt           *time.Time `json:"event_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SafetyClaimsDashboardTopProductRow struct {
	Product   string  `json:"product"`
	Orders    int     `json:"orders"`
	Claims    int     `json:"claims"`
	ClaimRate float64 `json:"claim_rate"`
}

type SafetyClaimsDashboardResponse struct {
	GeneratedAt          time.Time                            `json:"generated_at"`
	DateRange            string                               `json:"date_range"`
	RangeStart           string                               `json:"range_start"`
	RangeEnd             string                               `json:"range_end"`
	AvailableStates      []string                             `json:"available_states"`
	AvailableCities      []string                             `json:"available_cities"`
	Summary              SafetyClaimsDashboardSummary         `json:"summary"`
	Insights             SafetyClaimsDashboardInsight         `json:"insights"`
	ClaimsTrendDaily     []AnalyticsTimePoint                 `json:"claims_trend_daily"`
	ClaimsTrendWeekly    []AnalyticsTimePoint                 `json:"claims_trend_weekly"`
	ClaimsTrendMonthly   []AnalyticsTimePoint                 `json:"claims_trend_monthly"`
	ThicknessPerformance []SafetyClaimsDashboardThicknessRow  `json:"thickness_performance"`
	StatePerformance     []SafetyClaimsDashboardStateRow      `json:"state_performance"`
	TopClaimCities       []AnalyticsChartSlice                `json:"top_claim_cities"`
	SafetyClaimIssues    []AnalyticsChartSlice                `json:"safety_claim_issues"`
	ClaimsByOrderStatus  []AnalyticsChartSlice                `json:"claims_by_order_status"`
	SafetyClaimCases     []SafetyClaimsDashboardCaseRow       `json:"safety_claim_cases"`
	OrderDetails         []SafetyClaimsDashboardDetailRow     `json:"order_details"`
	TopClaimProducts     []SafetyClaimsDashboardTopProductRow `json:"top_claim_products"`
}

type RepeatCustomerOrder struct {
	AmazonOrderID  string     `json:"amazon_order_id"`
	ConfirmedDate  *time.Time `json:"confirmed_date,omitempty"`
	OrderStatus    string     `json:"order_status"`
	Customer       string     `json:"customer"`
	Phone          string     `json:"phone"`
	Address        string     `json:"address"`
	City           string     `json:"city"`
	State          string     `json:"state"`
	ProductSummary string     `json:"product_summary"`
}

type RepeatCustomerGroup struct {
	GroupKey    string                `json:"group_key"`
	DisplayName string                `json:"display_name"`
	OrderCount  int                   `json:"order_count"`
	Orders      []RepeatCustomerOrder `json:"orders"`
}

type RepeatCustomerResponse struct {
	Scope     string                `json:"scope"`
	ByPhone   []RepeatCustomerGroup `json:"by_phone"`
	ByAddress []RepeatCustomerGroup `json:"by_address"`
}
