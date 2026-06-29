package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sathishkumar-nce/amz-orders/internal/integrations/baselinker"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

type OrderService struct {
	repo             *repository.OrderRepository
	priorityRuleRepo *repository.PriorityRuleRepository
	baseLinkerClient *baselinker.Client
}

func NewOrderService(repo *repository.OrderRepository, priorityRuleRepo *repository.PriorityRuleRepository, blClient *baselinker.Client) *OrderService {
	return &OrderService{
		repo:             repo,
		priorityRuleRepo: priorityRuleRepo,
		baseLinkerClient: blClient,
	}
}

// ImportFromBaseLinker fetches orders from BaseLinker and imports them
func (s *OrderService) ImportFromBaseLinker(ctx context.Context, dateConfirmedFrom int64) (int, int, int, error) {
	log.Printf("🚚 ImportFromBaseLinker started (date_confirmed_from=%d)", dateConfirmedFrom)
	resp, err := s.baseLinkerClient.GetOrders(ctx, dateConfirmedFrom)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to fetch orders from BaseLinker: %w", err)
	}

	log.Printf("BaseLinker import: fetched=%d, processing_all_for_upsert_refresh=true", len(resp.Orders))
	return s.importOrders(ctx, resp.Orders)
}

// filterAlreadyImported drops BaseLinker orders whose ExternalOrderID is already
// present in the most recent `limit` rows of amazon_orders. Returns the filtered
// slice and the count of skipped orders.
func (s *OrderService) filterAlreadyImported(ctx context.Context, blOrders []models.BaseLinkerOrder, limit int) ([]models.BaseLinkerOrder, int) {
	if len(blOrders) == 0 {
		return blOrders, 0
	}

	existingIDs, err := s.repo.GetLatestOrderIDs(ctx, limit)
	if err != nil {
		log.Printf("⚠️  Failed to fetch latest order ids for de-dup, proceeding without filter: %v", err)
		return blOrders, 0
	}

	if len(existingIDs) == 0 {
		return blOrders, 0
	}

	existingSet := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = struct{}{}
	}

	filtered := make([]models.BaseLinkerOrder, 0, len(blOrders))
	skipped := 0
	for _, o := range blOrders {
		if _, found := existingSet[o.ExternalOrderID]; found {
			skipped++
			continue
		}
		filtered = append(filtered, o)
	}

	return filtered, skipped
}

// ImportFromFile imports orders from a local JSON file
func (s *OrderService) ImportFromFile(ctx context.Context, filePath string) (int, int, int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to read file: %w", err)
	}

	var resp models.GetOrdersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if resp.Status != "SUCCESS" {
		return 0, 0, 0, fmt.Errorf("invalid response status: %s", resp.Status)
	}

	return s.importOrders(ctx, resp.Orders)
}

func (s *OrderService) importOrders(ctx context.Context, blOrders []models.BaseLinkerOrder) (int, int, int, error) {
	totalFetched := len(blOrders)
	totalOrdersUpserted := 0
	totalProductsUpserted := 0
	errorCount := 0
	priorityRuleMap := map[string]map[string]struct{}{}

	if s.priorityRuleRepo != nil {
		rules, err := s.priorityRuleRepo.GetPrioritySKUMap(ctx)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("load priority rules: %w", err)
		}
		priorityRuleMap = rules
	}

	for _, blOrder := range blOrders {
		log.Printf("🔄 Processing BaseLinker order external_order_id=%s order_id=%d products=%d", blOrder.ExternalOrderID, blOrder.OrderID, len(blOrder.Products))

		// Convert BaseLinker order to Amazon order
		order := s.convertToAmazonOrder(&blOrder)
		order.Priority = determineOrderPriority(blOrder.Products, priorityRuleMap)

		// Convert products
		products := s.convertToOrderProducts(&blOrder)

		// Upsert order and products
		if err := s.repo.UpsertOrder(ctx, order, products); err != nil {
			log.Printf("Failed to upsert order %s: %v", order.AmazonOrderID, err)
			errorCount++
			continue
		}

		totalOrdersUpserted++
		totalProductsUpserted += len(products)
	}

	log.Printf("Import complete: fetched=%d, orders_upserted=%d, products_upserted=%d, errors=%d",
		totalFetched, totalOrdersUpserted, totalProductsUpserted, errorCount)

	return totalFetched, totalOrdersUpserted, totalProductsUpserted, nil
}

func determineOrderPriority(products []models.BaseLinkerOrderProduct, priorityRuleMap map[string]map[string]struct{}) string {
	skus := make([]string, 0, len(products))
	for _, product := range products {
		skus = append(skus, product.SKU)
	}

	return determineOrderPriorityFromSKUs(skus, priorityRuleMap)
}

func determineOrderPriorityFromSKUs(skus []string, priorityRuleMap map[string]map[string]struct{}) string {
	priorityOrder := []string{"p1", "p2", "p3", "p4"}
	matched := make(map[string]bool, len(priorityOrder))

	for _, rawSKU := range skus {
		sku := strings.ToUpper(strings.TrimSpace(rawSKU))
		if sku == "" {
			continue
		}

		for _, priority := range priorityOrder {
			if skuSet, exists := priorityRuleMap[priority]; exists {
				if _, matchedSKU := skuSet[sku]; matchedSKU {
					matched[priority] = true
				}
			}
		}
	}

	for _, priority := range priorityOrder {
		if matched[priority] {
			return priority
		}
	}

	return "p3"
}

func (s *OrderService) RecalculatePriorities(ctx context.Context) (int, error) {
	if s.priorityRuleRepo == nil {
		return 0, nil
	}

	priorityRuleMap, err := s.priorityRuleRepo.GetPrioritySKUMap(ctx)
	if err != nil {
		return 0, fmt.Errorf("load priority rules: %w", err)
	}

	orders, err := s.repo.ListOrderPriorityInputs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list order priority inputs: %w", err)
	}

	updatedCount := 0
	for _, order := range orders {
		nextPriority := determineOrderPriorityFromSKUs(order.SKUs, priorityRuleMap)
		currentPriority := strings.TrimSpace(strings.ToLower(order.Priority))
		if currentPriority == nextPriority {
			continue
		}

		if err := s.repo.UpdateOrderPriority(ctx, order.AmazonOrderID, nextPriority); err != nil {
			return updatedCount, fmt.Errorf("update priority for order %s: %w", order.AmazonOrderID, err)
		}

		updatedCount++
	}

	return updatedCount, nil
}

func (s *OrderService) convertToAmazonOrder(bl *models.BaseLinkerOrder) *models.AmazonOrder {
	return &models.AmazonOrder{
		AmazonOrderID:         bl.ExternalOrderID,
		BaseLinkerOrderID:     bl.OrderID,
		ShopOrderID:           bl.ShopOrderID,
		OrderSource:           utils.StringToNullString(bl.OrderSource),
		OrderSourceID:         utils.Int64ToNullInt64(bl.OrderSourceID),
		OrderSourceInfo:       utils.StringToNullString(bl.OrderSourceInfo),
		OrderStatusID:         bl.OrderStatusID,
		Confirmed:             bl.Confirmed,
		DateConfirmed:         utils.UnixToNullTime(bl.DateConfirmed),
		DateAdd:               utils.UnixToNullTime(bl.DateAdd),
		DateInStatus:          utils.UnixToNullTime(bl.DateInStatus),
		UserLogin:             utils.StringToNullString(bl.UserLogin),
		Phone:                 utils.StringToNullString(bl.Phone),
		Email:                 utils.StringToNullString(bl.Email),
		UserComments:          utils.StringToNullString(bl.UserComments),
		AdminComments:         utils.StringToNullString(bl.AdminComments),
		Currency:              utils.StringToNullString(bl.Currency),
		PaymentMethod:         utils.StringToNullString(bl.PaymentMethod),
		PaymentMethodCOD:      utils.StringToNullString(bl.PaymentMethodCOD),
		PaymentDone:           bl.PaymentDone,
		DeliveryMethodID:      utils.Int64ToNullInt64(bl.DeliveryMethodID),
		DeliveryMethod:        utils.StringToNullString(bl.DeliveryMethod),
		DeliveryPrice:         bl.DeliveryPrice,
		DeliveryPackageModule: utils.StringToNullString(bl.DeliveryPackageModule),
		DeliveryPackageNr:     utils.StringToNullString(bl.DeliveryPackageNr),
		DeliveryFullname:      utils.StringToNullString(bl.DeliveryFullname),
		DeliveryCompany:       utils.StringToNullString(bl.DeliveryCompany),
		DeliveryAddress:       utils.StringToNullString(bl.DeliveryAddress),
		DeliveryCity:          utils.StringToNullString(bl.DeliveryCity),
		DeliveryState:         utils.StringToNullString(bl.DeliveryState),
		DeliveryPostcode:      utils.StringToNullString(bl.DeliveryPostcode),
		DeliveryCountryCode:   utils.StringToNullString(bl.DeliveryCountryCode),
		DeliveryCountry:       utils.StringToNullString(bl.DeliveryCountry),
		DeliveryPointID:       utils.StringToNullString(bl.DeliveryPointID),
		DeliveryPointName:     utils.StringToNullString(bl.DeliveryPointName),
		DeliveryPointAddress:  utils.StringToNullString(bl.DeliveryPointAddress),
		DeliveryPointPostcode: utils.StringToNullString(bl.DeliveryPointPostcode),
		DeliveryPointCity:     utils.StringToNullString(bl.DeliveryPointCity),
		InvoiceFullname:       utils.StringToNullString(bl.InvoiceFullname),
		InvoiceCompany:        utils.StringToNullString(bl.InvoiceCompany),
		InvoiceNip:            utils.StringToNullString(bl.InvoiceNip),
		InvoiceAddress:        utils.StringToNullString(bl.InvoiceAddress),
		InvoiceCity:           utils.StringToNullString(bl.InvoiceCity),
		InvoiceState:          utils.StringToNullString(bl.InvoiceState),
		InvoicePostcode:       utils.StringToNullString(bl.InvoicePostcode),
		InvoiceCountryCode:    utils.StringToNullString(bl.InvoiceCountryCode),
		InvoiceCountry:        utils.StringToNullString(bl.InvoiceCountry),
		WantInvoice:           utils.StringToNullString(bl.WantInvoice),
		ExtraField1:           utils.StringToNullString(bl.ExtraField1),
		ExtraField2:           utils.StringToNullString(bl.ExtraField2),
		OrderPage:             utils.StringToNullString(bl.OrderPage),
		PickState:             bl.PickState,
		PackState:             bl.PackState,
		Star:                  bl.Star,
		CRMClientID:           bl.CRMClientID,
		Priority:              "p3",
		OrderStatus:           "received",
	}
}

func (s *OrderService) convertToOrderProducts(bl *models.BaseLinkerOrder) []models.OrderProduct {
	products := make([]models.OrderProduct, 0, len(bl.Products))

	for _, p := range bl.Products {
		isDiscount := utils.IsDiscountLine(p.SKU, p.Name, p.PriceBrutto)
		product := models.OrderProduct{
			OrderProductID: p.OrderProductID,
			AmazonOrderID:  bl.ExternalOrderID,
			Storage:        utils.StringToNullString(p.Storage),
			StorageID:      utils.Int64ToNullInt64(p.StorageID),
			ProductID:      utils.StringToNullString(p.ProductID),
			VariantID:      utils.StringToNullString(p.VariantID),
			Name:           utils.StringToNullString(p.Name),
			Attributes:     utils.StringToNullString(p.Attributes),
			SKU:            utils.StringToNullString(p.SKU),
			EAN:            utils.StringToNullString(p.EAN),
			Location:       utils.StringToNullString(p.Location),
			WarehouseID:    utils.Int64ToNullInt64(p.WarehouseID),
			AuctionID:      utils.StringToNullString(p.AuctionID),
			PriceBrutto:    utils.Float64ToNullFloat64(p.PriceBrutto),
			TaxRate:        utils.Float64ToNullFloat64(p.TaxRate),
			Quantity:       utils.Float64ToNullFloat64(p.Quantity),
			Weight:         utils.Float64ToNullFloat64(p.Weight),
			BundleID:       utils.Int64ToNullInt64(p.BundleID),
			IsDiscountLine: isDiscount,
		}

		if !isDiscount && p.SKU != "" {
			if skuData, found := utils.GetSKUMapper().GetBySKU(p.SKU); found {
				product.DefaultWidthInInches = utils.Float64ToNullFloat64(skuData.WidthInInches)
				product.DefaultLengthInInches = utils.Float64ToNullFloat64(skuData.LengthInInches)
				product.DefaultWidthInMM = utils.Float64ToNullFloat64(skuData.WidthInMM)
				product.DefaultLengthInMM = utils.Float64ToNullFloat64(skuData.LengthInMM)
				product.Thickness = utils.StringToNullString(skuData.Thickness)
				product.IsRound = skuData.IsRound
				log.Printf("✓ Auto-populated dimensions for order %s product %d from SKU %s (thickness=%s is_round=%v)",
					bl.ExternalOrderID, p.OrderProductID, p.SKU, skuData.Thickness, skuData.IsRound)
			}
		}

		products = append(products, product)
	}

	return products
}

// ListOrders returns paginated orders with filters
func (s *OrderService) ListOrders(ctx context.Context, filters map[string]interface{}, page, limit int) ([]models.AmazonOrder, int, error) {
	return s.repo.ListOrders(ctx, filters, page, limit)
}

// GetOrderByID returns a single order
func (s *OrderService) GetOrderByID(ctx context.Context, amazonOrderID string) (*models.AmazonOrder, error) {
	return s.repo.GetOrderByID(ctx, amazonOrderID)
}

func (s *OrderService) GetDashboardAnalytics(ctx context.Context, chartWindowDays, missingRiskWindowDays int, dateFrom, dateTo *time.Time) (*models.DashboardAnalytics, error) {
	if chartWindowDays <= 0 {
		chartWindowDays = 30
	}
	if missingRiskWindowDays <= 0 {
		missingRiskWindowDays = 7
	}
	return s.repo.GetDashboardAnalytics(ctx, chartWindowDays, missingRiskWindowDays, dateFrom, dateTo)
}

func (s *OrderService) GetExecutiveDashboard(ctx context.Context, filters models.ExecutiveDashboardFilters) (*models.ExecutiveDashboardResponse, error) {
	return s.repo.GetExecutiveDashboard(ctx, filters)
}

func (s *OrderService) GetReturnsDashboard(ctx context.Context, filters models.ReturnsDashboardFilters) (*models.ReturnsDashboardResponse, error) {
	return s.repo.GetReturnsDashboard(ctx, filters)
}

func (s *OrderService) GetSafetyClaimsDashboard(ctx context.Context, filters models.SafetyClaimsDashboardFilters) (*models.SafetyClaimsDashboardResponse, error) {
	return s.repo.GetSafetyClaimsDashboard(ctx, filters)
}

func (s *OrderService) GetRepeatCustomers(ctx context.Context, returnsOnly bool, confirmedDateFrom, confirmedDateTo *time.Time) (*models.RepeatCustomerResponse, error) {
	return s.repo.GetRepeatCustomers(ctx, returnsOnly, confirmedDateFrom, confirmedDateTo)
}

// UpdateManualFields updates manual business fields
func (s *OrderService) UpdateManualFields(ctx context.Context, amazonOrderID string, req *models.UpdateManualFieldsRequest, actor string) (*models.AmazonOrder, error) {
	if req.Priority != nil {
		if !isValidPriority(*req.Priority) {
			return nil, fmt.Errorf("invalid priority: must be one of: p1, p2, p3, p4")
		}
	}

	if req.OrderStatus != nil {
		if !isValidWorkflowOrderStatus(*req.OrderStatus) {
			return nil, fmt.Errorf("invalid order_status: must be one of: received, manufactured, cancelled, returned")
		}
	}

	return s.repo.UpdateManualFields(ctx, amazonOrderID, req, actor)
}

// UpdateProductManualFields updates product-level manual business fields.
func (s *OrderService) UpdateProductManualFields(ctx context.Context, amazonOrderID string, orderProductID int64, req *models.UpdateProductManualFieldsRequest, actor string) (*models.AmazonOrder, error) {
	return s.repo.UpdateProductManualFields(ctx, amazonOrderID, orderProductID, req, actor)
}

func isValidPriority(priority string) bool {
	valid := []string{"p1", "p2", "p3", "p4"}
	for _, v := range valid {
		if priority == v {
			return true
		}
	}
	return false
}

func isValidWorkflowOrderStatus(status string) bool {
	valid := []string{"received", "manufactured", "cancelled", "returned"}
	for _, v := range valid {
		if status == v {
			return true
		}
	}
	return false
}
