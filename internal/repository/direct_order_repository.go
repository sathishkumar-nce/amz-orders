package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

type DirectOrderRepository struct {
	db *pgxpool.Pool
}

func NewDirectOrderRepository(db *pgxpool.Pool) *DirectOrderRepository {
	return &DirectOrderRepository{db: db}
}

func (r *DirectOrderRepository) GetSuggestedNextOrderID(ctx context.Context) (string, error) {
	var nextID int64
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTRING(order_id FROM 'DO-(\d+)$') AS BIGINT)), 0) + 1
		FROM direct_orders
		WHERE order_id ~ '^DO-[0-9]+$'
	`).Scan(&nextID); err != nil {
		return "", err
	}
	return formatDirectOrderID(nextID), nil
}

func (r *DirectOrderRepository) Create(ctx context.Context, order *models.CreateDirectOrderRequest) (*models.DirectOrder, error) {
	log.Printf("📦 Repository create direct order started (order_id=%s items=%d)", order.OrderID, len(order.Items))
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	explicitID := int64(0)
	if strings.TrimSpace(order.OrderID) == "" {
		nextOrderID, err := r.GetSuggestedNextOrderID(ctx)
		if err != nil {
			return nil, err
		}
		order.OrderID = nextOrderID
	} else if isSystemDirectOrderID(order.OrderID) {
		exists, err := r.orderIDExists(ctx, order.OrderID)
		if err != nil {
			return nil, err
		}
		if exists {
			nextOrderID, err := r.GetSuggestedNextOrderID(ctx)
			if err != nil {
				return nil, err
			}
			order.OrderID = nextOrderID
		}
	}

	if err := tx.QueryRow(ctx, `SELECT nextval(pg_get_serial_sequence('direct_orders', 'id'))`).Scan(&explicitID); err != nil {
		return nil, err
	}

	args := []interface{}{
		nullString(order.Source),
		order.OrderID,
		order.OrderStatus,
		defaultString(order.CourierType, "manual"),
		order.CourierName,
		order.AWB,
		order.PaymentStatus,
		order.Amount,
		order.AdvanceAmount,
		order.CODAmount,
		order.CustomerName,
		order.Address,
		order.Pincode,
		order.Mobile,
		order.AlternateMobile,
		order.Email,
		order.AlternateEmail,
		order.Remarks,
		order.Priority,
		order.Issues,
		order.UpdatedBy,
		order.City,
		order.State,
		defaultString(order.Country, "India"),
		order.Landmark,
		defaultString(order.ShipmentType, "forward"),
		order.ServiceType,
		order.PickupLocation,
		defaultInt(order.PackageCount, 1),
		order.TotalWeight,
		order.LengthCM,
		order.WidthCM,
		order.HeightCM,
		nullableDate(order.InvoiceDate),
	}

	query := `
		INSERT INTO direct_orders (
			source, order_id, order_status, courier_type, courier_name, awb,
			payment_status, amount, advance_amount, cod_amount,
			customer_name, address, pincode, mobile, alternate_mobile,
			email, alternate_email, remarks, priority, issues, updated_by,
			city, state, country, landmark, shipment_type, service_type,
			pickup_location, package_count, total_weight, length_cm, width_cm, height_cm,
			invoice_date
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26, $27,
			$28, $29, $30, $31, $32, $33, $34
		)
	`
	if explicitID > 0 {
		query = `
			INSERT INTO direct_orders (
				id, source, order_id, order_status, courier_type, courier_name, awb,
				payment_status, amount, advance_amount, cod_amount,
				customer_name, address, pincode, mobile, alternate_mobile,
				email, alternate_email, remarks, priority, issues, updated_by,
				city, state, country, landmark, shipment_type, service_type,
				pickup_location, package_count, total_weight, length_cm, width_cm, height_cm,
				invoice_date
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13, $14, $15, $16,
				$17, $18, $19, $20, $21, $22,
				$23, $24, $25, $26, $27, $28,
				$29, $30, $31, $32, $33, $34, $35
			)
		`
		args = append([]interface{}{explicitID}, args...)
	}

	if _, err = tx.Exec(ctx, query, args...); err != nil {
		return nil, err
	}

	if err := r.replaceItems(ctx, tx, order.OrderID, order.Items, order.UpdatedBy); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	log.Printf("✅ Repository create direct order committed (order_id=%s)", order.OrderID)
	return r.GetByOrderID(ctx, order.OrderID)
}

func (r *DirectOrderRepository) GetByOrderID(ctx context.Context, orderID string) (*models.DirectOrder, error) {
	log.Printf("🔎 Repository get direct order by order_id=%s", orderID)
	query := `
		SELECT id, created_at, updated_at, deleted_at, source, order_id, order_status,
			courier_type, courier_name, awb, payment_status, amount, advance_amount, cod_amount,
			customer_name, address, pincode, mobile, alternate_mobile, email, alternate_email,
			remarks, priority, issues, updated_by, city, state, country, landmark, shipment_type,
			service_type, pickup_location, package_count, total_weight, length_cm, width_cm,
			height_cm, invoice_date, courier_order_id, courier_status,
			manifested_at, pickup_requested_at, courier_payload
		FROM direct_orders
		WHERE order_id = $1 AND deleted_at IS NULL
	`

	var order models.DirectOrder
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&order.ID, &order.CreatedAt, &order.UpdatedAt, &order.DeletedAt, &order.Source, &order.OrderID, &order.OrderStatus,
		&order.CourierType, &order.CourierName, &order.AWB, &order.PaymentStatus, &order.Amount, &order.AdvanceAmount, &order.CODAmount,
		&order.CustomerName, &order.Address, &order.Pincode, &order.Mobile, &order.AlternateMobile, &order.Email, &order.AlternateEmail,
		&order.Remarks, &order.Priority, &order.Issues, &order.UpdatedBy, &order.City, &order.State, &order.Country, &order.Landmark, &order.ShipmentType,
		&order.ServiceType, &order.PickupLocation, &order.PackageCount, &order.TotalWeight, &order.LengthCM, &order.WidthCM,
		&order.HeightCM, &order.InvoiceDate, &order.CourierOrderID, &order.CourierStatus,
		&order.ManifestedAt, &order.PickupRequestedAt, &order.CourierPayload,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	items, err := r.getItemsByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	order.Items = items
	log.Printf("✅ Repository get direct order completed (order_id=%s items=%d)", orderID, len(order.Items))
	return &order, nil
}

func (r *DirectOrderRepository) Update(ctx context.Context, orderID string, req *models.UpdateDirectOrderRequest) (*models.DirectOrder, error) {
	log.Printf("🛠️  Repository update direct order started (order_id=%s)", orderID)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var setParts []string
	var args []interface{}
	argIndex := 1

	add := func(column string, value interface{}) {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, argIndex))
		args = append(args, value)
		argIndex++
	}

	if req.Source != nil {
		add("source", nullString(*req.Source))
	}
	if req.OrderStatus != nil {
		add("order_status", *req.OrderStatus)
	}
	if req.CourierType != nil {
		add("courier_type", req.CourierType)
	}
	if req.CourierName != nil {
		add("courier_name", req.CourierName)
	}
	if req.AWB != nil {
		add("awb", req.AWB)
	}
	if req.PaymentStatus != nil {
		add("payment_status", *req.PaymentStatus)
	}
	if req.Amount != nil {
		add("amount", req.Amount)
	}
	if req.AdvanceAmount != nil {
		add("advance_amount", req.AdvanceAmount)
	}
	if req.CODAmount != nil {
		add("cod_amount", req.CODAmount)
	}
	if req.CustomerName != nil {
		add("customer_name", req.CustomerName)
	}
	if req.Address != nil {
		add("address", req.Address)
	}
	if req.Pincode != nil {
		add("pincode", req.Pincode)
	}
	if req.Mobile != nil {
		add("mobile", req.Mobile)
	}
	if req.AlternateMobile != nil {
		add("alternate_mobile", req.AlternateMobile)
	}
	if req.Email != nil {
		add("email", req.Email)
	}
	if req.AlternateEmail != nil {
		add("alternate_email", req.AlternateEmail)
	}
	if req.Remarks != nil {
		add("remarks", req.Remarks)
	}
	if req.Priority != nil {
		add("priority", *req.Priority)
	}
	if req.Issues != nil {
		add("issues", req.Issues)
	}
	if req.City != nil {
		add("city", req.City)
	}
	if req.State != nil {
		add("state", req.State)
	}
	if req.Country != nil {
		add("country", req.Country)
	}
	if req.Landmark != nil {
		add("landmark", req.Landmark)
	}
	if req.ShipmentType != nil {
		add("shipment_type", req.ShipmentType)
	}
	if req.ServiceType != nil {
		add("service_type", req.ServiceType)
	}
	if req.PickupLocation != nil {
		add("pickup_location", req.PickupLocation)
	}
	if req.PackageCount != nil {
		add("package_count", req.PackageCount)
	}
	if req.TotalWeight != nil {
		add("total_weight", req.TotalWeight)
	}
	if req.LengthCM != nil {
		add("length_cm", req.LengthCM)
	}
	if req.WidthCM != nil {
		add("width_cm", req.WidthCM)
	}
	if req.HeightCM != nil {
		add("height_cm", req.HeightCM)
	}
	if req.InvoiceDate != nil {
		add("invoice_date", nullableDate(req.InvoiceDate))
	}
	if req.UpdatedBy != nil {
		add("updated_by", req.UpdatedBy)
	}

	if len(setParts) > 0 {
		args = append(args, orderID)
		query := fmt.Sprintf(
			"UPDATE direct_orders SET %s WHERE order_id = $%d AND deleted_at IS NULL",
			strings.Join(setParts, ", "),
			argIndex,
		)
		result, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		if result.RowsAffected() == 0 {
			return nil, sql.ErrNoRows
		}
	}

	if req.Items != nil {
		if err := r.replaceItems(ctx, tx, orderID, *req.Items, req.UpdatedBy); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	log.Printf("✅ Repository update direct order committed (order_id=%s)", orderID)
	return r.GetByOrderID(ctx, orderID)
}

func (r *DirectOrderRepository) SoftDelete(ctx context.Context, orderID string) error {
	log.Printf("🗑️  Repository soft delete started (order_id=%s)", orderID)
	result, err := r.db.Exec(ctx, `UPDATE direct_orders SET deleted_at = $1 WHERE order_id = $2 AND deleted_at IS NULL`, time.Now(), orderID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	log.Printf("✅ Repository soft delete completed (order_id=%s)", orderID)
	return nil
}

func (r *DirectOrderRepository) List(ctx context.Context, filters models.DirectOrderFilters) ([]models.DirectOrder, int, error) {
	log.Printf("📋 Repository list direct orders started (page=%d limit=%d)", filters.Page, filters.Limit)
	whereClause, args, argIndex := buildDirectOrderWhere(filters)

	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM direct_orders WHERE %s", whereClause), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 {
		filters.Limit = 20
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}
	offset := (filters.Page - 1) * filters.Limit
	args = append(args, filters.Limit, offset)

	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, deleted_at, source, order_id, order_status,
			courier_type, courier_name, awb, payment_status, amount, advance_amount, cod_amount,
			customer_name, address, pincode, mobile, alternate_mobile, email, alternate_email,
			remarks, priority, issues, updated_by, city, state, country, landmark, shipment_type,
			service_type, pickup_location, package_count, total_weight, length_cm, width_cm,
			height_cm, invoice_date, courier_order_id, courier_status,
			manifested_at, pickup_requested_at, courier_payload
		FROM direct_orders
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders, err := r.scanOrders(ctx, rows)
	if err != nil {
		return nil, 0, err
	}

	log.Printf("✅ Repository list direct orders completed: returned=%d total=%d", len(orders), total)
	return orders, total, nil
}

func (r *DirectOrderRepository) ExportToCSV(ctx context.Context, filters models.DirectOrderFilters) ([]models.DirectOrder, error) {
	log.Printf("📤 Repository export direct orders to CSV started")
	whereClause, args, _ := buildDirectOrderWhere(filters)
	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, deleted_at, source, order_id, order_status,
			courier_type, courier_name, awb, payment_status, amount, advance_amount, cod_amount,
			customer_name, address, pincode, mobile, alternate_mobile, email, alternate_email,
			remarks, priority, issues, updated_by, city, state, country, landmark, shipment_type,
			service_type, pickup_location, package_count, total_weight, length_cm, width_cm,
			height_cm, invoice_date, courier_order_id, courier_status,
			manifested_at, pickup_requested_at, courier_payload
		FROM direct_orders
		WHERE %s
		ORDER BY created_at DESC
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders, err := r.scanOrders(ctx, rows)
	if err != nil {
		return nil, err
	}
	log.Printf("✅ Repository export direct orders to CSV completed: rows=%d", len(orders))
	return orders, nil
}

func (r *DirectOrderRepository) SaveDelhiveryForwardOrder(ctx context.Context, orderID string, result models.DelhiveryForwardOrderResult, updatedBy string) (*models.DirectOrder, error) {
	payload := any(nil)
	if len(result.CourierPayload) > 0 {
		payload = result.CourierPayload
	}

	query := `
		UPDATE direct_orders
		SET courier_type = $1,
			courier_name = $2,
			awb = $3,
			courier_order_id = $4,
			courier_status = $5,
			order_status = 'forwarded',
			manifested_at = $6,
			pickup_requested_at = $7,
			courier_payload = $8,
			updated_by = $9
		WHERE order_id = $10 AND deleted_at IS NULL
	`

	res, err := r.db.Exec(ctx, query,
		"delhivery",
		result.CourierName,
		result.AWB,
		result.CourierOrderID,
		result.CourierStatus,
		result.ManifestedAt,
		result.PickupRequestedAt,
		payload,
		updatedBy,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	if res.RowsAffected() == 0 {
		return nil, sql.ErrNoRows
	}
	return r.GetByOrderID(ctx, orderID)
}

func (r *DirectOrderRepository) scanOrders(ctx context.Context, rows pgx.Rows) ([]models.DirectOrder, error) {
	var orders []models.DirectOrder
	for rows.Next() {
		var order models.DirectOrder
		if err := rows.Scan(
			&order.ID, &order.CreatedAt, &order.UpdatedAt, &order.DeletedAt, &order.Source, &order.OrderID, &order.OrderStatus,
			&order.CourierType, &order.CourierName, &order.AWB, &order.PaymentStatus, &order.Amount, &order.AdvanceAmount, &order.CODAmount,
			&order.CustomerName, &order.Address, &order.Pincode, &order.Mobile, &order.AlternateMobile, &order.Email, &order.AlternateEmail,
			&order.Remarks, &order.Priority, &order.Issues, &order.UpdatedBy, &order.City, &order.State, &order.Country, &order.Landmark, &order.ShipmentType,
			&order.ServiceType, &order.PickupLocation, &order.PackageCount, &order.TotalWeight, &order.LengthCM, &order.WidthCM,
			&order.HeightCM, &order.InvoiceDate, &order.CourierOrderID, &order.CourierStatus,
			&order.ManifestedAt, &order.PickupRequestedAt, &order.CourierPayload,
		); err != nil {
			return nil, err
		}

		items, err := r.getItemsByOrderID(ctx, order.OrderID)
		if err != nil {
			return nil, err
		}
		order.Items = items
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *DirectOrderRepository) getItemsByOrderID(ctx context.Context, orderID string) ([]models.DirectOrderItem, error) {
	query := `
		SELECT id, created_at, updated_at, order_id, item, quantity, dimension, thickness, weight, amount, remark, updated_by, sku, hsn, unit_price, tax_rate,
			customer_width_in_inches, customer_length_in_inches, customer_width_in_mm, customer_length_in_mm, corner_radius_and_notes
		FROM direct_order_items
		WHERE order_id = $1
		ORDER BY id ASC
	`

	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.DirectOrderItem
	for rows.Next() {
		var item models.DirectOrderItem
		if err := rows.Scan(
			&item.ID, &item.CreatedAt, &item.UpdatedAt, &item.OrderID, &item.Item, &item.Quantity, &item.Dimension,
			&item.Thickness, &item.Weight, &item.Amount, &item.Remark, &item.UpdatedBy, &item.SKU, &item.HSN, &item.UnitPrice, &item.TaxRate,
			&item.CustomerWidthInches, &item.CustomerLengthInches, &item.CustomerWidthMM, &item.CustomerLengthMM, &item.CornerRadiusAndNotes,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *DirectOrderRepository) replaceItems(ctx context.Context, tx pgx.Tx, orderID string, items []models.CreateDirectOrderItemRequest, updatedBy *string) error {
	if _, err := tx.Exec(ctx, "DELETE FROM direct_order_items WHERE order_id = $1", orderID); err != nil {
		return err
	}

	itemQuery := `
		INSERT INTO direct_order_items (
			order_id, item, quantity, dimension, thickness, weight, amount, remark, updated_by, sku, hsn, unit_price, tax_rate,
			customer_width_in_inches, customer_length_in_inches, customer_width_in_mm, customer_length_in_mm, corner_radius_and_notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	for _, item := range items {
		if _, err := tx.Exec(ctx, itemQuery,
			orderID,
			item.Item,
			defaultItemQuantity(item.Quantity),
			item.Dimension,
			item.Thickness,
			item.Weight,
			item.Amount,
			item.Remark,
			updatedBy,
			item.SKU,
			item.HSN,
			item.UnitPrice,
			item.TaxRate,
			item.CustomerWidthInches,
			item.CustomerLengthInches,
			item.CustomerWidthMM,
			item.CustomerLengthMM,
			item.CornerRadiusAndNotes,
		); err != nil {
			return err
		}
	}
	return nil
}

func buildDirectOrderWhere(filters models.DirectOrderFilters) (string, []interface{}, int) {
	var whereClauses []string
	var args []interface{}
	argIndex := 1

	whereClauses = append(whereClauses, "deleted_at IS NULL")

	if filters.OrderID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("order_id ILIKE $%d", argIndex))
		args = append(args, "%"+filters.OrderID+"%")
		argIndex++
	}
	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(`(
			order_id ILIKE $%d OR
			COALESCE(awb, '') ILIKE $%d OR
			COALESCE(customer_name, '') ILIKE $%d OR
			COALESCE(mobile, '') ILIKE $%d OR
			COALESCE(pincode, '') ILIKE $%d
		)`, argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, "%"+filters.Search+"%")
		argIndex++
	}
	if filters.AWB != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("awb ILIKE $%d", argIndex))
		args = append(args, "%"+filters.AWB+"%")
		argIndex++
	}
	if filters.OrderStatus != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("order_status = $%d", argIndex))
		args = append(args, filters.OrderStatus)
		argIndex++
	}
	if filters.PaymentStatus != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("payment_status = $%d", argIndex))
		args = append(args, filters.PaymentStatus)
		argIndex++
	}
	if filters.Priority != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("priority = $%d", argIndex))
		args = append(args, filters.Priority)
		argIndex++
	}
	if filters.Source != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source = $%d", argIndex))
		args = append(args, filters.Source)
		argIndex++
	}
	if filters.Mobile != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("mobile ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Mobile+"%")
		argIndex++
	}
	if filters.CustomerName != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("customer_name ILIKE $%d", argIndex))
		args = append(args, "%"+filters.CustomerName+"%")
		argIndex++
	}
	if filters.Item != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM direct_order_items doi WHERE doi.order_id = direct_orders.order_id AND doi.item ILIKE $%d)", argIndex))
		args = append(args, "%"+filters.Item+"%")
		argIndex++
	}
	if filters.Quantity > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM direct_order_items doi WHERE doi.order_id = direct_orders.order_id AND doi.quantity = $%d)", argIndex))
		args = append(args, filters.Quantity)
		argIndex++
	}
	if filters.Pincode != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("pincode = $%d", argIndex))
		args = append(args, filters.Pincode)
		argIndex++
	}
	if filters.DateExact != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, filters.DateExact+" 00:00:00")
		argIndex++
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, filters.DateExact+" 23:59:59")
		argIndex++
	}
	if filters.DateFrom != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, filters.DateFrom)
		argIndex++
	}
	if filters.DateTo != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, filters.DateTo+" 23:59:59")
		argIndex++
	}

	return strings.Join(whereClauses, " AND "), args, argIndex
}

func buildDirectOrderDashboardConditions(filters models.ExecutiveDashboardFilters, orderAlias string, argPos int) ([]string, []interface{}, int) {
	conditions := []string{fmt.Sprintf("%s.deleted_at IS NULL", orderAlias)}
	args := make([]interface{}, 0, 6)

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.invoice_date, %s.created_at) >= $%d", orderAlias, orderAlias, argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}
	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.invoice_date, %s.created_at) < $%d", orderAlias, orderAlias, argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}
	if strings.TrimSpace(filters.State) != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(NULLIF(BTRIM(%s.state), ''), '') ILIKE $%d", orderAlias, argPos))
		args = append(args, wildcardPattern(filters.State))
		argPos++
	}
	if strings.TrimSpace(filters.City) != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(NULLIF(BTRIM(%s.city), ''), '') ILIKE $%d", orderAlias, argPos))
		args = append(args, wildcardPattern(filters.City))
		argPos++
	}
	if strings.TrimSpace(filters.OrderStatus) != "" {
		conditions = append(conditions, fmt.Sprintf("%s.order_status = $%d", orderAlias, argPos))
		args = append(args, filters.OrderStatus)
		argPos++
	}
	if strings.TrimSpace(filters.Thickness) != "" {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM direct_order_items doi_filter
			WHERE doi_filter.order_id = %s.order_id
			  AND COALESCE(NULLIF(BTRIM(doi_filter.thickness), ''), '') ILIKE $%d
		)`, orderAlias, argPos))
		args = append(args, wildcardPattern(filters.Thickness))
		argPos++
	}

	return conditions, args, argPos
}

func (r *DirectOrderRepository) GetExecutiveDashboard(ctx context.Context, filters models.ExecutiveDashboardFilters) (*models.DirectOrderExecutiveDashboardResponse, error) {
	if filters.FromDate == nil || filters.ToDate == nil {
		return nil, fmt.Errorf("direct order executive dashboard requires from_date and to_date")
	}

	location := analyticsISTLocation()
	response := &models.DirectOrderExecutiveDashboardResponse{
		GeneratedAt:       time.Now(),
		DateRange:         filters.DateRange,
		RangeStart:        filters.FromDate.In(location).Format("2006-01-02"),
		RangeEnd:          filters.ToDate.Add(-time.Second).In(location).Format("2006-01-02"),
		AvailableStates:   []string{},
		AvailableCities:   []string{},
		OrdersTrend:       []models.AnalyticsTimePoint{},
		OtherIssuesTrend:  []models.AnalyticsTimePoint{},
		OrdersByState:     []models.AnalyticsChartSlice{},
		OrdersByCity:      []models.AnalyticsChartSlice{},
		OrdersByThickness: []models.AnalyticsChartSlice{},
	}

	baseConditions, baseArgs, _ := buildDirectOrderDashboardConditions(filters, "o", 1)
	baseWhere := buildWhereClause(baseConditions)

	queryCount := func(query string, args ...interface{}) (int, error) {
		var count int
		if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	}

	var err error
	if response.Summary.TotalOrders, err = queryCount(
		fmt.Sprintf("SELECT COUNT(DISTINCT o.order_id) FROM direct_orders o %s", baseWhere),
		baseArgs...,
	); err != nil {
		return nil, fmt.Errorf("count direct total orders: %w", err)
	}

	nextArg := len(baseArgs) + 1
	statusCount := func(status string) (int, error) {
		args := append(append([]interface{}{}, baseArgs...), status)
		return queryCount(
			fmt.Sprintf("SELECT COUNT(DISTINCT o.order_id) FROM direct_orders o %s", appendWhereCondition(baseWhere, fmt.Sprintf("o.order_status = $%d", nextArg))),
			args...,
		)
	}

	if response.Summary.ManufacturedOrders, err = statusCount("manufactured"); err != nil {
		return nil, fmt.Errorf("count manufactured direct orders: %w", err)
	}
	if response.Summary.OtherIssuesOrders, err = statusCount("other-issues"); err != nil {
		return nil, fmt.Errorf("count other-issues direct orders: %w", err)
	}
	if response.Summary.CancelledOrders, err = statusCount("cancelled"); err != nil {
		return nil, fmt.Errorf("count cancelled direct orders: %w", err)
	}
	if response.Summary.OnHoldOrders, err = statusCount("on-hold"); err != nil {
		return nil, fmt.Errorf("count on-hold direct orders: %w", err)
	}

	days := int(filters.ToDate.Sub(*filters.FromDate).Hours() / 24)
	if days <= 0 {
		days = 1
	}
	if response.Summary.TotalOrders > 0 {
		response.Summary.AveragePerDay = float64(response.Summary.TotalOrders) / float64(days)
	}

	orderTrendCounts := make(map[string]float64)
	orderTrendRows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT TO_CHAR(COALESCE(o.invoice_date, o.created_at)::date, 'YYYY-MM-DD') AS day_key,
		       COUNT(DISTINCT o.order_id) AS count
		FROM direct_orders o
		%s
		GROUP BY 1
		ORDER BY 1
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query direct orders trend: %w", err)
	}
	for orderTrendRows.Next() {
		var dayKey string
		var count float64
		if err := orderTrendRows.Scan(&dayKey, &count); err != nil {
			orderTrendRows.Close()
			return nil, fmt.Errorf("scan direct orders trend: %w", err)
		}
		orderTrendCounts[dayKey] = count
	}
	if err := orderTrendRows.Err(); err != nil {
		orderTrendRows.Close()
		return nil, fmt.Errorf("iterate direct orders trend: %w", err)
	}
	orderTrendRows.Close()

	otherIssueArgs := append(append([]interface{}{}, baseArgs...), "other-issues")
	otherIssueWhere := appendWhereCondition(baseWhere, fmt.Sprintf("o.order_status = $%d", nextArg))
	otherIssueTrendCounts := make(map[string]float64)
	otherIssueRows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT TO_CHAR(COALESCE(o.updated_at, o.invoice_date, o.created_at)::date, 'YYYY-MM-DD') AS day_key,
		       COUNT(DISTINCT o.order_id) AS count
		FROM direct_orders o
		%s
		GROUP BY 1
		ORDER BY 1
	`, otherIssueWhere), otherIssueArgs...)
	if err != nil {
		return nil, fmt.Errorf("query direct other issues trend: %w", err)
	}
	for otherIssueRows.Next() {
		var dayKey string
		var count float64
		if err := otherIssueRows.Scan(&dayKey, &count); err != nil {
			otherIssueRows.Close()
			return nil, fmt.Errorf("scan direct other issues trend: %w", err)
		}
		otherIssueTrendCounts[dayKey] = count
	}
	if err := otherIssueRows.Err(); err != nil {
		otherIssueRows.Close()
		return nil, fmt.Errorf("iterate direct other issues trend: %w", err)
	}
	otherIssueRows.Close()

	dailyPeriods := buildDailyAnalyticsPeriods(filters.FromDate.In(location), filters.ToDate.In(location))
	for _, period := range dailyPeriods {
		dayKey := period.Start.Format("2006-01-02")
		response.OrdersTrend = append(response.OrdersTrend, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: orderTrendCounts[dayKey],
		})
		response.OtherIssuesTrend = append(response.OtherIssuesTrend, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: otherIssueTrendCounts[dayKey],
		})
	}

	stateRows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(BTRIM(o.state), ''), 'Not available') AS label,
		       COUNT(DISTINCT o.order_id) AS count
		FROM direct_orders o
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query direct orders by state: %w", err)
	}
	for stateRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := stateRows.Scan(&slice.Label, &slice.Count); err != nil {
			stateRows.Close()
			return nil, fmt.Errorf("scan direct orders by state: %w", err)
		}
		response.OrdersByState = append(response.OrdersByState, slice)
	}
	if err := stateRows.Err(); err != nil {
		stateRows.Close()
		return nil, fmt.Errorf("iterate direct orders by state: %w", err)
	}
	stateRows.Close()

	cityRows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(BTRIM(o.city), ''), 'Not available') AS label,
		       COUNT(DISTINCT o.order_id) AS count
		FROM direct_orders o
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query direct orders by city: %w", err)
	}
	for cityRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := cityRows.Scan(&slice.Label, &slice.Count); err != nil {
			cityRows.Close()
			return nil, fmt.Errorf("scan direct orders by city: %w", err)
		}
		response.OrdersByCity = append(response.OrdersByCity, slice)
	}
	if err := cityRows.Err(); err != nil {
		cityRows.Close()
		return nil, fmt.Errorf("iterate direct orders by city: %w", err)
	}
	cityRows.Close()

	thicknessRows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s AS label,
		       COUNT(DISTINCT o.order_id) AS count
		FROM direct_orders o
		INNER JOIN direct_order_items doi ON doi.order_id = o.order_id
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, normalizedThicknessCase("doi.thickness"), baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query direct orders by thickness: %w", err)
	}
	for thicknessRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := thicknessRows.Scan(&slice.Label, &slice.Count); err != nil {
			thicknessRows.Close()
			return nil, fmt.Errorf("scan direct orders by thickness: %w", err)
		}
		response.OrdersByThickness = append(response.OrdersByThickness, slice)
	}
	if err := thicknessRows.Err(); err != nil {
		thicknessRows.Close()
		return nil, fmt.Errorf("iterate direct orders by thickness: %w", err)
	}
	thicknessRows.Close()

	stateConditions, stateArgs, _ := buildDirectOrderDashboardConditions(filters, "o", 1)
	stateWhere := appendWhereCondition(buildWhereClause(stateConditions), "COALESCE(NULLIF(BTRIM(o.state), ''), '') <> ''")
	if response.AvailableStates, err = queryExecutiveStringList(ctx, r.db, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.state)
		FROM direct_orders o
		%s
		ORDER BY 1
	`, stateWhere), stateArgs...); err != nil {
		return nil, fmt.Errorf("query direct available states: %w", err)
	}

	cityConditions, cityArgs, _ := buildDirectOrderDashboardConditions(filters, "o", 1)
	cityWhere := appendWhereCondition(buildWhereClause(cityConditions), "COALESCE(NULLIF(BTRIM(o.city), ''), '') <> ''")
	if response.AvailableCities, err = queryExecutiveStringList(ctx, r.db, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.city)
		FROM direct_orders o
		%s
		ORDER BY 1
	`, cityWhere), cityArgs...); err != nil {
		return nil, fmt.Errorf("query direct available cities: %w", err)
	}

	response.OrdersByState = topChartSlices(response.OrdersByState, 10)
	response.OrdersByCity = topChartSlices(response.OrdersByCity, 10)
	response.OrdersByThickness = topChartSlices(response.OrdersByThickness, 10)

	return response, nil
}

func nullableDate(value *string) interface{} {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func defaultString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func defaultInt(value *int, fallback int) int {
	if value == nil || *value <= 0 {
		return fallback
	}
	return *value
}

func defaultItemQuantity(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func formatDirectOrderID(id int64) string {
	return fmt.Sprintf("DO-%06d", id)
}

func nullString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (r *DirectOrderRepository) orderIDExists(ctx context.Context, orderID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM direct_orders WHERE order_id = $1)`, orderID).Scan(&exists)
	return exists, err
}

var systemDirectOrderIDPattern = regexp.MustCompile(`^DO-\d+$`)

func isSystemDirectOrderID(orderID string) bool {
	return systemDirectOrderIDPattern.MatchString(strings.TrimSpace(orderID))
}
