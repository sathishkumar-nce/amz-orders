package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// DirectOrder represents the direct_orders table.
type DirectOrder struct {
	ID                int64             `json:"id"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         sql.NullTime      `json:"deleted_at,omitempty"`
	Source            sql.NullString    `json:"source,omitempty"`
	OrderID           string            `json:"order_id"`
	OrderStatus       string            `json:"order_status"`
	CourierType       sql.NullString    `json:"courier_type,omitempty"`
	CourierName       sql.NullString    `json:"courier_name,omitempty"`
	AWB               sql.NullString    `json:"awb,omitempty"`
	PaymentStatus     string            `json:"payment_status"`
	Amount            sql.NullFloat64   `json:"amount,omitempty"`
	AdvanceAmount     sql.NullFloat64   `json:"advance_amount,omitempty"`
	CODAmount         sql.NullFloat64   `json:"cod_amount,omitempty"`
	CustomerName      sql.NullString    `json:"customer_name,omitempty"`
	Address           sql.NullString    `json:"address,omitempty"`
	Pincode           sql.NullString    `json:"pincode,omitempty"`
	Mobile            sql.NullString    `json:"mobile,omitempty"`
	AlternateMobile   sql.NullString    `json:"alternate_mobile,omitempty"`
	Email             sql.NullString    `json:"email,omitempty"`
	AlternateEmail    sql.NullString    `json:"alternate_email,omitempty"`
	Remarks           sql.NullString    `json:"remarks,omitempty"`
	Priority          string            `json:"priority"`
	Issues            sql.NullString    `json:"issues,omitempty"`
	UpdatedBy         sql.NullString    `json:"updated_by,omitempty"`
	City              sql.NullString    `json:"city,omitempty"`
	State             sql.NullString    `json:"state,omitempty"`
	Country           sql.NullString    `json:"country,omitempty"`
	Landmark          sql.NullString    `json:"landmark,omitempty"`
	ShipmentType      sql.NullString    `json:"shipment_type,omitempty"`
	ServiceType       sql.NullString    `json:"service_type,omitempty"`
	PickupLocation    sql.NullString    `json:"pickup_location,omitempty"`
	PackageCount      sql.NullInt64     `json:"package_count,omitempty"`
	TotalWeight       sql.NullFloat64   `json:"total_weight,omitempty"`
	LengthCM          sql.NullFloat64   `json:"length_cm,omitempty"`
	WidthCM           sql.NullFloat64   `json:"width_cm,omitempty"`
	HeightCM          sql.NullFloat64   `json:"height_cm,omitempty"`
	InvoiceDate       sql.NullTime      `json:"invoice_date,omitempty"`
	CourierOrderID    sql.NullString    `json:"courier_order_id,omitempty"`
	CourierStatus     sql.NullString    `json:"courier_status,omitempty"`
	ManifestedAt      sql.NullTime      `json:"manifested_at,omitempty"`
	PickupRequestedAt sql.NullTime      `json:"pickup_requested_at,omitempty"`
	CourierPayload    json.RawMessage   `json:"courier_payload,omitempty"`
	Items             []DirectOrderItem `json:"items,omitempty"`
}

// DirectOrderItem represents the direct_order_items table.
type DirectOrderItem struct {
	ID                   int64           `json:"id"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	OrderID              string          `json:"order_id"`
	Item                 sql.NullString  `json:"item,omitempty"`
	Quantity             int             `json:"quantity"`
	Dimension            sql.NullString  `json:"dimension,omitempty"`
	Thickness            sql.NullString  `json:"thickness,omitempty"`
	Weight               sql.NullFloat64 `json:"weight,omitempty"`
	Amount               sql.NullFloat64 `json:"amount,omitempty"`
	Remark               sql.NullString  `json:"remark,omitempty"`
	UpdatedBy            sql.NullString  `json:"updated_by,omitempty"`
	SKU                  sql.NullString  `json:"sku,omitempty"`
	HSN                  sql.NullString  `json:"hsn,omitempty"`
	UnitPrice            sql.NullFloat64 `json:"unit_price,omitempty"`
	TaxRate              sql.NullFloat64 `json:"tax_rate,omitempty"`
	CustomerWidthInches  sql.NullFloat64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInches sql.NullFloat64 `json:"customer_length_in_inches,omitempty"`
	CustomerWidthMM      sql.NullFloat64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthMM     sql.NullFloat64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes sql.NullString  `json:"corner_radius_and_notes,omitempty"`
}

type CreateDirectOrderRequest struct {
	Source          string                         `json:"source"`
	OrderID         string                         `json:"order_id"`
	OrderStatus     string                         `json:"order_status"`
	CourierType     *string                        `json:"courier_type,omitempty"`
	CourierName     *string                        `json:"courier_name,omitempty"`
	AWB             *string                        `json:"awb,omitempty"`
	PaymentStatus   string                         `json:"payment_status"`
	Amount          *float64                       `json:"amount,omitempty"`
	AdvanceAmount   *float64                       `json:"advance_amount,omitempty"`
	CODAmount       *float64                       `json:"cod_amount,omitempty"`
	CustomerName    *string                        `json:"customer_name,omitempty"`
	Address         *string                        `json:"address,omitempty"`
	Pincode         *string                        `json:"pincode,omitempty"`
	Mobile          *string                        `json:"mobile,omitempty"`
	AlternateMobile *string                        `json:"alternate_mobile,omitempty"`
	Email           *string                        `json:"email,omitempty"`
	AlternateEmail  *string                        `json:"alternate_email,omitempty"`
	Remarks         *string                        `json:"remarks,omitempty"`
	Priority        string                         `json:"priority"`
	Issues          *string                        `json:"issues,omitempty"`
	City            *string                        `json:"city,omitempty"`
	State           *string                        `json:"state,omitempty"`
	Country         *string                        `json:"country,omitempty"`
	Landmark        *string                        `json:"landmark,omitempty"`
	ShipmentType    *string                        `json:"shipment_type,omitempty"`
	ServiceType     *string                        `json:"service_type,omitempty"`
	PickupLocation  *string                        `json:"pickup_location,omitempty"`
	PackageCount    *int                           `json:"package_count,omitempty"`
	TotalWeight     *float64                       `json:"total_weight,omitempty"`
	LengthCM        *float64                       `json:"length_cm,omitempty"`
	WidthCM         *float64                       `json:"width_cm,omitempty"`
	HeightCM        *float64                       `json:"height_cm,omitempty"`
	InvoiceDate     *string                        `json:"invoice_date,omitempty"`
	Items           []CreateDirectOrderItemRequest `json:"items,omitempty"`
	UpdatedBy       *string                        `json:"-"`
}

type CreateDirectOrderItemRequest struct {
	Item                 *string  `json:"item,omitempty"`
	Quantity             int      `json:"quantity"`
	Dimension            *string  `json:"dimension,omitempty"`
	Thickness            *string  `json:"thickness,omitempty"`
	Weight               *float64 `json:"weight,omitempty"`
	Amount               *float64 `json:"amount,omitempty"`
	Remark               *string  `json:"remark,omitempty"`
	SKU                  *string  `json:"sku,omitempty"`
	HSN                  *string  `json:"hsn,omitempty"`
	UnitPrice            *float64 `json:"unit_price,omitempty"`
	TaxRate              *float64 `json:"tax_rate,omitempty"`
	CustomerWidthInches  *float64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInches *float64 `json:"customer_length_in_inches,omitempty"`
	CustomerWidthMM      *float64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthMM     *float64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes *string  `json:"corner_radius_and_notes,omitempty"`
}

type UpdateDirectOrderRequest struct {
	Source          *string                         `json:"source,omitempty"`
	OrderStatus     *string                         `json:"order_status,omitempty"`
	CourierType     *string                         `json:"courier_type,omitempty"`
	CourierName     *string                         `json:"courier_name,omitempty"`
	AWB             *string                         `json:"awb,omitempty"`
	PaymentStatus   *string                         `json:"payment_status,omitempty"`
	Amount          *float64                        `json:"amount,omitempty"`
	AdvanceAmount   *float64                        `json:"advance_amount,omitempty"`
	CODAmount       *float64                        `json:"cod_amount,omitempty"`
	CustomerName    *string                         `json:"customer_name,omitempty"`
	Address         *string                         `json:"address,omitempty"`
	Pincode         *string                         `json:"pincode,omitempty"`
	Mobile          *string                         `json:"mobile,omitempty"`
	AlternateMobile *string                         `json:"alternate_mobile,omitempty"`
	Email           *string                         `json:"email,omitempty"`
	AlternateEmail  *string                         `json:"alternate_email,omitempty"`
	Remarks         *string                         `json:"remarks,omitempty"`
	Priority        *string                         `json:"priority,omitempty"`
	Issues          *string                         `json:"issues,omitempty"`
	City            *string                         `json:"city,omitempty"`
	State           *string                         `json:"state,omitempty"`
	Country         *string                         `json:"country,omitempty"`
	Landmark        *string                         `json:"landmark,omitempty"`
	ShipmentType    *string                         `json:"shipment_type,omitempty"`
	ServiceType     *string                         `json:"service_type,omitempty"`
	PickupLocation  *string                         `json:"pickup_location,omitempty"`
	PackageCount    *int                            `json:"package_count,omitempty"`
	TotalWeight     *float64                        `json:"total_weight,omitempty"`
	LengthCM        *float64                        `json:"length_cm,omitempty"`
	WidthCM         *float64                        `json:"width_cm,omitempty"`
	HeightCM        *float64                        `json:"height_cm,omitempty"`
	InvoiceDate     *string                         `json:"invoice_date,omitempty"`
	Items           *[]CreateDirectOrderItemRequest `json:"items,omitempty"`
	UpdatedBy       *string                         `json:"-"`
}

type DirectOrderFilters struct {
	OrderID       string
	Search        string
	AWB           string
	OrderStatus   string
	PaymentStatus string
	Priority      string
	Source        string
	Mobile        string
	CustomerName  string
	Item          string
	Quantity      int
	Pincode       string
	DateExact     string
	DateFrom      string
	DateTo        string
	Page          int
	Limit         int
}

type DirectOrderBulkForwardRequest struct {
	OrderIDs []string `json:"order_ids"`
}

type DirectOrderBulkForwardItem struct {
	OrderID string               `json:"order_id"`
	Order   *DirectOrderResponse `json:"order,omitempty"`
	Error   string               `json:"error,omitempty"`
}

type DirectOrderBulkForwardResponse struct {
	Message      string                       `json:"message"`
	Requested    int                          `json:"requested"`
	SuccessCount int                          `json:"success_count"`
	FailureCount int                          `json:"failure_count"`
	Successes    []DirectOrderBulkForwardItem `json:"successes"`
	Failures     []DirectOrderBulkForwardItem `json:"failures"`
}

type DirectOrderPincodeLookupResult struct {
	Pincode     string                              `json:"pincode"`
	City        string                              `json:"city,omitempty"`
	District    string                              `json:"district,omitempty"`
	State       string                              `json:"state,omitempty"`
	StateCode   string                              `json:"state_code,omitempty"`
	Country     string                              `json:"country,omitempty"`
	Serviceable bool                                `json:"serviceable"`
	COD         bool                                `json:"cod"`
	Prepaid     bool                                `json:"prepaid"`
	Raw         []DirectOrderPincodeLookupCandidate `json:"raw,omitempty"`
}

type DirectOrderPincodeLookupCandidate struct {
	Pincode     string `json:"pincode"`
	City        string `json:"city,omitempty"`
	District    string `json:"district,omitempty"`
	State       string `json:"state,omitempty"`
	StateCode   string `json:"state_code,omitempty"`
	Country     string `json:"country,omitempty"`
	Serviceable bool   `json:"serviceable"`
	COD         bool   `json:"cod"`
	Prepaid     bool   `json:"prepaid"`
}

type DirectOrderExecutiveDashboardSummary struct {
	TotalOrders        int     `json:"total_orders"`
	ManufacturedOrders int     `json:"manufactured_orders"`
	OtherIssuesOrders  int     `json:"other_issues_orders"`
	CancelledOrders    int     `json:"cancelled_orders"`
	OnHoldOrders       int     `json:"on_hold_orders"`
	AveragePerDay      float64 `json:"average_per_day"`
}

type DirectOrderExecutiveDashboardResponse struct {
	GeneratedAt       time.Time                            `json:"generated_at"`
	DateRange         string                               `json:"date_range"`
	RangeStart        string                               `json:"range_start"`
	RangeEnd          string                               `json:"range_end"`
	AvailableStates   []string                             `json:"available_states"`
	AvailableCities   []string                             `json:"available_cities"`
	Summary           DirectOrderExecutiveDashboardSummary `json:"summary"`
	OrdersTrend       []AnalyticsTimePoint                 `json:"orders_trend"`
	OtherIssuesTrend  []AnalyticsTimePoint                 `json:"other_issues_trend"`
	OrdersByState     []AnalyticsChartSlice                `json:"orders_by_state"`
	OrdersByCity      []AnalyticsChartSlice                `json:"orders_by_city"`
	OrdersByThickness []AnalyticsChartSlice                `json:"orders_by_thickness"`
}

type DirectOrderItemResponse struct {
	ID                   int64    `json:"id"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
	OrderID              string   `json:"order_id"`
	Item                 *string  `json:"item,omitempty"`
	Quantity             int      `json:"quantity"`
	Dimension            *string  `json:"dimension,omitempty"`
	Thickness            *string  `json:"thickness,omitempty"`
	Weight               *float64 `json:"weight,omitempty"`
	Amount               *float64 `json:"amount,omitempty"`
	Remark               *string  `json:"remark,omitempty"`
	UpdatedBy            *string  `json:"updated_by,omitempty"`
	SKU                  *string  `json:"sku,omitempty"`
	HSN                  *string  `json:"hsn,omitempty"`
	UnitPrice            *float64 `json:"unit_price,omitempty"`
	TaxRate              *float64 `json:"tax_rate,omitempty"`
	CustomerWidthInches  *float64 `json:"customer_width_in_inches,omitempty"`
	CustomerLengthInches *float64 `json:"customer_length_in_inches,omitempty"`
	CustomerWidthMM      *float64 `json:"customer_width_in_mm,omitempty"`
	CustomerLengthMM     *float64 `json:"customer_length_in_mm,omitempty"`
	CornerRadiusAndNotes *string  `json:"corner_radius_and_notes,omitempty"`
}

type DirectOrderResponse struct {
	ID                int64                     `json:"id"`
	CreatedAt         string                    `json:"created_at"`
	UpdatedAt         string                    `json:"updated_at"`
	DeletedAt         *string                   `json:"deleted_at,omitempty"`
	Source            *string                   `json:"source,omitempty"`
	OrderID           string                    `json:"order_id"`
	OrderStatus       string                    `json:"order_status"`
	CourierType       *string                   `json:"courier_type,omitempty"`
	CourierName       *string                   `json:"courier_name,omitempty"`
	AWB               *string                   `json:"awb,omitempty"`
	PaymentStatus     string                    `json:"payment_status"`
	Amount            *float64                  `json:"amount,omitempty"`
	AdvanceAmount     *float64                  `json:"advance_amount,omitempty"`
	CODAmount         *float64                  `json:"cod_amount,omitempty"`
	CustomerName      *string                   `json:"customer_name,omitempty"`
	Address           *string                   `json:"address,omitempty"`
	Pincode           *string                   `json:"pincode,omitempty"`
	Mobile            *string                   `json:"mobile,omitempty"`
	AlternateMobile   *string                   `json:"alternate_mobile,omitempty"`
	Email             *string                   `json:"email,omitempty"`
	AlternateEmail    *string                   `json:"alternate_email,omitempty"`
	Remarks           *string                   `json:"remarks,omitempty"`
	Priority          string                    `json:"priority"`
	Issues            *string                   `json:"issues,omitempty"`
	UpdatedBy         *string                   `json:"updated_by,omitempty"`
	City              *string                   `json:"city,omitempty"`
	State             *string                   `json:"state,omitempty"`
	Country           *string                   `json:"country,omitempty"`
	Landmark          *string                   `json:"landmark,omitempty"`
	ShipmentType      *string                   `json:"shipment_type,omitempty"`
	ServiceType       *string                   `json:"service_type,omitempty"`
	PickupLocation    *string                   `json:"pickup_location,omitempty"`
	PackageCount      *int64                    `json:"package_count,omitempty"`
	TotalWeight       *float64                  `json:"total_weight,omitempty"`
	LengthCM          *float64                  `json:"length_cm,omitempty"`
	WidthCM           *float64                  `json:"width_cm,omitempty"`
	HeightCM          *float64                  `json:"height_cm,omitempty"`
	InvoiceDate       *string                   `json:"invoice_date,omitempty"`
	CourierOrderID    *string                   `json:"courier_order_id,omitempty"`
	CourierStatus     *string                   `json:"courier_status,omitempty"`
	ManifestedAt      *string                   `json:"manifested_at,omitempty"`
	PickupRequestedAt *string                   `json:"pickup_requested_at,omitempty"`
	CourierPayload    json.RawMessage           `json:"courier_payload,omitempty"`
	Items             []DirectOrderItemResponse `json:"items"`
}

type DelhiveryForwardOrderResult struct {
	CourierName       string
	AWB               *string
	CourierOrderID    *string
	CourierStatus     *string
	ManifestedAt      *time.Time
	PickupRequestedAt *time.Time
	CourierPayload    json.RawMessage
}

func ToDirectOrderResponse(order *DirectOrder) *DirectOrderResponse {
	if order == nil {
		return nil
	}

	items := make([]DirectOrderItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, DirectOrderItemResponse{
			ID:                   item.ID,
			CreatedAt:            item.CreatedAt.Format(time.RFC3339),
			UpdatedAt:            item.UpdatedAt.Format(time.RFC3339),
			OrderID:              item.OrderID,
			Item:                 nullStringPtr(item.Item),
			Quantity:             item.Quantity,
			Dimension:            nullStringPtr(item.Dimension),
			Thickness:            nullStringPtr(item.Thickness),
			Weight:               nullFloat64Ptr(item.Weight),
			Amount:               nullFloat64Ptr(item.Amount),
			Remark:               nullStringPtr(item.Remark),
			UpdatedBy:            nullStringPtr(item.UpdatedBy),
			SKU:                  nullStringPtr(item.SKU),
			HSN:                  nullStringPtr(item.HSN),
			UnitPrice:            nullFloat64Ptr(item.UnitPrice),
			TaxRate:              nullFloat64Ptr(item.TaxRate),
			CustomerWidthInches:  nullFloat64Ptr(item.CustomerWidthInches),
			CustomerLengthInches: nullFloat64Ptr(item.CustomerLengthInches),
			CustomerWidthMM:      nullFloat64Ptr(item.CustomerWidthMM),
			CustomerLengthMM:     nullFloat64Ptr(item.CustomerLengthMM),
			CornerRadiusAndNotes: nullStringPtr(item.CornerRadiusAndNotes),
		})
	}

	return &DirectOrderResponse{
		ID:                order.ID,
		CreatedAt:         order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         order.UpdatedAt.Format(time.RFC3339),
		DeletedAt:         nullTimePtr(order.DeletedAt),
		Source:            nullStringPtr(order.Source),
		OrderID:           order.OrderID,
		OrderStatus:       order.OrderStatus,
		CourierType:       nullStringPtr(order.CourierType),
		CourierName:       nullStringPtr(order.CourierName),
		AWB:               nullStringPtr(order.AWB),
		PaymentStatus:     order.PaymentStatus,
		Amount:            nullFloat64Ptr(order.Amount),
		AdvanceAmount:     nullFloat64Ptr(order.AdvanceAmount),
		CODAmount:         nullFloat64Ptr(order.CODAmount),
		CustomerName:      nullStringPtr(order.CustomerName),
		Address:           nullStringPtr(order.Address),
		Pincode:           nullStringPtr(order.Pincode),
		Mobile:            nullStringPtr(order.Mobile),
		AlternateMobile:   nullStringPtr(order.AlternateMobile),
		Email:             nullStringPtr(order.Email),
		AlternateEmail:    nullStringPtr(order.AlternateEmail),
		Remarks:           nullStringPtr(order.Remarks),
		Priority:          order.Priority,
		Issues:            nullStringPtr(order.Issues),
		UpdatedBy:         nullStringPtr(order.UpdatedBy),
		City:              nullStringPtr(order.City),
		State:             nullStringPtr(order.State),
		Country:           nullStringPtr(order.Country),
		Landmark:          nullStringPtr(order.Landmark),
		ShipmentType:      nullStringPtr(order.ShipmentType),
		ServiceType:       nullStringPtr(order.ServiceType),
		PickupLocation:    nullStringPtr(order.PickupLocation),
		PackageCount:      nullInt64Ptr(order.PackageCount),
		TotalWeight:       nullFloat64Ptr(order.TotalWeight),
		LengthCM:          nullFloat64Ptr(order.LengthCM),
		WidthCM:           nullFloat64Ptr(order.WidthCM),
		HeightCM:          nullFloat64Ptr(order.HeightCM),
		InvoiceDate:       nullTimePtr(order.InvoiceDate),
		CourierOrderID:    nullStringPtr(order.CourierOrderID),
		CourierStatus:     nullStringPtr(order.CourierStatus),
		ManifestedAt:      nullTimePtr(order.ManifestedAt),
		PickupRequestedAt: nullTimePtr(order.PickupRequestedAt),
		CourierPayload:    order.CourierPayload,
		Items:             items,
	}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func nullTimePtr(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	v := value.Time.Format(time.RFC3339)
	return &v
}
