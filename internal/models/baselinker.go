package models

// GetOrdersRequest represents BaseLinker API request
type GetOrdersRequest struct {
	DateConfirmedFrom int64 `json:"date_confirmed_from"`
}

// GetOrdersResponse represents BaseLinker API response
type GetOrdersResponse struct {
	Status string            `json:"status"`
	Orders []BaseLinkerOrder `json:"orders"`
}

// BaseLinkerOrder represents a single order from BaseLinker
type BaseLinkerOrder struct {
	OrderID               int64                    `json:"order_id"`
	ShopOrderID           int64                    `json:"shop_order_id"`
	ExternalOrderID       string                   `json:"external_order_id"`
	OrderSource           string                   `json:"order_source"`
	OrderSourceID         int64                    `json:"order_source_id"`
	OrderSourceInfo       string                   `json:"order_source_info"`
	OrderStatusID         int64                    `json:"order_status_id"`
	Confirmed             bool                     `json:"confirmed"`
	DateConfirmed         int64                    `json:"date_confirmed"`
	DateAdd               int64                    `json:"date_add"`
	DateInStatus          int64                    `json:"date_in_status"`
	UserLogin             string                   `json:"user_login"`
	Phone                 string                   `json:"phone"`
	Email                 string                   `json:"email"`
	UserComments          string                   `json:"user_comments"`
	AdminComments         string                   `json:"admin_comments"`
	Currency              string                   `json:"currency"`
	PaymentMethod         string                   `json:"payment_method"`
	PaymentMethodCOD      string                   `json:"payment_method_cod"`
	PaymentDone           float64                  `json:"payment_done"`
	DeliveryMethodID      int64                    `json:"delivery_method_id"`
	DeliveryMethod        string                   `json:"delivery_method"`
	DeliveryPrice         float64                  `json:"delivery_price"`
	DeliveryPackageModule string                   `json:"delivery_package_module"`
	DeliveryPackageNr     string                   `json:"delivery_package_nr"`
	DeliveryFullname      string                   `json:"delivery_fullname"`
	DeliveryCompany       string                   `json:"delivery_company"`
	DeliveryAddress       string                   `json:"delivery_address"`
	DeliveryCity          string                   `json:"delivery_city"`
	DeliveryState         string                   `json:"delivery_state"`
	DeliveryPostcode      string                   `json:"delivery_postcode"`
	DeliveryCountryCode   string                   `json:"delivery_country_code"`
	DeliveryCountry       string                   `json:"delivery_country"`
	DeliveryPointID       string                   `json:"delivery_point_id"`
	DeliveryPointName     string                   `json:"delivery_point_name"`
	DeliveryPointAddress  string                   `json:"delivery_point_address"`
	DeliveryPointPostcode string                   `json:"delivery_point_postcode"`
	DeliveryPointCity     string                   `json:"delivery_point_city"`
	InvoiceFullname       string                   `json:"invoice_fullname"`
	InvoiceCompany        string                   `json:"invoice_company"`
	InvoiceNip            string                   `json:"invoice_nip"`
	InvoiceAddress        string                   `json:"invoice_address"`
	InvoiceCity           string                   `json:"invoice_city"`
	InvoiceState          string                   `json:"invoice_state"`
	InvoicePostcode       string                   `json:"invoice_postcode"`
	InvoiceCountryCode    string                   `json:"invoice_country_code"`
	InvoiceCountry        string                   `json:"invoice_country"`
	WantInvoice           string                   `json:"want_invoice"`
	ExtraField1           string                   `json:"extra_field_1"`
	ExtraField2           string                   `json:"extra_field_2"`
	OrderPage             string                   `json:"order_page"`
	PickState             int                      `json:"pick_state"`
	PackState             int                      `json:"pack_state"`
	Star                  int                      `json:"star"`
	CRMClientID           int64                    `json:"crm_client_id"`
	Products              []BaseLinkerOrderProduct `json:"products"`
}

// BaseLinkerOrderProduct represents a product in an order
type BaseLinkerOrderProduct struct {
	OrderProductID int64   `json:"order_product_id"`
	Storage        string  `json:"storage"`
	StorageID      int64   `json:"storage_id"`
	ProductID      string  `json:"product_id"`
	VariantID      string  `json:"variant_id"`
	Name           string  `json:"name"`
	Attributes     string  `json:"attributes"`
	SKU            string  `json:"sku"`
	EAN            string  `json:"ean"`
	Location       string  `json:"location"`
	WarehouseID    int64   `json:"warehouse_id"`
	AuctionID      string  `json:"auction_id"`
	PriceBrutto    float64 `json:"price_brutto"`
	TaxRate        float64 `json:"tax_rate"`
	Quantity       float64 `json:"quantity"`
	Weight         float64 `json:"weight"`
	BundleID       int64   `json:"bundle_id"`
}
