package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/integrations/interakt"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

type OrderRepository struct {
	pool           *pgxpool.Pool
	interaktClient *interakt.Client
}

const analyticsISTOffsetSeconds = 5*60*60 + 30*60

type analyticsPeriod struct {
	Key   string
	Label string
	Start time.Time
	End   time.Time
}

type repeatCustomerRow struct {
	AmazonOrderID  string
	ConfirmedDate  sql.NullTime
	OrderStatus    string
	Customer       string
	Phone          string
	Address        string
	City           string
	State          string
	Postcode       string
	ProductSummary string
}

type orderProductListItem struct {
	OrderProductID                      int64    `json:"order_product_id"`
	AmazonOrderID                       string   `json:"amazon_order_id"`
	Name                                *string  `json:"name"`
	SKU                                 *string  `json:"sku"`
	AuctionID                           *string  `json:"auction_id"`
	PriceBrutto                         *float64 `json:"price_brutto"`
	Thickness                           *string  `json:"thickness"`
	Quantity                            *float64 `json:"quantity"`
	DefaultWidthInInches                *float64 `json:"default_width_in_inches"`
	DefaultLengthInInches               *float64 `json:"default_length_in_inches"`
	CustomerWidthInInches               *float64 `json:"customer_width_in_inches"`
	CustomerLengthInInches              *float64 `json:"customer_length_in_inches"`
	DefaultWidthInMM                    *float64 `json:"default_width_in_mm"`
	DefaultLengthInMM                   *float64 `json:"default_length_in_mm"`
	CustomerWidthInMM                   *float64 `json:"customer_width_in_mm"`
	CustomerLengthInMM                  *float64 `json:"customer_length_in_mm"`
	CornerRadiusAndNotes                *string  `json:"corner_radius_and_notes"`
	SafetyClaimed                       *bool    `json:"safety_claimed"`
	SafetyClaimedUpdatedAt              *string  `json:"safety_claimed_updated_at"`
	SafetyClaimIssues                   *string  `json:"safety_claim_issues"`
	ReturnInitiated                     *bool    `json:"return_initiated"`
	ReturnInitiatedUpdatedAt            *string  `json:"return_initiated_updated_at"`
	ReturnInitiatedReason               *string  `json:"return_initiated_reason"`
	ReturnInitiatedFollowupAction       *string  `json:"return_initiated_followup_action"`
	ReturnInitiatedCompromised          *bool    `json:"return_initiated_compromised"`
	ReturnInitiatedCompromisedReason    *string  `json:"return_initiated_compromised_reason"`
	ReturnInitiatedCompromisedUpdatedAt *string  `json:"return_initiated_compromised_updated_at"`
	OtherIssues                         *bool    `json:"other_issues"`
	OtherIssuesReason                   *string  `json:"other_issues_reason"`
	OtherIssueUpdatedAt                 *string  `json:"other_issue_updated_at"`
	IsRound                             bool     `json:"is_round"`
	IsDiscountLine                      bool     `json:"is_discount_line"`
	UpdatedBy                           *string  `json:"updated_by"`
}

type orderPriorityInput struct {
	AmazonOrderID string
	Priority      string
	SKUs          []string
}

type executiveRecentActivityRow struct {
	AmazonOrderID   string
	ConfirmedDate   sql.NullTime
	Customer        string
	State           string
	Thickness       string
	OrderStatus     string
	ReturnInitiated bool
	SafetyClaimed   bool
	OtherIssue      bool
	UpdatedAt       time.Time
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{
		pool:           pool,
		interaktClient: nil,
	}
}

func buildAnalyticsPeriods(now time.Time) []analyticsPeriod {
	localNow := now
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	threeDaysStart := todayStart.AddDate(0, 0, -2)
	sevenDaysStart := todayStart.AddDate(0, 0, -6)
	fifteenDaysStart := todayStart.AddDate(0, 0, -14)
	thirtyDaysStart := todayStart.AddDate(0, 0, -29)

	return []analyticsPeriod{
		{Key: "today", Label: "Today", Start: todayStart, End: localNow},
		{Key: "yesterday", Label: "Yesterday", Start: yesterdayStart, End: todayStart},
		{Key: "last_3_days", Label: "Last 3 Days", Start: threeDaysStart, End: localNow},
		{Key: "last_7_days", Label: "Last 7 Days", Start: sevenDaysStart, End: localNow},
		{Key: "last_15_days", Label: "Last 15 Days", Start: fifteenDaysStart, End: localNow},
		{Key: "last_30_days", Label: "Last 30 Days", Start: thirtyDaysStart, End: localNow},
	}
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func analyticsISTLocation() *time.Location {
	return time.FixedZone("IST", analyticsISTOffsetSeconds)
}

func currentBusinessDayStart(now time.Time, location *time.Location) time.Time {
	localNow := now.In(location)
	cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 13, 30, 0, 0, location)
	if localNow.Before(cutoff) {
		return cutoff.AddDate(0, 0, -1)
	}
	return cutoff
}

func buildMissingRiskPeriods(now time.Time, dayCount int, location *time.Location) []analyticsPeriod {
	if dayCount <= 0 {
		dayCount = 7
	}

	localNow := now.In(location)
	currentStart := currentBusinessDayStart(now, location)
	periods := make([]analyticsPeriod, 0, dayCount)

	for offset := dayCount - 1; offset >= 0; offset-- {
		start := currentStart.AddDate(0, 0, -offset)
		end := start.Add(24 * time.Hour)
		if end.After(localNow) {
			end = localNow
		}

		labelTime := start.Add(12 * time.Hour)
		label := labelTime.Format("02 Jan")
		if offset == 0 {
			label = "Today"
		} else if offset == 1 {
			label = "Yesterday"
		}

		periods = append(periods, analyticsPeriod{
			Key:   start.Format("2006-01-02-1504"),
			Label: label,
			Start: start,
			End:   end,
		})
	}

	return periods
}

func (r *OrderRepository) countDistinctOrdersBetween(ctx context.Context, query string, start, end time.Time) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, query, start, end).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func topChartSlices(rows []models.AnalyticsChartSlice, limit int) []models.AnalyticsChartSlice {
	if len(rows) <= limit || limit <= 0 {
		return rows
	}

	top := make([]models.AnalyticsChartSlice, 0, limit+1)
	top = append(top, rows[:limit]...)
	otherCount := 0
	for _, row := range rows[limit:] {
		otherCount += row.Count
	}
	if otherCount > 0 {
		top = append(top, models.AnalyticsChartSlice{
			Label: "Others",
			Count: otherCount,
		})
	}
	return top
}

func wildcardPattern(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	pattern := strings.NewReplacer("*", "%", "?", "_").Replace(trimmed)
	if strings.ContainsAny(pattern, "%_") {
		return pattern
	}
	return "%" + pattern + "%"
}

func addILikeCondition(whereConditions *[]string, args *[]interface{}, argPos *int, expression string, value string) {
	pattern := wildcardPattern(value)
	if pattern == "" {
		return
	}

	*whereConditions = append(*whereConditions, fmt.Sprintf("%s ILIKE $%d", expression, *argPos))
	*args = append(*args, pattern)
	*argPos = *argPos + 1
}

func (r *OrderRepository) queryChartSlices(ctx context.Context, query string, limit int, args ...interface{}) ([]models.AnalyticsChartSlice, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slices []models.AnalyticsChartSlice
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, err
		}
		slices = append(slices, models.AnalyticsChartSlice{
			Label: label,
			Count: count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topChartSlices(slices, limit), nil
}

func normalizePhoneForGrouping(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func normalizeAddressForGrouping(address, city, state, postcode string) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(address)),
		strings.ToLower(strings.TrimSpace(city)),
		strings.ToLower(strings.TrimSpace(state)),
		strings.ToLower(strings.TrimSpace(postcode)),
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "|")
}

func buildWhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(conditions, " AND ")
}

func appendWhereCondition(whereClause string, condition string) string {
	if strings.TrimSpace(condition) == "" {
		return whereClause
	}
	if whereClause == "" {
		return "WHERE " + condition
	}
	return whereClause + " AND " + condition
}

func buildExecutiveDashboardConditions(filters models.ExecutiveDashboardFilters, orderAlias string, includeState bool, includeCity bool, argPos int) ([]string, []interface{}, int) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) >= $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}

	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) < $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}

	if filters.OrderStatus != "" {
		conditions = append(conditions, fmt.Sprintf("%s.order_status = $%d", orderAlias, argPos))
		args = append(args, filters.OrderStatus)
		argPos++
	}

	if includeState && filters.State != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_state = $%d", orderAlias, argPos))
		args = append(args, filters.State)
		argPos++
	}

	if includeCity && filters.City != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_city = $%d", orderAlias, argPos))
		args = append(args, filters.City)
		argPos++
	}

	if filters.Thickness != "" {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM amazon_order_products p_filter
			WHERE p_filter.amazon_order_id = %s.amazon_order_id
			AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
			AND REPLACE(LOWER(COALESCE(p_filter.thickness, '')), ' ', '') = REPLACE(LOWER($%d::text), ' ', '')
		)`, orderAlias, argPos))
		args = append(args, filters.Thickness)
		argPos++
	}

	return conditions, args, argPos
}

func buildReturnsDashboardConditions(filters models.ReturnsDashboardFilters, orderAlias string, includeState bool, includeCity bool, argPos int) ([]string, []interface{}, int) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) >= $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}

	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) < $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}

	if filters.OrderStatus != "" {
		conditions = append(conditions, fmt.Sprintf("%s.order_status = $%d", orderAlias, argPos))
		args = append(args, filters.OrderStatus)
		argPos++
	}

	if includeState && filters.State != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_state = $%d", orderAlias, argPos))
		args = append(args, filters.State)
		argPos++
	}

	if includeCity && filters.City != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_city = $%d", orderAlias, argPos))
		args = append(args, filters.City)
		argPos++
	}

	if filters.Thickness != "" {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM amazon_order_products p_filter
			WHERE p_filter.amazon_order_id = %s.amazon_order_id
			AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
			AND REPLACE(LOWER(COALESCE(p_filter.thickness, '')), ' ', '') = REPLACE(LOWER($%d::text), ' ', '')
		)`, orderAlias, argPos))
		args = append(args, filters.Thickness)
		argPos++
	}

	if filters.ReturnInitiated != nil {
		if *filters.ReturnInitiated {
			conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.return_initiated, FALSE) = TRUE
			)`, orderAlias))
		} else {
			conditions = append(conditions, fmt.Sprintf(`NOT EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.return_initiated, FALSE) = TRUE
			)`, orderAlias))
		}
	}

	if filters.ReturnInitiatedCompromised != nil {
		if *filters.ReturnInitiatedCompromised {
			conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.return_initiated_compromised, FALSE) = TRUE
			)`, orderAlias))
		} else {
			conditions = append(conditions, fmt.Sprintf(`NOT EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.return_initiated_compromised, FALSE) = TRUE
			)`, orderAlias))
		}
	}

	return conditions, args, argPos
}

func buildSafetyClaimsDashboardConditions(filters models.SafetyClaimsDashboardFilters, orderAlias string, includeState bool, includeCity bool, argPos int) ([]string, []interface{}, int) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)

	if filters.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) >= $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.FromDate)
		argPos++
	}

	if filters.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.date_confirmed, %s.date_add) < $%d::timestamp", orderAlias, orderAlias, argPos))
		args = append(args, *filters.ToDate)
		argPos++
	}

	if filters.OrderStatus != "" {
		conditions = append(conditions, fmt.Sprintf("%s.order_status = $%d", orderAlias, argPos))
		args = append(args, filters.OrderStatus)
		argPos++
	}

	if includeState && filters.State != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_state = $%d", orderAlias, argPos))
		args = append(args, filters.State)
		argPos++
	}

	if includeCity && filters.City != "" {
		conditions = append(conditions, fmt.Sprintf("%s.delivery_city = $%d", orderAlias, argPos))
		args = append(args, filters.City)
		argPos++
	}

	if filters.Thickness != "" {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM amazon_order_products p_filter
			WHERE p_filter.amazon_order_id = %s.amazon_order_id
			AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
			AND REPLACE(LOWER(COALESCE(p_filter.thickness, '')), ' ', '') = REPLACE(LOWER($%d::text), ' ', '')
		)`, orderAlias, argPos))
		args = append(args, filters.Thickness)
		argPos++
	}

	if filters.SafetyClaimed != nil {
		if *filters.SafetyClaimed {
			conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.safety_claimed, FALSE) = TRUE
			)`, orderAlias))
		} else {
			conditions = append(conditions, fmt.Sprintf(`NOT EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = %s.amazon_order_id
				AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
				AND COALESCE(p_filter.safety_claimed, FALSE) = TRUE
			)`, orderAlias))
		}
	}

	return conditions, args, argPos
}

func normalizedThicknessCase(column string) string {
	return fmt.Sprintf(`CASE
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(%s), ''), '')), ' ', '') = '1mm' THEN '1 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(%s), ''), '')), ' ', '') = '1.5mm' THEN '1.5 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(%s), ''), '')), ' ', '') = '2mm' THEN '2 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(%s), ''), '')), ' ', '') = '3mm' THEN '3 mm'
		ELSE COALESCE(NULLIF(BTRIM(%s), ''), 'Not set')
	END`, column, column, column, column, column)
}

func executivePriceBandCase(column string) string {
	return fmt.Sprintf(`CASE
		WHEN %s IS NULL THEN 'Unknown'
		WHEN %s < 1000 THEN 'Below 1000'
		WHEN %s >= 1000 AND %s < 1500 THEN '1000-1500'
		WHEN %s >= 1500 AND %s < 2000 THEN '1500-2000'
		WHEN %s >= 2000 AND %s < 2500 THEN '2000-2500'
		WHEN %s >= 2500 AND %s < 3000 THEN '2500-3000'
		WHEN %s >= 3000 AND %s < 4000 THEN '3000-4000'
		ELSE '>4000'
	END`, column, column, column, column, column, column, column, column, column, column, column, column)
}

func queryExecutiveCount(ctx context.Context, pool *pgxpool.Pool, query string, args ...interface{}) (int, error) {
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func queryExecutiveStringList(ctx context.Context, pool *pgxpool.Pool, query string, args ...interface{}) ([]string, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func (r *OrderRepository) ListOrderPriorityInputs(ctx context.Context) ([]orderPriorityInput, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			o.amazon_order_id,
			o.priority,
			COALESCE(
				ARRAY_REMOVE(ARRAY_AGG(DISTINCT NULLIF(UPPER(BTRIM(p.sku)), '')), NULL),
				ARRAY[]::TEXT[]
			) AS skus
		FROM amazon_orders o
		LEFT JOIN amazon_order_products p
			ON p.amazon_order_id = o.amazon_order_id
			AND NOT p.is_discount_line
		GROUP BY o.amazon_order_id, o.priority
	`)
	if err != nil {
		return nil, fmt.Errorf("list order priority inputs: %w", err)
	}
	defer rows.Close()

	inputs := make([]orderPriorityInput, 0)
	for rows.Next() {
		var item orderPriorityInput
		if err := rows.Scan(&item.AmazonOrderID, &item.Priority, &item.SKUs); err != nil {
			return nil, fmt.Errorf("scan order priority input: %w", err)
		}
		inputs = append(inputs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order priority inputs: %w", err)
	}

	return inputs, nil
}

func (r *OrderRepository) UpdateOrderPriority(ctx context.Context, amazonOrderID, priority string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE amazon_orders
		SET priority = $2,
			updated_at = NOW()
		WHERE amazon_order_id = $1
	`, amazonOrderID, priority)
	if err != nil {
		return fmt.Errorf("update order priority for %s: %w", amazonOrderID, err)
	}
	return nil
}

func sortRepeatGroups(groups []models.RepeatCustomerGroup) {
	for index := range groups {
		sort.Slice(groups[index].Orders, func(i, j int) bool {
			left := groups[index].Orders[i].ConfirmedDate
			right := groups[index].Orders[j].ConfirmedDate
			if left == nil {
				return false
			}
			if right == nil {
				return true
			}
			return left.After(*right)
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].OrderCount == groups[j].OrderCount {
			return groups[i].DisplayName < groups[j].DisplayName
		}
		return groups[i].OrderCount > groups[j].OrderCount
	})
}

func buildDailyAnalyticsPeriods(start, endExclusive time.Time) []analyticsPeriod {
	periods := make([]analyticsPeriod, 0)
	for current := start; current.Before(endExclusive); current = current.AddDate(0, 0, 1) {
		next := current.AddDate(0, 0, 1)
		periods = append(periods, analyticsPeriod{
			Key:   current.Format("2006-01-02"),
			Label: current.Format("02 Jan"),
			Start: current,
			End:   next,
		})
	}
	return periods
}

func aggregateAnalyticsPoints(points []models.AnalyticsTimePoint, granularity string, location *time.Location) []models.AnalyticsTimePoint {
	if len(points) == 0 {
		return []models.AnalyticsTimePoint{}
	}

	type aggregateRow struct {
		Date  string
		Label string
		Count float64
	}

	aggregates := make([]aggregateRow, 0)
	indexByKey := make(map[string]int)

	for _, point := range points {
		parsed, err := time.ParseInLocation("2006-01-02", point.Date, location)
		if err != nil {
			continue
		}

		var bucketStart time.Time
		var label string
		switch granularity {
		case "weekly":
			weekdayOffset := (int(parsed.Weekday()) + 6) % 7
			bucketStart = parsed.AddDate(0, 0, -weekdayOffset)
			label = bucketStart.Format("02 Jan")
		case "monthly":
			bucketStart = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, location)
			label = bucketStart.Format("Jan 2006")
		default:
			bucketStart = parsed
			label = point.Label
		}

		key := bucketStart.Format("2006-01-02")
		if index, exists := indexByKey[key]; exists {
			aggregates[index].Count += point.Count
			continue
		}

		indexByKey[key] = len(aggregates)
		aggregates = append(aggregates, aggregateRow{
			Date:  key,
			Label: label,
			Count: point.Count,
		})
	}

	result := make([]models.AnalyticsTimePoint, 0, len(aggregates))
	for _, row := range aggregates {
		result = append(result, models.AnalyticsTimePoint{
			Date:  row.Date,
			Label: row.Label,
			Count: row.Count,
		})
	}
	return result
}

func (r *OrderRepository) GetDashboardAnalytics(ctx context.Context, chartWindowDays, missingRiskWindowDays int, dateFrom, dateTo *time.Time) (*models.DashboardAnalytics, error) {
	if chartWindowDays <= 0 {
		chartWindowDays = 30
	}
	if missingRiskWindowDays <= 0 {
		missingRiskWindowDays = 7
	}

	now := time.Now()
	periods := buildAnalyticsPeriods(time.Now())
	missingRiskPeriods := buildMissingRiskPeriods(now, missingRiskWindowDays, analyticsISTLocation())
	response := &models.DashboardAnalytics{
		GeneratedAt:           now,
		ChartWindowDays:       chartWindowDays,
		MissingRiskWindowDays: missingRiskWindowDays,
	}
	location := analyticsISTLocation()
	rangeEndExclusive := startOfDay(now.In(location)).AddDate(0, 0, 1)
	chartStart := startOfDay(now.In(location).AddDate(0, 0, -(chartWindowDays - 1)))
	if dateFrom != nil && dateTo != nil {
		chartStart = startOfDay(dateFrom.In(location))
		rangeEndExclusive = startOfDay(dateTo.In(location)).AddDate(0, 0, 1)
		chartWindowDays = int(rangeEndExclusive.Sub(chartStart).Hours() / 24)
		if chartWindowDays <= 0 {
			chartWindowDays = 1
		}
		response.ChartWindowDays = chartWindowDays
		response.RangeStart = chartStart.Format("2006-01-02")
		response.RangeEnd = rangeEndExclusive.Add(-time.Nanosecond).Format("2006-01-02")
	} else {
		response.RangeStart = chartStart.Format("2006-01-02")
		response.RangeEnd = rangeEndExclusive.Add(-time.Nanosecond).Format("2006-01-02")
	}
	dailyPeriods := buildDailyAnalyticsPeriods(chartStart, rangeEndExclusive)

	const ordersReceivedQuery = `
		SELECT COUNT(DISTINCT amazon_order_id)
		FROM amazon_orders
		WHERE COALESCE(date_confirmed, date_add) >= $1
		  AND COALESCE(date_confirmed, date_add) < $2
	`

	const returnsUpdatedQuery = `
		SELECT COUNT(DISTINCT amazon_order_id)
		FROM amazon_order_products
		WHERE return_initiated = TRUE
		  AND return_initiated_updated_at IS NOT NULL
		  AND return_initiated_updated_at >= $1
		  AND return_initiated_updated_at < $2
	`

	const safetyClaimsUpdatedQuery = `
		SELECT COUNT(DISTINCT amazon_order_id)
		FROM amazon_order_products
		WHERE safety_claimed = TRUE
		  AND safety_claimed_updated_at IS NOT NULL
		  AND safety_claimed_updated_at >= $1
		  AND safety_claimed_updated_at < $2
	`

	const issuesUpdatedQuery = `
		SELECT COUNT(DISTINCT amazon_order_id)
		FROM amazon_order_products
		WHERE other_issues = TRUE
		  AND other_issue_updated_at IS NOT NULL
		  AND other_issue_updated_at >= $1
		  AND other_issue_updated_at < $2
	`

	const missingCustomerDetailsQuery = `
		SELECT COUNT(*)
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		WHERE NOT p.is_discount_line
		  AND COALESCE(o.date_confirmed, o.date_add) >= $1
		  AND COALESCE(o.date_confirmed, o.date_add) < $2
		  AND p.customer_width_in_inches IS NULL
		  AND p.customer_length_in_inches IS NULL
		  AND COALESCE(NULLIF(BTRIM(p.corner_radius_and_notes), ''), '') = ''
	`

	const totalCustomerDetailRowsQuery = `
		SELECT COUNT(*)
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		WHERE NOT p.is_discount_line
		  AND COALESCE(o.date_confirmed, o.date_add) >= $1
		  AND COALESCE(o.date_confirmed, o.date_add) < $2
	`

	const incompleteCustomerDetailRowsQuery = `
		SELECT COUNT(*)
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		WHERE NOT p.is_discount_line
		  AND COALESCE(o.date_confirmed, o.date_add) >= $1
		  AND COALESCE(o.date_confirmed, o.date_add) < $2
		  AND (
		  	p.customer_width_in_inches IS NULL
		  	OR p.customer_length_in_inches IS NULL
		  	OR COALESCE(NULLIF(BTRIM(p.corner_radius_and_notes), ''), '') = ''
		  )
	`

	const ordersByLocationQuery = `
		SELECT
			COALESCE(NULLIF(TRIM(CONCAT_WS(' / ', NULLIF(BTRIM(delivery_city), ''), NULLIF(BTRIM(delivery_state), ''))), ''), 'Unknown') AS label,
			COUNT(*) AS count
		FROM amazon_orders
		WHERE COALESCE(date_confirmed, date_add) >= $1
		  AND COALESCE(date_confirmed, date_add) < $2
		GROUP BY label
		ORDER BY count DESC, label ASC
	`

	const thicknessDistributionQuery = `
		SELECT
			COALESCE(NULLIF(BTRIM(thickness), ''), 'Unknown') AS label,
			COUNT(*) AS count
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		WHERE NOT is_discount_line
		  AND COALESCE(o.date_confirmed, o.date_add) >= $1
		  AND COALESCE(o.date_confirmed, o.date_add) < $2
		GROUP BY label
		ORDER BY count DESC, label ASC
	`

	const returnsByLocationQuery = `
		SELECT
			COALESCE(NULLIF(TRIM(CONCAT_WS(' / ', NULLIF(BTRIM(o.delivery_city), ''), NULLIF(BTRIM(o.delivery_state), ''))), ''), 'Unknown') AS label,
			COUNT(DISTINCT o.amazon_order_id) AS count
		FROM amazon_orders o
		WHERE EXISTS (
			SELECT 1
			FROM amazon_order_products p
			WHERE p.amazon_order_id = o.amazon_order_id
			  AND p.return_initiated = TRUE
			  AND p.return_initiated_updated_at IS NOT NULL
			  AND p.return_initiated_updated_at >= $1
			  AND p.return_initiated_updated_at < $2
		)
		GROUP BY label
		ORDER BY count DESC, label ASC
	`

	const mostOrderedSKUsQuery = `
		SELECT
			COALESCE(NULLIF(BTRIM(p.sku), ''), 'Unknown') AS label,
			CAST(ROUND(SUM(COALESCE(p.quantity, 0))) AS INTEGER) AS count
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		WHERE NOT p.is_discount_line
		  AND COALESCE(o.date_confirmed, o.date_add) >= $1
		  AND COALESCE(o.date_confirmed, o.date_add) < $2
		GROUP BY label
		HAVING SUM(COALESCE(p.quantity, 0)) > 0
		ORDER BY SUM(COALESCE(p.quantity, 0)) DESC, label ASC
	`

	for _, period := range periods {
		ordersReceived, err := r.countDistinctOrdersBetween(ctx, ordersReceivedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count orders received for %s: %w", period.Key, err)
		}
		returnsUpdated, err := r.countDistinctOrdersBetween(ctx, returnsUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count returns updated for %s: %w", period.Key, err)
		}
		safetyClaimsUpdated, err := r.countDistinctOrdersBetween(ctx, safetyClaimsUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count safety claims updated for %s: %w", period.Key, err)
		}
		issuesUpdated, err := r.countDistinctOrdersBetween(ctx, issuesUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count issues updated for %s: %w", period.Key, err)
		}
		missingCustomerDetails, err := r.countDistinctOrdersBetween(ctx, missingCustomerDetailsQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count missing customer details for %s: %w", period.Key, err)
		}
		totalCustomerDetailRows, err := r.countDistinctOrdersBetween(ctx, totalCustomerDetailRowsQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count total customer detail rows for %s: %w", period.Key, err)
		}

		percentage := 0.0
		if totalCustomerDetailRows > 0 {
			percentage = (float64(missingCustomerDetails) / float64(totalCustomerDetailRows)) * 100
		}

		response.OrdersReceived = append(response.OrdersReceived, models.AnalyticsPeriodStat{
			Key:   period.Key,
			Label: period.Label,
			Count: ordersReceived,
		})
		response.ReturnsUpdated = append(response.ReturnsUpdated, models.AnalyticsPeriodStat{
			Key:   period.Key,
			Label: period.Label,
			Count: returnsUpdated,
		})
		response.SafetyClaimsUpdated = append(response.SafetyClaimsUpdated, models.AnalyticsPeriodStat{
			Key:   period.Key,
			Label: period.Label,
			Count: safetyClaimsUpdated,
		})
		response.IssuesUpdated = append(response.IssuesUpdated, models.AnalyticsPeriodStat{
			Key:   period.Key,
			Label: period.Label,
			Count: issuesUpdated,
		})
		response.MissingCustomerDetails = append(response.MissingCustomerDetails, models.AnalyticsPeriodStat{
			Key:        period.Key,
			Label:      period.Label,
			Count:      missingCustomerDetails,
			Total:      totalCustomerDetailRows,
			Percentage: percentage,
		})
	}

	for _, period := range dailyPeriods {
		ordersReceived, err := r.countDistinctOrdersBetween(ctx, ordersReceivedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily orders received for %s: %w", period.Key, err)
		}
		returnsUpdated, err := r.countDistinctOrdersBetween(ctx, returnsUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily returns updated for %s: %w", period.Key, err)
		}
		safetyClaimsUpdated, err := r.countDistinctOrdersBetween(ctx, safetyClaimsUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily safety claims updated for %s: %w", period.Key, err)
		}
		issuesUpdated, err := r.countDistinctOrdersBetween(ctx, issuesUpdatedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily issues updated for %s: %w", period.Key, err)
		}

		response.OrdersReceivedDaily = append(response.OrdersReceivedDaily, models.AnalyticsTimePoint{
			Date:  period.Key,
			Label: period.Label,
			Count: float64(ordersReceived),
		})
		response.ReturnsUpdatedDaily = append(response.ReturnsUpdatedDaily, models.AnalyticsTimePoint{
			Date:  period.Key,
			Label: period.Label,
			Count: float64(returnsUpdated),
		})
		response.SafetyClaimsUpdatedDaily = append(response.SafetyClaimsUpdatedDaily, models.AnalyticsTimePoint{
			Date:  period.Key,
			Label: period.Label,
			Count: float64(safetyClaimsUpdated),
		})
		response.IssuesUpdatedDaily = append(response.IssuesUpdatedDaily, models.AnalyticsTimePoint{
			Date:  period.Key,
			Label: period.Label,
			Count: float64(issuesUpdated),
		})

		missingCustomerDetails, err := r.countDistinctOrdersBetween(ctx, incompleteCustomerDetailRowsQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily missing customer detail rows for %s: %w", period.Key, err)
		}
		totalCustomerDetailRows, err := r.countDistinctOrdersBetween(ctx, totalCustomerDetailRowsQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count daily total customer detail rows for %s: %w", period.Key, err)
		}

		coveragePercentage := 0.0
		if totalCustomerDetailRows > 0 {
			coveragePercentage = (float64(missingCustomerDetails) / float64(totalCustomerDetailRows)) * 100
		}

		response.CustomerInputCoverageDaily = append(response.CustomerInputCoverageDaily, models.AnalyticsTimePoint{
			Date:  period.Key,
			Label: period.Label,
			Count: coveragePercentage,
		})
	}

	for _, period := range missingRiskPeriods {
		ordersReceived, err := r.countDistinctOrdersBetween(ctx, ordersReceivedQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count missing-risk orders received for %s: %w", period.Key, err)
		}

		missingCustomerDetails, err := r.countDistinctOrdersBetween(ctx, missingCustomerDetailsQuery, period.Start, period.End)
		if err != nil {
			return nil, fmt.Errorf("count missing-risk customer details for %s: %w", period.Key, err)
		}

		percentage := 0.0
		if ordersReceived > 0 {
			percentage = (float64(missingCustomerDetails) / float64(ordersReceived)) * 100
		}

		response.MissingDetailsRiskDaily = append(response.MissingDetailsRiskDaily, models.AnalyticsPeriodStat{
			Key:        period.Key,
			Label:      period.Label,
			Count:      missingCustomerDetails,
			Total:      ordersReceived,
			Percentage: percentage,
		})
	}

	ordersByLocation, err := r.queryChartSlices(ctx, ordersByLocationQuery, 7, chartStart, rangeEndExclusive)
	if err != nil {
		return nil, fmt.Errorf("query orders by location chart: %w", err)
	}
	thicknessDistribution, err := r.queryChartSlices(ctx, thicknessDistributionQuery, 7, chartStart, rangeEndExclusive)
	if err != nil {
		return nil, fmt.Errorf("query thickness distribution chart: %w", err)
	}
	mostOrderedSKUs, err := r.queryChartSlices(ctx, mostOrderedSKUsQuery, 7, chartStart, rangeEndExclusive)
	if err != nil {
		return nil, fmt.Errorf("query most ordered skus chart: %w", err)
	}
	returnsByLocation, err := r.queryChartSlices(ctx, returnsByLocationQuery, 7, chartStart, rangeEndExclusive)
	if err != nil {
		return nil, fmt.Errorf("query returns by location chart: %w", err)
	}

	response.OrdersByLocation = ordersByLocation
	response.ThicknessDistribution = thicknessDistribution
	response.MostOrderedSKUs = mostOrderedSKUs
	response.ReturnsByLocation = returnsByLocation

	return response, nil
}

func (r *OrderRepository) GetExecutiveDashboard(ctx context.Context, filters models.ExecutiveDashboardFilters) (*models.ExecutiveDashboardResponse, error) {
	if filters.FromDate == nil || filters.ToDate == nil {
		return nil, fmt.Errorf("executive dashboard requires from_date and to_date")
	}

	location := analyticsISTLocation()
	response := &models.ExecutiveDashboardResponse{
		GeneratedAt:           time.Now(),
		DateRange:             filters.DateRange,
		RangeStart:            filters.FromDate.In(location).Format("2006-01-02"),
		RangeEnd:              filters.ToDate.Add(-time.Second).In(location).Format("2006-01-02"),
		AvailableStates:       []string{},
		AvailableCities:       []string{},
		OrdersTrend:           []models.AnalyticsTimePoint{},
		ReturnsTrend:          []models.AnalyticsTimePoint{},
		CustomerInputGapTrend: []models.AnalyticsTimePoint{},
		IssueDistribution:     []models.AnalyticsChartSlice{},
		OrdersByThickness:     []models.AnalyticsChartSlice{},
		OrdersByState:         []models.AnalyticsChartSlice{},
		OrdersBySKU:           []models.AnalyticsChartSlice{},
		OrdersByPriceBand:     []models.AnalyticsChartSlice{},
		RecentActivity:        []models.ExecutiveDashboardRecentActivityRow{},
	}

	baseConditions, baseArgs, _ := buildExecutiveDashboardConditions(filters, "o", true, true, 1)
	baseWhere := buildWhereClause(baseConditions)

	extraConditionQuery := func(condition string, extraArgs ...interface{}) (int, error) {
		query := fmt.Sprintf("SELECT COUNT(DISTINCT o.amazon_order_id) FROM amazon_orders o %s", appendWhereCondition(baseWhere, condition))
		args := append(append([]interface{}{}, baseArgs...), extraArgs...)
		return queryExecutiveCount(ctx, r.pool, query, args...)
	}

	var err error
	if response.Summary.TotalOrders, err = queryExecutiveCount(ctx, r.pool, fmt.Sprintf("SELECT COUNT(DISTINCT o.amazon_order_id) FROM amazon_orders o %s", baseWhere), baseArgs...); err != nil {
		return nil, fmt.Errorf("count total orders: %w", err)
	}

	nextArg := len(baseArgs) + 1
	if response.Summary.ManufacturedOrders, err = extraConditionQuery(fmt.Sprintf("o.order_status = $%d", nextArg), "manufactured"); err != nil {
		return nil, fmt.Errorf("count manufactured orders: %w", err)
	}
	if response.Summary.CancelledOrders, err = extraConditionQuery(fmt.Sprintf("o.order_status = $%d", nextArg), "cancelled"); err != nil {
		return nil, fmt.Errorf("count cancelled orders: %w", err)
	}
	if response.Summary.ReturnedOrders, err = extraConditionQuery(fmt.Sprintf("o.order_status = $%d", nextArg), "returned"); err != nil {
		return nil, fmt.Errorf("count returned orders: %w", err)
	}
	if response.Summary.SafetyClaims, err = extraConditionQuery(fmt.Sprintf(`EXISTS (
		SELECT 1 FROM amazon_order_products p_filter
		WHERE p_filter.amazon_order_id = o.amazon_order_id
		AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
		AND p_filter.safety_claimed = $%d
	)`, nextArg), true); err != nil {
		return nil, fmt.Errorf("count safety claims: %w", err)
	}
	if response.Summary.OtherIssues, err = extraConditionQuery(fmt.Sprintf(`EXISTS (
		SELECT 1 FROM amazon_order_products p_filter
		WHERE p_filter.amazon_order_id = o.amazon_order_id
		AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
		AND p_filter.other_issues = $%d
	)`, nextArg), true); err != nil {
		return nil, fmt.Errorf("count other issues: %w", err)
	}
	if response.Summary.OpenReturns, err = extraConditionQuery(fmt.Sprintf(`EXISTS (
		SELECT 1 FROM amazon_order_products p_filter
		WHERE p_filter.amazon_order_id = o.amazon_order_id
		AND COALESCE(p_filter.is_discount_line, FALSE) = FALSE
		AND p_filter.return_initiated = $%d
	) AND o.order_status <> $%d`, nextArg, nextArg+1), true, "returned"); err != nil {
		return nil, fmt.Errorf("count open returns: %w", err)
	}
	response.Summary.PendingSafetyClaims = response.Summary.SafetyClaims
	response.Summary.PendingOtherIssues = response.Summary.OtherIssues
	if response.Summary.TotalOrders > 0 {
		response.Summary.ReturnRate = float64(response.Summary.ReturnedOrders) / float64(response.Summary.TotalOrders) * 100
	}

	orderTrendCounts := make(map[string]float64)
	orderTrendRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT TO_CHAR(COALESCE(o.date_confirmed, o.date_add)::date, 'YYYY-MM-DD') AS day_key,
		       COUNT(DISTINCT o.amazon_order_id) AS count
		FROM amazon_orders o
		%s
		GROUP BY 1
		ORDER BY 1
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query orders trend: %w", err)
	}
	for orderTrendRows.Next() {
		var dayKey string
		var count float64
		if err := orderTrendRows.Scan(&dayKey, &count); err != nil {
			orderTrendRows.Close()
			return nil, fmt.Errorf("scan orders trend: %w", err)
		}
		orderTrendCounts[dayKey] = count
	}
	if err := orderTrendRows.Err(); err != nil {
		orderTrendRows.Close()
		return nil, fmt.Errorf("iterate orders trend: %w", err)
	}
	orderTrendRows.Close()

	returnedTrendWhere := appendWhereCondition(baseWhere, fmt.Sprintf("o.order_status = $%d", nextArg))
	returnedTrendArgs := append(append([]interface{}{}, baseArgs...), "returned")
	returnTrendCounts := make(map[string]float64)
	returnTrendRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT TO_CHAR(COALESCE(o.order_status_updated_at, o.updated_at, o.date_confirmed, o.date_add)::date, 'YYYY-MM-DD') AS day_key,
		       COUNT(DISTINCT o.amazon_order_id) AS count
		FROM amazon_orders o
		%s
		GROUP BY 1
		ORDER BY 1
	`, returnedTrendWhere), returnedTrendArgs...)
	if err != nil {
		return nil, fmt.Errorf("query returns trend: %w", err)
	}
	for returnTrendRows.Next() {
		var dayKey string
		var count float64
		if err := returnTrendRows.Scan(&dayKey, &count); err != nil {
			returnTrendRows.Close()
			return nil, fmt.Errorf("scan returns trend: %w", err)
		}
		returnTrendCounts[dayKey] = count
	}
	if err := returnTrendRows.Err(); err != nil {
		returnTrendRows.Close()
		return nil, fmt.Errorf("iterate returns trend: %w", err)
	}
	returnTrendRows.Close()

	customerInputGapCounts := make(map[string]float64)
	customerInputGapRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH daily_orders AS (
			SELECT
				TO_CHAR(COALESCE(o.date_confirmed, o.date_add)::date, 'YYYY-MM-DD') AS day_key,
				o.amazon_order_id
			FROM amazon_orders o
			%s
		)
		SELECT
			day_key,
			(
				COUNT(DISTINCT daily_orders.amazon_order_id) FILTER (
					WHERE EXISTS (
						SELECT 1
						FROM amazon_order_products p_missing
						WHERE p_missing.amazon_order_id = daily_orders.amazon_order_id
						  AND COALESCE(p_missing.is_discount_line, FALSE) = FALSE
						  AND (
							p_missing.customer_width_in_inches IS NULL
							OR p_missing.customer_length_in_inches IS NULL
							OR COALESCE(NULLIF(BTRIM(p_missing.corner_radius_and_notes), ''), '') = ''
						  )
					)
				)::float * 100.0
			) / NULLIF(COUNT(DISTINCT daily_orders.amazon_order_id), 0) AS percentage
		FROM daily_orders
		GROUP BY day_key
		ORDER BY day_key
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query executive customer input gap trend: %w", err)
	}
	for customerInputGapRows.Next() {
		var dayKey string
		var percentage sql.NullFloat64
		if err := customerInputGapRows.Scan(&dayKey, &percentage); err != nil {
			customerInputGapRows.Close()
			return nil, fmt.Errorf("scan executive customer input gap trend: %w", err)
		}
		if percentage.Valid {
			customerInputGapCounts[dayKey] = percentage.Float64
		}
	}
	if err := customerInputGapRows.Err(); err != nil {
		customerInputGapRows.Close()
		return nil, fmt.Errorf("iterate executive customer input gap trend: %w", err)
	}
	customerInputGapRows.Close()

	dailyPeriods := buildDailyAnalyticsPeriods(filters.FromDate.In(location), filters.ToDate.In(location))
	for _, period := range dailyPeriods {
		dayKey := period.Start.Format("2006-01-02")
		response.OrdersTrend = append(response.OrdersTrend, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: orderTrendCounts[dayKey],
		})
		response.ReturnsTrend = append(response.ReturnsTrend, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: returnTrendCounts[dayKey],
		})
		response.CustomerInputGapTrend = append(response.CustomerInputGapTrend, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: customerInputGapCounts[dayKey],
		})
	}

	issueQuery := fmt.Sprintf(`
		WITH filtered_orders AS (
			SELECT
				o.amazon_order_id,
				o.order_status,
				EXISTS (
					SELECT 1 FROM amazon_order_products p_safety
					WHERE p_safety.amazon_order_id = o.amazon_order_id
					AND COALESCE(p_safety.is_discount_line, FALSE) = FALSE
					AND p_safety.safety_claimed = TRUE
				) AS has_safety_claim,
				EXISTS (
					SELECT 1 FROM amazon_order_products p_issue
					WHERE p_issue.amazon_order_id = o.amazon_order_id
					AND COALESCE(p_issue.is_discount_line, FALSE) = FALSE
					AND p_issue.other_issues = TRUE
				) AS has_other_issue
			FROM amazon_orders o
			%s
		)
		SELECT label, COUNT(*) AS count
		FROM (
			SELECT CASE
				WHEN order_status = 'returned' THEN 'Returned'
				WHEN has_safety_claim THEN 'Safety Claimed'
				WHEN has_other_issue THEN 'Other Issues'
				ELSE 'No Issues'
			END AS label
			FROM filtered_orders
		) classified
		GROUP BY label
	`, baseWhere)
	issueRows, err := r.pool.Query(ctx, issueQuery, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query issue distribution: %w", err)
	}
	issueMap := map[string]int{
		"No Issues":      0,
		"Returned":       0,
		"Safety Claimed": 0,
		"Other Issues":   0,
	}
	for issueRows.Next() {
		var label string
		var count int
		if err := issueRows.Scan(&label, &count); err != nil {
			issueRows.Close()
			return nil, fmt.Errorf("scan issue distribution: %w", err)
		}
		issueMap[label] = count
	}
	if err := issueRows.Err(); err != nil {
		issueRows.Close()
		return nil, fmt.Errorf("iterate issue distribution: %w", err)
	}
	issueRows.Close()
	response.IssueDistribution = []models.AnalyticsChartSlice{
		{Label: "No Issues", Count: issueMap["No Issues"]},
		{Label: "Returned", Count: issueMap["Returned"]},
		{Label: "Safety Claimed", Count: issueMap["Safety Claimed"]},
		{Label: "Other Issues", Count: issueMap["Other Issues"]},
	}

	thicknessRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			%s AS label,
			COUNT(DISTINCT p.amazon_order_id) AS count
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, normalizedThicknessCase("p.thickness"), appendWhereCondition(baseWhere, "COALESCE(p.is_discount_line, FALSE) = FALSE")), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query executive orders by thickness: %w", err)
	}
	for thicknessRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := thicknessRows.Scan(&slice.Label, &slice.Count); err != nil {
			thicknessRows.Close()
			return nil, fmt.Errorf("scan executive orders by thickness: %w", err)
		}
		response.OrdersByThickness = append(response.OrdersByThickness, slice)
	}
	if err := thicknessRows.Err(); err != nil {
		thicknessRows.Close()
		return nil, fmt.Errorf("iterate executive orders by thickness: %w", err)
	}
	thicknessRows.Close()

	stateRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(BTRIM(o.delivery_state), ''), 'Not available') AS label,
			COUNT(DISTINCT o.amazon_order_id) AS count
		FROM amazon_orders o
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query executive orders by state: %w", err)
	}
	for stateRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := stateRows.Scan(&slice.Label, &slice.Count); err != nil {
			stateRows.Close()
			return nil, fmt.Errorf("scan executive orders by state: %w", err)
		}
		response.OrdersByState = append(response.OrdersByState, slice)
	}
	if err := stateRows.Err(); err != nil {
		stateRows.Close()
		return nil, fmt.Errorf("iterate executive orders by state: %w", err)
	}
	stateRows.Close()

	skuRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(BTRIM(p.sku), ''), 'Unknown SKU') AS label,
			COUNT(DISTINCT p.amazon_order_id) AS count
		FROM amazon_order_products p
		INNER JOIN amazon_orders o ON o.amazon_order_id = p.amazon_order_id
		%s
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, appendWhereCondition(baseWhere, "COALESCE(p.is_discount_line, FALSE) = FALSE")), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query executive orders by sku: %w", err)
	}
	for skuRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := skuRows.Scan(&slice.Label, &slice.Count); err != nil {
			skuRows.Close()
			return nil, fmt.Errorf("scan executive orders by sku: %w", err)
		}
		response.OrdersBySKU = append(response.OrdersBySKU, slice)
	}
	if err := skuRows.Err(); err != nil {
		skuRows.Close()
		return nil, fmt.Errorf("iterate executive orders by sku: %w", err)
	}
	skuRows.Close()

	priceBandRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			%s AS label,
			COUNT(DISTINCT o.amazon_order_id) AS count
		FROM amazon_orders o
		%s
		GROUP BY 1
		ORDER BY CASE %s
			WHEN 'Below 1000' THEN 1
			WHEN '1000-1500' THEN 2
			WHEN '1500-2000' THEN 3
			WHEN '2000-2500' THEN 4
			WHEN '2500-3000' THEN 5
			WHEN '3000-4000' THEN 6
			WHEN '>4000' THEN 7
			ELSE 8
		END
	`, executivePriceBandCase("o.main_price_brutto"), baseWhere, executivePriceBandCase("o.main_price_brutto")), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query executive orders by price band: %w", err)
	}
	for priceBandRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := priceBandRows.Scan(&slice.Label, &slice.Count); err != nil {
			priceBandRows.Close()
			return nil, fmt.Errorf("scan executive orders by price band: %w", err)
		}
		response.OrdersByPriceBand = append(response.OrdersByPriceBand, slice)
	}
	if err := priceBandRows.Err(); err != nil {
		priceBandRows.Close()
		return nil, fmt.Errorf("iterate executive orders by price band: %w", err)
	}
	priceBandRows.Close()

	stateConditions, stateArgs, _ := buildExecutiveDashboardConditions(filters, "o", false, false, 1)
	stateWhere := appendWhereCondition(buildWhereClause(stateConditions), "COALESCE(NULLIF(BTRIM(o.delivery_state), ''), '') <> ''")
	if response.AvailableStates, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_state)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, stateWhere), stateArgs...); err != nil {
		return nil, fmt.Errorf("query available states: %w", err)
	}

	cityConditions, cityArgs, _ := buildExecutiveDashboardConditions(filters, "o", true, false, 1)
	cityWhere := appendWhereCondition(buildWhereClause(cityConditions), "COALESCE(NULLIF(BTRIM(o.delivery_city), ''), '') <> ''")
	if response.AvailableCities, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_city)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, cityWhere), cityArgs...); err != nil {
		return nil, fmt.Errorf("query available cities: %w", err)
	}

	recentRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			o.amazon_order_id,
			COALESCE(o.date_confirmed, o.date_add) AS confirmed_date,
			COALESCE(NULLIF(BTRIM(o.delivery_fullname), ''), NULLIF(BTRIM(o.user_login), ''), 'Unknown customer') AS customer,
			COALESCE(NULLIF(BTRIM(o.delivery_state), ''), 'Not available') AS state,
			COALESCE(STRING_AGG(DISTINCT NULLIF(BTRIM(p.thickness), ''), ', '), 'Not set') AS thickness,
			o.order_status,
			COALESCE(BOOL_OR(COALESCE(p.return_initiated, FALSE)), FALSE) AS return_initiated,
			COALESCE(BOOL_OR(COALESCE(p.safety_claimed, FALSE)), FALSE) AS safety_claimed,
			COALESCE(BOOL_OR(COALESCE(p.other_issues, FALSE)), FALSE) AS other_issue,
			GREATEST(
				o.updated_at,
				COALESCE(MAX(p.updated_at), o.updated_at),
				COALESCE(MAX(p.return_initiated_updated_at), o.updated_at),
				COALESCE(MAX(p.safety_claimed_updated_at), o.updated_at),
				COALESCE(MAX(p.other_issue_updated_at), o.updated_at)
			) AS updated_at
		FROM amazon_orders o
		LEFT JOIN amazon_order_products p
			ON p.amazon_order_id = o.amazon_order_id
			AND COALESCE(p.is_discount_line, FALSE) = FALSE
		%s
		GROUP BY
			o.amazon_order_id,
			COALESCE(o.date_confirmed, o.date_add),
			COALESCE(NULLIF(BTRIM(o.delivery_fullname), ''), NULLIF(BTRIM(o.user_login), ''), 'Unknown customer'),
			COALESCE(NULLIF(BTRIM(o.delivery_state), ''), 'Not available'),
			o.order_status,
			o.updated_at
		ORDER BY updated_at DESC
		LIMIT 20
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query recent activity: %w", err)
	}
	defer recentRows.Close()

	for recentRows.Next() {
		var row executiveRecentActivityRow
		if err := recentRows.Scan(
			&row.AmazonOrderID,
			&row.ConfirmedDate,
			&row.Customer,
			&row.State,
			&row.Thickness,
			&row.OrderStatus,
			&row.ReturnInitiated,
			&row.SafetyClaimed,
			&row.OtherIssue,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent activity: %w", err)
		}

		var confirmedDate *time.Time
		if row.ConfirmedDate.Valid {
			value := row.ConfirmedDate.Time
			confirmedDate = &value
		}

		response.RecentActivity = append(response.RecentActivity, models.ExecutiveDashboardRecentActivityRow{
			AmazonOrderID:   row.AmazonOrderID,
			ConfirmedDate:   confirmedDate,
			Customer:        row.Customer,
			State:           row.State,
			Thickness:       row.Thickness,
			OrderStatus:     row.OrderStatus,
			ReturnInitiated: row.ReturnInitiated,
			SafetyClaimed:   row.SafetyClaimed,
			OtherIssue:      row.OtherIssue,
			UpdatedAt:       row.UpdatedAt,
		})
	}
	if err := recentRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent activity: %w", err)
	}

	return response, nil
}

func (r *OrderRepository) GetReturnsDashboard(ctx context.Context, filters models.ReturnsDashboardFilters) (*models.ReturnsDashboardResponse, error) {
	if filters.FromDate == nil || filters.ToDate == nil {
		return nil, fmt.Errorf("returns dashboard requires from_date and to_date")
	}

	location := analyticsISTLocation()
	response := &models.ReturnsDashboardResponse{
		GeneratedAt:          time.Now(),
		DateRange:            filters.DateRange,
		RangeStart:           filters.FromDate.In(location).Format("2006-01-02"),
		RangeEnd:             filters.ToDate.Add(-time.Second).In(location).Format("2006-01-02"),
		AvailableStates:      []string{},
		AvailableCities:      []string{},
		ReturnsTrendDaily:    []models.AnalyticsTimePoint{},
		ReturnsTrendWeekly:   []models.AnalyticsTimePoint{},
		ReturnsTrendMonthly:  []models.AnalyticsTimePoint{},
		ThicknessPerformance: []models.ReturnsDashboardThicknessRow{},
		StatePerformance:     []models.ReturnsDashboardStateRow{},
		TopReturnCities:      []models.AnalyticsChartSlice{},
		ReturnReasons:        []models.AnalyticsChartSlice{},
		FollowupActions:      []models.AnalyticsChartSlice{},
		CompromisedBreakdown: []models.AnalyticsChartSlice{},
		ReturnsByOrderStatus: []models.AnalyticsChartSlice{},
		PendingReturns:       []models.ReturnsDashboardPendingRow{},
		ReturnOrderDetails:   []models.ReturnsDashboardDetailRow{},
		TopReturningProducts: []models.ReturnsDashboardTopProductRow{},
	}

	baseConditions, baseArgs, _ := buildReturnsDashboardConditions(filters, "o", true, true, 1)
	baseWhere := buildWhereClause(baseConditions)

	normalizedThicknessExpr := `CASE
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(p.thickness), ''), '')), ' ', '') = '1mm' THEN '1 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(p.thickness), ''), '')), ' ', '') = '1.5mm' THEN '1.5 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(p.thickness), ''), '')), ' ', '') = '2mm' THEN '2 mm'
		WHEN REPLACE(LOWER(COALESCE(NULLIF(BTRIM(p.thickness), ''), '')), ' ', '') = '3mm' THEN '3 mm'
		ELSE COALESCE(NULLIF(BTRIM(p.thickness), ''), 'Not set')
	END`

	baseCTE := fmt.Sprintf(`
		WITH filtered_orders AS (
			SELECT
				o.amazon_order_id,
				COALESCE(o.date_confirmed, o.date_add) AS confirmed_date,
				COALESCE(NULLIF(BTRIM(o.delivery_fullname), ''), NULLIF(BTRIM(o.user_login), ''), 'Unknown customer') AS customer,
				COALESCE(NULLIF(BTRIM(o.phone), ''), 'Not available') AS phone,
				COALESCE(NULLIF(BTRIM(o.delivery_state), ''), 'Not available') AS state,
				COALESCE(NULLIF(BTRIM(o.delivery_city), ''), 'Not available') AS city,
				o.order_status,
				o.updated_at
			FROM amazon_orders o
			%s
		),
		filtered_products AS (
			SELECT
				p.order_product_id,
				p.amazon_order_id,
				COALESCE(NULLIF(BTRIM(p.name), ''), 'Unnamed product') AS product_name,
				%s AS thickness,
				COALESCE(p.quantity, 0) AS quantity,
				COALESCE(NULLIF(BTRIM(p.return_initiated_reason), ''), '') AS return_reason,
				COALESCE(NULLIF(BTRIM(p.return_initiated_followup_action), ''), '') AS followup_action,
				COALESCE(p.return_initiated, FALSE) AS return_initiated,
				COALESCE(p.return_initiated_compromised, FALSE) AS return_initiated_compromised,
				p.updated_at,
				p.return_initiated_updated_at,
				p.return_initiated_compromised_updated_at
			FROM amazon_order_products p
			JOIN filtered_orders fo
				ON fo.amazon_order_id = p.amazon_order_id
			WHERE COALESCE(p.is_discount_line, FALSE) = FALSE
		),
		order_rollup AS (
			SELECT
				fo.amazon_order_id,
				fo.confirmed_date,
				fo.customer,
				fo.phone,
				fo.state,
				fo.city,
				fo.order_status,
				COALESCE(STRING_AGG(DISTINCT fp.product_name, ' | '), 'Unnamed product') AS product_summary,
				COALESCE(STRING_AGG(DISTINCT fp.thickness, ', '), 'Not set') AS thickness,
				COALESCE(SUM(fp.quantity), 0) AS quantity,
				COALESCE(BOOL_OR(fp.return_initiated), FALSE) AS return_initiated,
				COALESCE(BOOL_OR(fp.return_initiated_compromised), FALSE) AS return_initiated_compromised,
				COALESCE(STRING_AGG(DISTINCT NULLIF(fp.return_reason, ''), ' | '), '') AS return_reason,
				COALESCE(STRING_AGG(DISTINCT NULLIF(fp.followup_action, ''), ' | '), '') AS followup_action,
				MIN(CASE
					WHEN fp.return_initiated THEN COALESCE(fp.return_initiated_updated_at, fp.updated_at, fo.confirmed_date)
					ELSE NULL
				END) AS return_event_at,
				GREATEST(
					fo.updated_at,
					COALESCE(MAX(fp.updated_at), fo.updated_at),
					COALESCE(MAX(fp.return_initiated_updated_at), fo.updated_at),
					COALESCE(MAX(fp.return_initiated_compromised_updated_at), fo.updated_at)
				) AS last_updated
			FROM filtered_orders fo
			LEFT JOIN filtered_products fp
				ON fp.amazon_order_id = fo.amazon_order_id
			GROUP BY
				fo.amazon_order_id,
				fo.confirmed_date,
				fo.customer,
				fo.phone,
				fo.state,
				fo.city,
				fo.order_status,
				fo.updated_at
		)
	`, baseWhere, normalizedThicknessExpr)

	summaryQuery := baseCTE + `
		SELECT
			COUNT(*) AS total_orders,
			COUNT(*) FILTER (WHERE return_initiated) AS returns_initiated,
			COUNT(*) FILTER (WHERE order_status = 'returned') AS returned_orders,
			COUNT(*) FILTER (WHERE return_initiated_compromised) AS returns_compromised,
			COUNT(*) FILTER (WHERE return_initiated AND order_status <> 'returned') AS pending_returns
		FROM order_rollup
	`

	if err := r.pool.QueryRow(ctx, summaryQuery, baseArgs...).Scan(
		&response.Summary.TotalOrders,
		&response.Summary.ReturnsInitiated,
		&response.Summary.ReturnedOrders,
		&response.Summary.ReturnsCompromised,
		&response.Summary.PendingReturns,
	); err != nil {
		return nil, fmt.Errorf("query returns dashboard summary: %w", err)
	}

	dailyReturnCounts := make(map[string]float64)
	dailyTrendRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			TO_CHAR(return_event_at::date, 'YYYY-MM-DD') AS day_key,
			COUNT(*) AS count
		FROM order_rollup
		WHERE return_initiated = TRUE
		  AND return_event_at IS NOT NULL
		GROUP BY 1
		ORDER BY 1
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query returns trend: %w", err)
	}
	for dailyTrendRows.Next() {
		var dayKey string
		var count float64
		if err := dailyTrendRows.Scan(&dayKey, &count); err != nil {
			dailyTrendRows.Close()
			return nil, fmt.Errorf("scan returns trend: %w", err)
		}
		dailyReturnCounts[dayKey] = count
	}
	if err := dailyTrendRows.Err(); err != nil {
		dailyTrendRows.Close()
		return nil, fmt.Errorf("iterate returns trend: %w", err)
	}
	dailyTrendRows.Close()

	dailyPeriods := buildDailyAnalyticsPeriods(filters.FromDate.In(location), filters.ToDate.In(location))
	for _, period := range dailyPeriods {
		dayKey := period.Start.Format("2006-01-02")
		response.ReturnsTrendDaily = append(response.ReturnsTrendDaily, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: dailyReturnCounts[dayKey],
		})
	}
	response.ReturnsTrendWeekly = aggregateAnalyticsPoints(response.ReturnsTrendDaily, "weekly", location)
	response.ReturnsTrendMonthly = aggregateAnalyticsPoints(response.ReturnsTrendDaily, "monthly", location)

	if response.Summary.TotalOrders > 0 {
		response.Summary.ReturnRate = float64(response.Summary.ReturnsInitiated) / float64(response.Summary.TotalOrders) * 100
	}
	if response.Summary.ReturnsInitiated > 0 {
		response.Summary.CompromiseRate = float64(response.Summary.ReturnsCompromised) / float64(response.Summary.ReturnsInitiated) * 100
	}
	dayCount := len(dailyPeriods)
	if dayCount == 0 {
		dayCount = 1
	}
	response.Summary.AverageReturnsPerDay = float64(response.Summary.ReturnsInitiated) / float64(dayCount)

	thicknessMap := map[string]models.ReturnsDashboardThicknessRow{
		"1 mm":   {Thickness: "1 mm"},
		"1.5 mm": {Thickness: "1.5 mm"},
		"2 mm":   {Thickness: "2 mm"},
		"3 mm":   {Thickness: "3 mm"},
	}
	thicknessRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			fp.thickness,
			COUNT(DISTINCT fp.amazon_order_id) AS orders,
			COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.return_initiated) AS returns
		FROM filtered_products fp
		GROUP BY fp.thickness
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query thickness performance: %w", err)
	}
	for thicknessRows.Next() {
		var thickness string
		var orders int
		var returns int
		if err := thicknessRows.Scan(&thickness, &orders, &returns); err != nil {
			thicknessRows.Close()
			return nil, fmt.Errorf("scan thickness performance: %w", err)
		}
		row, exists := thicknessMap[thickness]
		if !exists {
			continue
		}
		row.Orders = orders
		row.Returns = returns
		if orders > 0 {
			row.ReturnRate = float64(returns) / float64(orders) * 100
		}
		thicknessMap[thickness] = row
	}
	if err := thicknessRows.Err(); err != nil {
		thicknessRows.Close()
		return nil, fmt.Errorf("iterate thickness performance: %w", err)
	}
	thicknessRows.Close()
	for _, key := range []string{"1 mm", "1.5 mm", "2 mm", "3 mm"} {
		response.ThicknessPerformance = append(response.ThicknessPerformance, thicknessMap[key])
	}

	stateRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			state,
			COUNT(*) AS orders,
			COUNT(*) FILTER (WHERE return_initiated) AS returns
		FROM order_rollup
		GROUP BY state
		ORDER BY returns DESC, orders DESC, state ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query state performance: %w", err)
	}
	for stateRows.Next() {
		var row models.ReturnsDashboardStateRow
		if err := stateRows.Scan(&row.State, &row.Orders, &row.Returns); err != nil {
			stateRows.Close()
			return nil, fmt.Errorf("scan state performance: %w", err)
		}
		if row.Orders > 0 {
			row.ReturnRate = float64(row.Returns) / float64(row.Orders) * 100
		}
		response.StatePerformance = append(response.StatePerformance, row)
	}
	if err := stateRows.Err(); err != nil {
		stateRows.Close()
		return nil, fmt.Errorf("iterate state performance: %w", err)
	}
	stateRows.Close()

	cityRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			city,
			COUNT(*) AS count
		FROM order_rollup
		WHERE return_initiated = TRUE
		GROUP BY city
		ORDER BY count DESC, city ASC
		LIMIT 20
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query top return cities: %w", err)
	}
	for cityRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := cityRows.Scan(&slice.Label, &slice.Count); err != nil {
			cityRows.Close()
			return nil, fmt.Errorf("scan top return cities: %w", err)
		}
		response.TopReturnCities = append(response.TopReturnCities, slice)
	}
	if err := cityRows.Err(); err != nil {
		cityRows.Close()
		return nil, fmt.Errorf("iterate top return cities: %w", err)
	}
	cityRows.Close()

	reasonRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			COALESCE(NULLIF(fp.return_reason, ''), 'Not specified') AS label,
			COUNT(*) AS count
		FROM filtered_products fp
		WHERE fp.return_initiated = TRUE
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query return reasons: %w", err)
	}
	for reasonRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := reasonRows.Scan(&slice.Label, &slice.Count); err != nil {
			reasonRows.Close()
			return nil, fmt.Errorf("scan return reasons: %w", err)
		}
		response.ReturnReasons = append(response.ReturnReasons, slice)
	}
	if err := reasonRows.Err(); err != nil {
		reasonRows.Close()
		return nil, fmt.Errorf("iterate return reasons: %w", err)
	}
	reasonRows.Close()

	followupRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			COALESCE(NULLIF(fp.followup_action, ''), 'Not specified') AS label,
			COUNT(*) AS count
		FROM filtered_products fp
		WHERE fp.return_initiated = TRUE
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query followup actions: %w", err)
	}
	for followupRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := followupRows.Scan(&slice.Label, &slice.Count); err != nil {
			followupRows.Close()
			return nil, fmt.Errorf("scan followup actions: %w", err)
		}
		response.FollowupActions = append(response.FollowupActions, slice)
	}
	if err := followupRows.Err(); err != nil {
		followupRows.Close()
		return nil, fmt.Errorf("iterate followup actions: %w", err)
	}
	followupRows.Close()

	var compromisedCount int
	var notCompromisedCount int
	if err := r.pool.QueryRow(ctx, baseCTE+`
		SELECT
			COUNT(*) FILTER (WHERE return_initiated = TRUE AND return_initiated_compromised = TRUE) AS compromised,
			COUNT(*) FILTER (WHERE return_initiated = TRUE AND return_initiated_compromised = FALSE) AS not_compromised
		FROM order_rollup
	`, baseArgs...).Scan(&compromisedCount, &notCompromisedCount); err != nil {
		return nil, fmt.Errorf("query compromised breakdown: %w", err)
	}
	response.CompromisedBreakdown = []models.AnalyticsChartSlice{
		{Label: "Compromised", Count: compromisedCount},
		{Label: "Not Compromised", Count: notCompromisedCount},
	}

	statusRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			order_status,
			COUNT(*) AS count
		FROM order_rollup
		WHERE return_initiated = TRUE
		GROUP BY order_status
		ORDER BY CASE order_status
			WHEN 'received' THEN 1
			WHEN 'manufactured' THEN 2
			WHEN 'cancelled' THEN 3
			WHEN 'returned' THEN 4
			ELSE 5
		END
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query returns by order status: %w", err)
	}
	for statusRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := statusRows.Scan(&slice.Label, &slice.Count); err != nil {
			statusRows.Close()
			return nil, fmt.Errorf("scan returns by order status: %w", err)
		}
		response.ReturnsByOrderStatus = append(response.ReturnsByOrderStatus, slice)
	}
	if err := statusRows.Err(); err != nil {
		statusRows.Close()
		return nil, fmt.Errorf("iterate returns by order status: %w", err)
	}
	statusRows.Close()

	detailRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			amazon_order_id,
			confirmed_date,
			product_summary,
			customer,
			phone,
			state,
			city,
			thickness,
			quantity,
			COALESCE(NULLIF(return_reason, ''), 'Not specified') AS return_reason,
			COALESCE(NULLIF(followup_action, ''), 'Not specified') AS followup_action,
			return_initiated_compromised,
			order_status,
			return_event_at,
			last_updated
		FROM order_rollup
		WHERE return_initiated = TRUE
		ORDER BY last_updated DESC, amazon_order_id ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query return order details: %w", err)
	}
	for detailRows.Next() {
		var confirmedDate sql.NullTime
		var eventAt sql.NullTime
		var row models.ReturnsDashboardDetailRow
		if err := detailRows.Scan(
			&row.AmazonOrderID,
			&confirmedDate,
			&row.Product,
			&row.Customer,
			&row.Phone,
			&row.State,
			&row.City,
			&row.Thickness,
			&row.Quantity,
			&row.ReturnReason,
			&row.FollowupAction,
			&row.Compromised,
			&row.OrderStatus,
			&eventAt,
			&row.UpdatedAt,
		); err != nil {
			detailRows.Close()
			return nil, fmt.Errorf("scan return order details: %w", err)
		}
		if confirmedDate.Valid {
			value := confirmedDate.Time
			row.ConfirmedDate = &value
		}
		if eventAt.Valid {
			value := eventAt.Time
			row.EventAt = &value
		}
		response.ReturnOrderDetails = append(response.ReturnOrderDetails, row)
	}
	if err := detailRows.Err(); err != nil {
		detailRows.Close()
		return nil, fmt.Errorf("iterate return order details: %w", err)
	}
	detailRows.Close()

	pendingRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			amazon_order_id,
			confirmed_date,
			customer,
			phone,
			state,
			city,
			thickness,
			quantity,
			COALESCE(NULLIF(return_reason, ''), 'Not specified') AS return_reason,
			COALESCE(NULLIF(followup_action, ''), 'Not specified') AS followup_action,
			return_initiated_compromised,
			order_status,
			last_updated
		FROM order_rollup
		WHERE return_initiated = TRUE
		  AND order_status <> 'returned'
		ORDER BY last_updated DESC, amazon_order_id ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query pending returns: %w", err)
	}
	for pendingRows.Next() {
		var confirmedDate sql.NullTime
		var row models.ReturnsDashboardPendingRow
		if err := pendingRows.Scan(
			&row.AmazonOrderID,
			&confirmedDate,
			&row.Customer,
			&row.Phone,
			&row.State,
			&row.City,
			&row.Thickness,
			&row.Quantity,
			&row.ReturnReason,
			&row.FollowupAction,
			&row.Compromised,
			&row.OrderStatus,
			&row.UpdatedAt,
		); err != nil {
			pendingRows.Close()
			return nil, fmt.Errorf("scan pending returns: %w", err)
		}
		if confirmedDate.Valid {
			value := confirmedDate.Time
			row.ConfirmedDate = &value
		}
		response.PendingReturns = append(response.PendingReturns, row)
	}
	if err := pendingRows.Err(); err != nil {
		pendingRows.Close()
		return nil, fmt.Errorf("iterate pending returns: %w", err)
	}
	pendingRows.Close()

	productRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			fp.product_name,
			COUNT(DISTINCT fp.amazon_order_id) AS orders,
			COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.return_initiated = TRUE) AS returns
		FROM filtered_products fp
		GROUP BY fp.product_name
		HAVING COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.return_initiated = TRUE) > 0
		ORDER BY
			(COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.return_initiated = TRUE))::float
				/ NULLIF(COUNT(DISTINCT fp.amazon_order_id), 0) DESC,
			returns DESC,
			orders DESC,
			fp.product_name ASC
		LIMIT 20
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query top returning products: %w", err)
	}
	for productRows.Next() {
		var row models.ReturnsDashboardTopProductRow
		if err := productRows.Scan(&row.Product, &row.Orders, &row.Returns); err != nil {
			productRows.Close()
			return nil, fmt.Errorf("scan top returning products: %w", err)
		}
		if row.Orders > 0 {
			row.ReturnRate = float64(row.Returns) / float64(row.Orders) * 100
		}
		response.TopReturningProducts = append(response.TopReturningProducts, row)
	}
	if err := productRows.Err(); err != nil {
		productRows.Close()
		return nil, fmt.Errorf("iterate top returning products: %w", err)
	}
	productRows.Close()

	stateConditions, stateArgs, _ := buildReturnsDashboardConditions(filters, "o", false, false, 1)
	stateWhere := appendWhereCondition(buildWhereClause(stateConditions), "COALESCE(NULLIF(BTRIM(o.delivery_state), ''), '') <> ''")
	if response.AvailableStates, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_state)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, stateWhere), stateArgs...); err != nil {
		return nil, fmt.Errorf("query return dashboard states: %w", err)
	}

	cityConditions, cityArgs, _ := buildReturnsDashboardConditions(filters, "o", true, false, 1)
	cityWhere := appendWhereCondition(buildWhereClause(cityConditions), "COALESCE(NULLIF(BTRIM(o.delivery_city), ''), '') <> ''")
	if response.AvailableCities, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_city)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, cityWhere), cityArgs...); err != nil {
		return nil, fmt.Errorf("query return dashboard cities: %w", err)
	}

	return response, nil
}

func (r *OrderRepository) GetSafetyClaimsDashboard(ctx context.Context, filters models.SafetyClaimsDashboardFilters) (*models.SafetyClaimsDashboardResponse, error) {
	if filters.FromDate == nil || filters.ToDate == nil {
		return nil, fmt.Errorf("safety claims dashboard requires from_date and to_date")
	}

	location := analyticsISTLocation()
	response := &models.SafetyClaimsDashboardResponse{
		GeneratedAt:          time.Now(),
		DateRange:            filters.DateRange,
		RangeStart:           filters.FromDate.In(location).Format("2006-01-02"),
		RangeEnd:             filters.ToDate.Add(-time.Second).In(location).Format("2006-01-02"),
		AvailableStates:      []string{},
		AvailableCities:      []string{},
		ClaimsTrendDaily:     []models.AnalyticsTimePoint{},
		ClaimsTrendWeekly:    []models.AnalyticsTimePoint{},
		ClaimsTrendMonthly:   []models.AnalyticsTimePoint{},
		ThicknessPerformance: []models.SafetyClaimsDashboardThicknessRow{},
		StatePerformance:     []models.SafetyClaimsDashboardStateRow{},
		TopClaimCities:       []models.AnalyticsChartSlice{},
		SafetyClaimIssues:    []models.AnalyticsChartSlice{},
		ClaimsByOrderStatus:  []models.AnalyticsChartSlice{},
		SafetyClaimCases:     []models.SafetyClaimsDashboardCaseRow{},
		OrderDetails:         []models.SafetyClaimsDashboardDetailRow{},
		TopClaimProducts:     []models.SafetyClaimsDashboardTopProductRow{},
		Insights: models.SafetyClaimsDashboardInsight{
			HighestClaimState:     "No data",
			HighestClaimThickness: "No data",
			HighestClaimProduct:   "No data",
			HighestClaimDayOfWeek: "No data",
		},
	}

	baseConditions, baseArgs, _ := buildSafetyClaimsDashboardConditions(filters, "o", true, true, 1)
	baseWhere := buildWhereClause(baseConditions)
	normalizedThicknessExpr := normalizedThicknessCase("p.thickness")

	baseCTE := fmt.Sprintf(`
		WITH filtered_orders AS (
			SELECT
				o.amazon_order_id,
				COALESCE(o.date_confirmed, o.date_add) AS confirmed_date,
				COALESCE(NULLIF(BTRIM(o.delivery_fullname), ''), NULLIF(BTRIM(o.user_login), ''), 'Unknown customer') AS customer,
				COALESCE(NULLIF(BTRIM(o.phone), ''), 'Not available') AS phone,
				COALESCE(NULLIF(BTRIM(o.delivery_state), ''), 'Not available') AS state,
				COALESCE(NULLIF(BTRIM(o.delivery_city), ''), 'Not available') AS city,
				o.order_status,
				o.updated_at
			FROM amazon_orders o
			%s
		),
		filtered_products AS (
			SELECT
				p.order_product_id,
				p.amazon_order_id,
				COALESCE(NULLIF(BTRIM(p.name), ''), 'Unnamed product') AS product_name,
				%s AS thickness,
				COALESCE(NULLIF(BTRIM(p.safety_claim_issues), ''), '') AS safety_claim_issues,
				COALESCE(p.safety_claimed, FALSE) AS safety_claimed,
				p.updated_at,
				p.safety_claimed_updated_at
			FROM amazon_order_products p
			JOIN filtered_orders fo
				ON fo.amazon_order_id = p.amazon_order_id
			WHERE COALESCE(p.is_discount_line, FALSE) = FALSE
		),
		order_rollup AS (
			SELECT
				fo.amazon_order_id,
				fo.confirmed_date,
				fo.customer,
				fo.phone,
				fo.state,
				fo.city,
				fo.order_status,
				COALESCE(STRING_AGG(DISTINCT fp.product_name, ' | '), 'Unnamed product') AS product_summary,
				COALESCE(STRING_AGG(DISTINCT fp.thickness, ', '), 'Not set') AS thickness,
				COALESCE(BOOL_OR(fp.safety_claimed), FALSE) AS safety_claimed,
				COALESCE(STRING_AGG(DISTINCT NULLIF(fp.safety_claim_issues, ''), ' | '), '') AS safety_claim_issues,
				MIN(CASE
					WHEN fp.safety_claimed THEN COALESCE(fp.safety_claimed_updated_at, fp.updated_at, fo.confirmed_date)
					ELSE NULL
				END) AS safety_claim_event_at,
				GREATEST(
					fo.updated_at,
					COALESCE(MAX(fp.updated_at), fo.updated_at),
					COALESCE(MAX(fp.safety_claimed_updated_at), fo.updated_at)
				) AS last_updated
			FROM filtered_orders fo
			LEFT JOIN filtered_products fp
				ON fp.amazon_order_id = fo.amazon_order_id
			GROUP BY
				fo.amazon_order_id,
				fo.confirmed_date,
				fo.customer,
				fo.phone,
				fo.state,
				fo.city,
				fo.order_status,
				fo.updated_at
		)
	`, baseWhere, normalizedThicknessExpr)

	summaryQuery := baseCTE + `
		SELECT
			COUNT(*) AS total_orders,
			COUNT(*) FILTER (WHERE safety_claimed) AS safety_claimed_orders,
			COUNT(*) FILTER (WHERE order_status = 'returned') AS returned_orders,
			COUNT(*) FILTER (WHERE order_status = 'returned' AND safety_claimed) AS returned_orders_with_safety_claims
		FROM order_rollup
	`

	if err := r.pool.QueryRow(ctx, summaryQuery, baseArgs...).Scan(
		&response.Summary.TotalOrders,
		&response.Summary.SafetyClaimedOrders,
		&response.Summary.ReturnedOrders,
		&response.Summary.ReturnedOrdersWithClaims,
	); err != nil {
		return nil, fmt.Errorf("query safety claims dashboard summary: %w", err)
	}
	response.Summary.PendingSafetyClaims = response.Summary.SafetyClaimedOrders
	if response.Summary.TotalOrders > 0 {
		response.Summary.SafetyClaimRate = float64(response.Summary.SafetyClaimedOrders) / float64(response.Summary.TotalOrders) * 100
	}
	if response.Summary.ReturnedOrders > 0 {
		response.Summary.SafetyClaimConversionRate = float64(response.Summary.ReturnedOrdersWithClaims) / float64(response.Summary.ReturnedOrders) * 100
	}

	dailyClaimCounts := make(map[string]float64)
	dailyTrendRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			TO_CHAR(safety_claim_event_at::date, 'YYYY-MM-DD') AS day_key,
			COUNT(*) AS count
		FROM order_rollup
		WHERE safety_claimed = TRUE
		  AND safety_claim_event_at IS NOT NULL
		GROUP BY 1
		ORDER BY 1
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claims trend: %w", err)
	}
	for dailyTrendRows.Next() {
		var dayKey string
		var count float64
		if err := dailyTrendRows.Scan(&dayKey, &count); err != nil {
			dailyTrendRows.Close()
			return nil, fmt.Errorf("scan safety claims trend: %w", err)
		}
		dailyClaimCounts[dayKey] = count
	}
	if err := dailyTrendRows.Err(); err != nil {
		dailyTrendRows.Close()
		return nil, fmt.Errorf("iterate safety claims trend: %w", err)
	}
	dailyTrendRows.Close()

	dailyPeriods := buildDailyAnalyticsPeriods(filters.FromDate.In(location), filters.ToDate.In(location))
	for _, period := range dailyPeriods {
		dayKey := period.Start.Format("2006-01-02")
		response.ClaimsTrendDaily = append(response.ClaimsTrendDaily, models.AnalyticsTimePoint{
			Date:  dayKey,
			Label: period.Label,
			Count: dailyClaimCounts[dayKey],
		})
	}
	response.ClaimsTrendWeekly = aggregateAnalyticsPoints(response.ClaimsTrendDaily, "weekly", location)
	response.ClaimsTrendMonthly = aggregateAnalyticsPoints(response.ClaimsTrendDaily, "monthly", location)

	thicknessMap := map[string]models.SafetyClaimsDashboardThicknessRow{
		"1 mm":   {Thickness: "1 mm"},
		"1.5 mm": {Thickness: "1.5 mm"},
		"2 mm":   {Thickness: "2 mm"},
		"3 mm":   {Thickness: "3 mm"},
	}
	thicknessRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			fp.thickness,
			COUNT(DISTINCT fp.amazon_order_id) AS orders,
			COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed) AS claims
		FROM filtered_products fp
		GROUP BY fp.thickness
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claim thickness performance: %w", err)
	}
	for thicknessRows.Next() {
		var thickness string
		var orders int
		var claims int
		if err := thicknessRows.Scan(&thickness, &orders, &claims); err != nil {
			thicknessRows.Close()
			return nil, fmt.Errorf("scan safety claim thickness performance: %w", err)
		}
		row, exists := thicknessMap[thickness]
		if !exists {
			continue
		}
		row.Orders = orders
		row.Claims = claims
		if orders > 0 {
			row.ClaimRate = float64(claims) / float64(orders) * 100
		}
		thicknessMap[thickness] = row
	}
	if err := thicknessRows.Err(); err != nil {
		thicknessRows.Close()
		return nil, fmt.Errorf("iterate safety claim thickness performance: %w", err)
	}
	thicknessRows.Close()
	for _, key := range []string{"1 mm", "1.5 mm", "2 mm", "3 mm"} {
		response.ThicknessPerformance = append(response.ThicknessPerformance, thicknessMap[key])
	}

	stateRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			state,
			COUNT(*) AS orders,
			COUNT(*) FILTER (WHERE safety_claimed) AS claims
		FROM order_rollup
		GROUP BY state
		ORDER BY claims DESC, orders DESC, state ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claim state performance: %w", err)
	}
	for stateRows.Next() {
		var row models.SafetyClaimsDashboardStateRow
		if err := stateRows.Scan(&row.State, &row.Orders, &row.Claims); err != nil {
			stateRows.Close()
			return nil, fmt.Errorf("scan safety claim state performance: %w", err)
		}
		if row.Orders > 0 {
			row.ClaimRate = float64(row.Claims) / float64(row.Orders) * 100
		}
		response.StatePerformance = append(response.StatePerformance, row)
	}
	if err := stateRows.Err(); err != nil {
		stateRows.Close()
		return nil, fmt.Errorf("iterate safety claim state performance: %w", err)
	}
	stateRows.Close()

	cityRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			city,
			COUNT(*) AS count
		FROM order_rollup
		WHERE safety_claimed = TRUE
		GROUP BY city
		ORDER BY count DESC, city ASC
		LIMIT 20
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query top claim cities: %w", err)
	}
	for cityRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := cityRows.Scan(&slice.Label, &slice.Count); err != nil {
			cityRows.Close()
			return nil, fmt.Errorf("scan top claim cities: %w", err)
		}
		response.TopClaimCities = append(response.TopClaimCities, slice)
	}
	if err := cityRows.Err(); err != nil {
		cityRows.Close()
		return nil, fmt.Errorf("iterate top claim cities: %w", err)
	}
	cityRows.Close()

	issueRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			COALESCE(NULLIF(fp.safety_claim_issues, ''), 'Not specified') AS label,
			COUNT(*) AS count
		FROM filtered_products fp
		WHERE fp.safety_claimed = TRUE
		GROUP BY 1
		ORDER BY count DESC, label ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claim issues: %w", err)
	}
	for issueRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := issueRows.Scan(&slice.Label, &slice.Count); err != nil {
			issueRows.Close()
			return nil, fmt.Errorf("scan safety claim issues: %w", err)
		}
		response.SafetyClaimIssues = append(response.SafetyClaimIssues, slice)
	}
	if err := issueRows.Err(); err != nil {
		issueRows.Close()
		return nil, fmt.Errorf("iterate safety claim issues: %w", err)
	}
	issueRows.Close()

	statusRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			order_status,
			COUNT(*) AS count
		FROM order_rollup
		WHERE safety_claimed = TRUE
		GROUP BY order_status
		ORDER BY CASE order_status
			WHEN 'received' THEN 1
			WHEN 'manufactured' THEN 2
			WHEN 'cancelled' THEN 3
			WHEN 'returned' THEN 4
			ELSE 5
		END
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query claims by order status: %w", err)
	}
	for statusRows.Next() {
		var slice models.AnalyticsChartSlice
		if err := statusRows.Scan(&slice.Label, &slice.Count); err != nil {
			statusRows.Close()
			return nil, fmt.Errorf("scan claims by order status: %w", err)
		}
		response.ClaimsByOrderStatus = append(response.ClaimsByOrderStatus, slice)
	}
	if err := statusRows.Err(); err != nil {
		statusRows.Close()
		return nil, fmt.Errorf("iterate claims by order status: %w", err)
	}
	statusRows.Close()

	detailRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			amazon_order_id,
			confirmed_date,
			customer,
			phone,
			state,
			city,
			thickness,
			product_summary,
			order_status,
			safety_claimed,
			COALESCE(NULLIF(safety_claim_issues, ''), 'Not specified') AS safety_claim_issues,
			safety_claim_event_at,
			last_updated
		FROM order_rollup
		ORDER BY last_updated DESC, amazon_order_id ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claim order details: %w", err)
	}
	for detailRows.Next() {
		var confirmedDate sql.NullTime
		var eventAt sql.NullTime
		var row models.SafetyClaimsDashboardDetailRow
		if err := detailRows.Scan(
			&row.AmazonOrderID,
			&confirmedDate,
			&row.Customer,
			&row.Phone,
			&row.State,
			&row.City,
			&row.Thickness,
			&row.Product,
			&row.OrderStatus,
			&row.SafetyClaimed,
			&row.SafetyClaimIssues,
			&eventAt,
			&row.UpdatedAt,
		); err != nil {
			detailRows.Close()
			return nil, fmt.Errorf("scan safety claim order details: %w", err)
		}
		if confirmedDate.Valid {
			value := confirmedDate.Time
			row.ConfirmedDate = &value
		}
		if eventAt.Valid {
			value := eventAt.Time
			row.EventAt = &value
		}
		response.OrderDetails = append(response.OrderDetails, row)
	}
	if err := detailRows.Err(); err != nil {
		detailRows.Close()
		return nil, fmt.Errorf("iterate safety claim order details: %w", err)
	}
	detailRows.Close()

	caseRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			amazon_order_id,
			confirmed_date,
			customer,
			phone,
			state,
			city,
			thickness,
			product_summary,
			order_status,
			COALESCE(NULLIF(safety_claim_issues, ''), 'Not specified') AS safety_claim_issues,
			last_updated
		FROM order_rollup
		WHERE safety_claimed = TRUE
		ORDER BY last_updated DESC, amazon_order_id ASC
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query safety claim cases: %w", err)
	}
	for caseRows.Next() {
		var confirmedDate sql.NullTime
		var row models.SafetyClaimsDashboardCaseRow
		if err := caseRows.Scan(
			&row.AmazonOrderID,
			&confirmedDate,
			&row.Customer,
			&row.Phone,
			&row.State,
			&row.City,
			&row.Thickness,
			&row.Product,
			&row.OrderStatus,
			&row.SafetyClaimIssues,
			&row.UpdatedAt,
		); err != nil {
			caseRows.Close()
			return nil, fmt.Errorf("scan safety claim cases: %w", err)
		}
		if confirmedDate.Valid {
			value := confirmedDate.Time
			row.ConfirmedDate = &value
		}
		response.SafetyClaimCases = append(response.SafetyClaimCases, row)
	}
	if err := caseRows.Err(); err != nil {
		caseRows.Close()
		return nil, fmt.Errorf("iterate safety claim cases: %w", err)
	}
	caseRows.Close()

	productRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			fp.product_name,
			COUNT(DISTINCT fp.amazon_order_id) AS orders,
			COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed = TRUE) AS claims
		FROM filtered_products fp
		GROUP BY fp.product_name
		HAVING COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed = TRUE) > 0
		ORDER BY
			(COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed = TRUE))::float
				/ NULLIF(COUNT(DISTINCT fp.amazon_order_id), 0) DESC,
			claims DESC,
			orders DESC,
			fp.product_name ASC
		LIMIT 20
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query top claim products: %w", err)
	}
	for productRows.Next() {
		var row models.SafetyClaimsDashboardTopProductRow
		if err := productRows.Scan(&row.Product, &row.Orders, &row.Claims); err != nil {
			productRows.Close()
			return nil, fmt.Errorf("scan top claim products: %w", err)
		}
		if row.Orders > 0 {
			row.ClaimRate = float64(row.Claims) / float64(row.Orders) * 100
		}
		response.TopClaimProducts = append(response.TopClaimProducts, row)
	}
	if err := productRows.Err(); err != nil {
		productRows.Close()
		return nil, fmt.Errorf("iterate top claim products: %w", err)
	}
	productRows.Close()

	if len(response.StatePerformance) > 0 && response.StatePerformance[0].Claims > 0 {
		response.Insights.HighestClaimState = response.StatePerformance[0].State
	}

	topThicknessLabel := "No data"
	topThicknessClaims := 0
	for _, row := range response.ThicknessPerformance {
		if row.Claims > topThicknessClaims {
			topThicknessClaims = row.Claims
			topThicknessLabel = row.Thickness
		}
	}
	if topThicknessClaims > 0 {
		response.Insights.HighestClaimThickness = topThicknessLabel
	}

	productInsightRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			fp.product_name,
			COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed = TRUE) AS claims
		FROM filtered_products fp
		GROUP BY fp.product_name
		HAVING COUNT(DISTINCT fp.amazon_order_id) FILTER (WHERE fp.safety_claimed = TRUE) > 0
		ORDER BY claims DESC, fp.product_name ASC
		LIMIT 1
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query highest claim product: %w", err)
	}
	for productInsightRows.Next() {
		var product string
		var claims int
		if err := productInsightRows.Scan(&product, &claims); err != nil {
			productInsightRows.Close()
			return nil, fmt.Errorf("scan highest claim product: %w", err)
		}
		if claims > 0 {
			response.Insights.HighestClaimProduct = product
		}
	}
	if err := productInsightRows.Err(); err != nil {
		productInsightRows.Close()
		return nil, fmt.Errorf("iterate highest claim product: %w", err)
	}
	productInsightRows.Close()

	dayRows, err := r.pool.Query(ctx, baseCTE+`
		SELECT
			TRIM(TO_CHAR(safety_claim_event_at, 'Day')) AS day_name,
			COUNT(*) AS claims
		FROM order_rollup
		WHERE safety_claimed = TRUE
		  AND safety_claim_event_at IS NOT NULL
		GROUP BY 1
		ORDER BY claims DESC, day_name ASC
		LIMIT 1
	`, baseArgs...)
	if err != nil {
		return nil, fmt.Errorf("query highest claim day of week: %w", err)
	}
	for dayRows.Next() {
		var dayName string
		var claims int
		if err := dayRows.Scan(&dayName, &claims); err != nil {
			dayRows.Close()
			return nil, fmt.Errorf("scan highest claim day of week: %w", err)
		}
		if claims > 0 {
			response.Insights.HighestClaimDayOfWeek = dayName
		}
	}
	if err := dayRows.Err(); err != nil {
		dayRows.Close()
		return nil, fmt.Errorf("iterate highest claim day of week: %w", err)
	}
	dayRows.Close()

	stateConditions, stateArgs, _ := buildSafetyClaimsDashboardConditions(filters, "o", false, false, 1)
	stateWhere := appendWhereCondition(buildWhereClause(stateConditions), "COALESCE(NULLIF(BTRIM(o.delivery_state), ''), '') <> ''")
	if response.AvailableStates, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_state)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, stateWhere), stateArgs...); err != nil {
		return nil, fmt.Errorf("query safety dashboard states: %w", err)
	}

	cityConditions, cityArgs, _ := buildSafetyClaimsDashboardConditions(filters, "o", true, false, 1)
	cityWhere := appendWhereCondition(buildWhereClause(cityConditions), "COALESCE(NULLIF(BTRIM(o.delivery_city), ''), '') <> ''")
	if response.AvailableCities, err = queryExecutiveStringList(ctx, r.pool, fmt.Sprintf(`
		SELECT DISTINCT BTRIM(o.delivery_city)
		FROM amazon_orders o
		%s
		ORDER BY 1
	`, cityWhere), cityArgs...); err != nil {
		return nil, fmt.Errorf("query safety dashboard cities: %w", err)
	}

	return response, nil
}

func (r *OrderRepository) GetRepeatCustomers(ctx context.Context, returnsOnly bool, confirmedDateFrom, confirmedDateTo *time.Time) (*models.RepeatCustomerResponse, error) {
	query := `
		WITH base_orders AS (
			SELECT
				o.amazon_order_id,
				COALESCE(o.date_confirmed, o.date_add) AS confirmed_date,
				o.order_status,
				COALESCE(NULLIF(BTRIM(o.delivery_fullname), ''), NULLIF(BTRIM(o.user_login), ''), 'Unknown customer') AS customer,
				COALESCE(NULLIF(BTRIM(o.phone), ''), '') AS phone,
				COALESCE(NULLIF(BTRIM(o.delivery_address), ''), '') AS address,
				COALESCE(NULLIF(BTRIM(o.delivery_city), ''), '') AS city,
				COALESCE(NULLIF(BTRIM(o.delivery_state), ''), '') AS state,
				COALESCE(NULLIF(BTRIM(o.delivery_postcode), ''), '') AS postcode,
				REGEXP_REPLACE(COALESCE(NULLIF(BTRIM(o.phone), ''), ''), '[^0-9]', '', 'g') AS phone_key,
				ARRAY_TO_STRING(
					ARRAY_REMOVE(ARRAY[
						NULLIF(LOWER(BTRIM(o.delivery_address)), ''),
						NULLIF(LOWER(BTRIM(o.delivery_city)), ''),
						NULLIF(LOWER(BTRIM(o.delivery_state)), ''),
						NULLIF(LOWER(BTRIM(o.delivery_postcode)), '')
					], NULL),
					'|'
				) AS address_key
			FROM amazon_orders o
			WHERE ($1 = FALSE OR EXISTS (
				SELECT 1
				FROM amazon_order_products p_return
				WHERE p_return.amazon_order_id = o.amazon_order_id
				  AND p_return.return_initiated = TRUE
			))
			  AND ($2::timestamp IS NULL OR COALESCE(o.date_confirmed, o.date_add) >= $2::timestamp)
			  AND ($3::timestamp IS NULL OR COALESCE(o.date_confirmed, o.date_add) < $3::timestamp)
		),
		duplicate_phones AS (
			SELECT phone_key
			FROM base_orders
			WHERE phone_key <> ''
			GROUP BY phone_key
			HAVING COUNT(*) > 1
		),
		duplicate_addresses AS (
			SELECT address_key
			FROM base_orders
			WHERE address_key <> ''
			GROUP BY address_key
			HAVING COUNT(*) > 1
		),
		matching_orders AS (
			SELECT DISTINCT
				bo.amazon_order_id,
				bo.confirmed_date,
				bo.order_status,
				bo.customer,
				bo.phone,
				bo.address,
				bo.city,
				bo.state,
				bo.postcode
			FROM base_orders bo
			LEFT JOIN duplicate_phones dp ON dp.phone_key = bo.phone_key
			LEFT JOIN duplicate_addresses da ON da.address_key = bo.address_key
			WHERE dp.phone_key IS NOT NULL
			   OR da.address_key IS NOT NULL
		)
		SELECT
			mo.amazon_order_id,
			mo.confirmed_date,
			mo.order_status,
			mo.customer,
			mo.phone,
			mo.address,
			mo.city,
			mo.state,
			mo.postcode,
			COALESCE(products.product_summary, '') AS product_summary
		FROM matching_orders mo
		LEFT JOIN LATERAL (
			SELECT STRING_AGG(
				DISTINCT COALESCE(NULLIF(BTRIM(p.sku), ''), NULLIF(BTRIM(p.name), ''), 'Product'),
				' | '
			) AS product_summary
			FROM amazon_order_products p
			WHERE p.amazon_order_id = mo.amazon_order_id
			  AND NOT p.is_discount_line
		) products ON TRUE
		ORDER BY mo.confirmed_date DESC NULLS LAST, mo.amazon_order_id DESC
	`

	rows, err := r.pool.Query(ctx, query, returnsOnly, confirmedDateFrom, confirmedDateTo)
	if err != nil {
		return nil, fmt.Errorf("query repeat customers: %w", err)
	}
	defer rows.Close()

	phoneGroups := map[string][]models.RepeatCustomerOrder{}
	addressGroups := map[string][]models.RepeatCustomerOrder{}
	phoneDisplay := map[string]string{}
	addressDisplay := map[string]string{}

	for rows.Next() {
		var row repeatCustomerRow
		if err := rows.Scan(
			&row.AmazonOrderID,
			&row.ConfirmedDate,
			&row.OrderStatus,
			&row.Customer,
			&row.Phone,
			&row.Address,
			&row.City,
			&row.State,
			&row.Postcode,
			&row.ProductSummary,
		); err != nil {
			return nil, fmt.Errorf("scan repeat customer row: %w", err)
		}

		order := models.RepeatCustomerOrder{
			AmazonOrderID:  row.AmazonOrderID,
			OrderStatus:    row.OrderStatus,
			Customer:       row.Customer,
			Phone:          row.Phone,
			Address:        row.Address,
			City:           row.City,
			State:          row.State,
			ProductSummary: row.ProductSummary,
		}
		if row.ConfirmedDate.Valid {
			confirmedAt := row.ConfirmedDate.Time
			order.ConfirmedDate = &confirmedAt
		}

		phoneKey := normalizePhoneForGrouping(row.Phone)
		if phoneKey != "" {
			phoneGroups[phoneKey] = append(phoneGroups[phoneKey], order)
			if phoneDisplay[phoneKey] == "" {
				phoneDisplay[phoneKey] = row.Phone
			}
		}

		addressKey := normalizeAddressForGrouping(row.Address, row.City, row.State, row.Postcode)
		if addressKey != "" {
			addressGroups[addressKey] = append(addressGroups[addressKey], order)
			if addressDisplay[addressKey] == "" {
				addressDisplay[addressKey] = strings.TrimSpace(strings.Join([]string{
					row.Address,
					row.City,
					row.State,
					row.Postcode,
				}, ", "))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repeat customers: %w", err)
	}

	response := &models.RepeatCustomerResponse{
		Scope: "orders",
	}
	if returnsOnly {
		response.Scope = "returns"
	}

	for key, orders := range phoneGroups {
		if len(orders) <= 1 {
			continue
		}
		response.ByPhone = append(response.ByPhone, models.RepeatCustomerGroup{
			GroupKey:    key,
			DisplayName: phoneDisplay[key],
			OrderCount:  len(orders),
			Orders:      orders,
		})
	}

	for key, orders := range addressGroups {
		if len(orders) <= 1 {
			continue
		}
		response.ByAddress = append(response.ByAddress, models.RepeatCustomerGroup{
			GroupKey:    key,
			DisplayName: addressDisplay[key],
			OrderCount:  len(orders),
			Orders:      orders,
		})
	}

	sortRepeatGroups(response.ByPhone)
	sortRepeatGroups(response.ByAddress)

	return response, nil
}

// SetInteraktClient sets the Interakt client used for new-order WhatsApp sends.
func (r *OrderRepository) SetInteraktClient(client *interakt.Client) {
	r.interaktClient = client
}

// UpsertOrder inserts or updates an order with its products in a transaction
func (r *OrderRepository) UpsertOrder(ctx context.Context, order *models.AmazonOrder, products []models.OrderProduct) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Track if this is a new order
	isNewOrder := false

	// Check if order exists
	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM amazon_orders WHERE amazon_order_id = $1)", order.AmazonOrderID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check order existence: %w", err)
	}
	isNewOrder = !exists
	log.Printf("🧾 Upsert starting for Amazon order %s (is_new=%t, products=%d)", order.AmazonOrderID, isNewOrder, len(products))

	// Upsert order
	orderInserted, err := r.upsertOrderTx(ctx, tx, order)
	if err != nil {
		return err
	}
	if !orderInserted {
		log.Printf("ℹ️  Amazon order %s already existed; ON CONFLICT updated the existing row", order.AmazonOrderID)
	}

	// Upsert products
	insertedProducts := 0
	for _, product := range products {
		inserted, err := r.upsertProductTx(ctx, tx, &product)
		if err != nil {
			return err
		}
		if inserted {
			insertedProducts++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Printf("✅ Upsert committed for Amazon order %s (order_inserted=%t, products_inserted=%d/%d)", order.AmazonOrderID, orderInserted, insertedProducts, len(products))

	if err := r.backfillDerivedDimensions(ctx, order); err != nil {
		log.Printf("⚠️  Failed to backfill derived dimensions during upsert for order %s: %v", order.AmazonOrderID, err)
	}

	// Send WhatsApp message only for freshly inserted orders if Interakt is enabled.
	switch {
	case !orderInserted:
		log.Printf("ℹ️  Interakt send skipped for Amazon order %s because this upsert updated an existing order", order.AmazonOrderID)
	case r.interaktClient == nil:
		log.Printf("ℹ️  Interakt send skipped for Amazon order %s because Interakt client is not configured", order.AmazonOrderID)
	case !r.interaktClient.Enabled():
		log.Printf("ℹ️  Interakt send skipped for Amazon order %s because Interakt is disabled", order.AmazonOrderID)
	default:
		if err := r.sendOrderConfirmationWhatsApp(ctx, order, products); err != nil {
			// Log error but don't fail the transaction
			log.Printf("⚠️  Failed to send WhatsApp message for order %s: %v", order.AmazonOrderID, err)
		}
	}

	return nil
}

// sendOrderConfirmationWhatsApp sends the new-order WhatsApp message via Interakt.
func (r *OrderRepository) sendOrderConfirmationWhatsApp(ctx context.Context, order *models.AmazonOrder, products []models.OrderProduct) error {
	customerName := ""
	if order.DeliveryFullname.Valid {
		customerName = order.DeliveryFullname.String
	}

	phone := ""
	if order.Phone.Valid {
		phone = order.Phone.String
	}

	orderDetails := utils.FormatOrderDetails(products)
	_, err := r.interaktClient.SendOrderMessage(ctx, interakt.SendOrderMessageRequest{
		CustomerName: customerName,
		OrderID:      order.AmazonOrderID,
		OrderDetails: orderDetails,
		PhoneNumber:  phone,
		CallbackData: order.AmazonOrderID,
	})
	if err != nil {
		return fmt.Errorf("send order confirmation via Interakt: %w", err)
	}

	log.Printf("✅ Order-level Interakt workflow completed for order %s", order.AmazonOrderID)
	return nil
}

func (r *OrderRepository) upsertOrderTx(ctx context.Context, tx pgx.Tx, order *models.AmazonOrder) (bool, error) {
	query := `
		INSERT INTO amazon_orders (
			amazon_order_id, baselinker_order_id, shop_order_id,
			order_source, order_source_id, order_source_info,
			order_status_id, confirmed, date_confirmed, date_add, date_in_status,
			user_login, phone, email, user_comments, admin_comments,
			currency, payment_method, payment_method_cod, payment_done,
			delivery_method_id, delivery_method, delivery_price,
			delivery_package_module, delivery_package_nr,
			delivery_fullname, delivery_company, delivery_address,
			delivery_city, delivery_state, delivery_postcode,
			delivery_country_code, delivery_country,
			delivery_point_id, delivery_point_name, delivery_point_address,
			delivery_point_postcode, delivery_point_city,
			invoice_fullname, invoice_company, invoice_nip, invoice_address,
			invoice_city, invoice_state, invoice_postcode,
			invoice_country_code, invoice_country, want_invoice,
			extra_field_1, extra_field_2, order_page,
			pick_state, pack_state, star, crm_client_id,
			main_order_product_id, main_product_name, main_sku, main_asin,
			main_price_brutto, main_tax_rate, main_quantity,
			default_width_in_inches, default_length_in_inches,
			default_width_in_mm, default_length_in_mm,
			priority, is_round, order_status,
			raw_payload
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35, $36, $37, $38, $39, $40,
			$41, $42, $43, $44, $45, $46, $47, $48, $49, $50,
			$51, $52, $53, $54, $55, $56, $57, $58, $59, $60,
			$61, $62, $63, $64, $65, $66, $67,
			$68, $69, $70
		)
		ON CONFLICT (amazon_order_id) DO UPDATE SET
			baselinker_order_id = EXCLUDED.baselinker_order_id,
			shop_order_id = EXCLUDED.shop_order_id,
			order_source = EXCLUDED.order_source,
			order_source_id = EXCLUDED.order_source_id,
			order_source_info = EXCLUDED.order_source_info,
			order_status_id = EXCLUDED.order_status_id,
			confirmed = EXCLUDED.confirmed,
			date_confirmed = EXCLUDED.date_confirmed,
			date_add = EXCLUDED.date_add,
			date_in_status = EXCLUDED.date_in_status,
			user_login = EXCLUDED.user_login,
			phone = EXCLUDED.phone,
			email = EXCLUDED.email,
			user_comments = EXCLUDED.user_comments,
			admin_comments = EXCLUDED.admin_comments,
			currency = EXCLUDED.currency,
			payment_method = EXCLUDED.payment_method,
			payment_method_cod = EXCLUDED.payment_method_cod,
			payment_done = EXCLUDED.payment_done,
			delivery_method_id = EXCLUDED.delivery_method_id,
			delivery_method = EXCLUDED.delivery_method,
			delivery_price = EXCLUDED.delivery_price,
			delivery_package_module = EXCLUDED.delivery_package_module,
			delivery_package_nr = EXCLUDED.delivery_package_nr,
			delivery_fullname = EXCLUDED.delivery_fullname,
			delivery_company = EXCLUDED.delivery_company,
			delivery_address = EXCLUDED.delivery_address,
			delivery_city = EXCLUDED.delivery_city,
			delivery_state = EXCLUDED.delivery_state,
			delivery_postcode = EXCLUDED.delivery_postcode,
			delivery_country_code = EXCLUDED.delivery_country_code,
			delivery_country = EXCLUDED.delivery_country,
			delivery_point_id = EXCLUDED.delivery_point_id,
			delivery_point_name = EXCLUDED.delivery_point_name,
			delivery_point_address = EXCLUDED.delivery_point_address,
			delivery_point_postcode = EXCLUDED.delivery_point_postcode,
			delivery_point_city = EXCLUDED.delivery_point_city,
			invoice_fullname = EXCLUDED.invoice_fullname,
			invoice_company = EXCLUDED.invoice_company,
			invoice_nip = EXCLUDED.invoice_nip,
			invoice_address = EXCLUDED.invoice_address,
			invoice_city = EXCLUDED.invoice_city,
			invoice_state = EXCLUDED.invoice_state,
			invoice_postcode = EXCLUDED.invoice_postcode,
			invoice_country_code = EXCLUDED.invoice_country_code,
			invoice_country = EXCLUDED.invoice_country,
			want_invoice = EXCLUDED.want_invoice,
			extra_field_1 = EXCLUDED.extra_field_1,
			extra_field_2 = EXCLUDED.extra_field_2,
			order_page = EXCLUDED.order_page,
			pick_state = EXCLUDED.pick_state,
			pack_state = EXCLUDED.pack_state,
			star = EXCLUDED.star,
			crm_client_id = EXCLUDED.crm_client_id,
			main_order_product_id = EXCLUDED.main_order_product_id,
			main_product_name = EXCLUDED.main_product_name,
			main_sku = EXCLUDED.main_sku,
			main_asin = EXCLUDED.main_asin,
			main_price_brutto = EXCLUDED.main_price_brutto,
			main_tax_rate = EXCLUDED.main_tax_rate,
			main_quantity = EXCLUDED.main_quantity,
			updated_at = NOW()
		RETURNING (xmax = 0) AS inserted
	`

	var inserted bool
	err := tx.QueryRow(ctx, query,
		order.AmazonOrderID, order.BaseLinkerOrderID, order.ShopOrderID,
		order.OrderSource, order.OrderSourceID, order.OrderSourceInfo,
		order.OrderStatusID, order.Confirmed, order.DateConfirmed, order.DateAdd, order.DateInStatus,
		order.UserLogin, order.Phone, order.Email, order.UserComments, order.AdminComments,
		order.Currency, order.PaymentMethod, order.PaymentMethodCOD, order.PaymentDone,
		order.DeliveryMethodID, order.DeliveryMethod, order.DeliveryPrice,
		order.DeliveryPackageModule, order.DeliveryPackageNr,
		order.DeliveryFullname, order.DeliveryCompany, order.DeliveryAddress,
		order.DeliveryCity, order.DeliveryState, order.DeliveryPostcode,
		order.DeliveryCountryCode, order.DeliveryCountry,
		order.DeliveryPointID, order.DeliveryPointName, order.DeliveryPointAddress,
		order.DeliveryPointPostcode, order.DeliveryPointCity,
		order.InvoiceFullname, order.InvoiceCompany, order.InvoiceNip, order.InvoiceAddress,
		order.InvoiceCity, order.InvoiceState, order.InvoicePostcode,
		order.InvoiceCountryCode, order.InvoiceCountry, order.WantInvoice,
		order.ExtraField1, order.ExtraField2, order.OrderPage,
		order.PickState, order.PackState, order.Star, order.CRMClientID,
		order.MainOrderProductID, order.MainProductName, order.MainSKU, order.MainASIN,
		order.MainPriceBrutto, order.MainTaxRate, order.MainQuantity,
		order.DefaultWidthInInches, order.DefaultLengthInInches,
		order.DefaultWidthInMM, order.DefaultLengthInMM,
		order.Priority, order.IsRound, order.OrderStatus,
		nil, // raw_payload will be set from original JSON
	).Scan(&inserted)

	if err != nil {
		return false, fmt.Errorf("insert amazon_orders failed for %s: %w", order.AmazonOrderID, err)
	}

	return inserted, nil
}

func (r *OrderRepository) upsertProductTx(ctx context.Context, tx pgx.Tx, product *models.OrderProduct) (bool, error) {
	query := `
		INSERT INTO amazon_order_products (
			order_product_id, amazon_order_id,
			storage, storage_id, product_id, variant_id,
			name, attributes, sku, ean, location,
			warehouse_id, auction_id,
			price_brutto, thickness, tax_rate, quantity, weight,
			default_width_in_inches, default_length_in_inches,
			customer_width_in_inches, customer_length_in_inches,
			default_width_in_mm, default_length_in_mm,
			customer_width_in_mm, customer_length_in_mm,
			corner_radius_and_notes,
			is_round,
			bundle_id, is_discount_line
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30
		)
		ON CONFLICT (order_product_id) DO UPDATE SET
			amazon_order_id = EXCLUDED.amazon_order_id,
			storage = EXCLUDED.storage,
			storage_id = EXCLUDED.storage_id,
			product_id = EXCLUDED.product_id,
			variant_id = EXCLUDED.variant_id,
			name = EXCLUDED.name,
			attributes = EXCLUDED.attributes,
			sku = EXCLUDED.sku,
			ean = EXCLUDED.ean,
			location = EXCLUDED.location,
			warehouse_id = EXCLUDED.warehouse_id,
			auction_id = EXCLUDED.auction_id,
			price_brutto = EXCLUDED.price_brutto,
			thickness = EXCLUDED.thickness,
			tax_rate = EXCLUDED.tax_rate,
			quantity = EXCLUDED.quantity,
			weight = EXCLUDED.weight,
			default_width_in_inches = EXCLUDED.default_width_in_inches,
			default_length_in_inches = EXCLUDED.default_length_in_inches,
			default_width_in_mm = EXCLUDED.default_width_in_mm,
			default_length_in_mm = EXCLUDED.default_length_in_mm,
			is_round = EXCLUDED.is_round,
			bundle_id = EXCLUDED.bundle_id,
			is_discount_line = EXCLUDED.is_discount_line,
			updated_at = NOW()
	`

	tag, err := tx.Exec(ctx, query,
		product.OrderProductID, product.AmazonOrderID,
		product.Storage, product.StorageID, product.ProductID, product.VariantID,
		product.Name, product.Attributes, product.SKU, product.EAN, product.Location,
		product.WarehouseID, product.AuctionID,
		product.PriceBrutto, product.Thickness, product.TaxRate, product.Quantity, product.Weight,
		product.DefaultWidthInInches, product.DefaultLengthInInches,
		product.CustomerWidthInInches, product.CustomerLengthInInches,
		product.DefaultWidthInMM, product.DefaultLengthInMM,
		product.CustomerWidthInMM, product.CustomerLengthInMM,
		product.CornerRadiusAndNotes,
		product.IsRound,
		product.BundleID, product.IsDiscountLine,
	)

	if err != nil {
		return false, fmt.Errorf("insert amazon_order_products failed for order_product_id=%d amazon_order_id=%s: %w", product.OrderProductID, product.AmazonOrderID, err)
	}

	return tag.RowsAffected() > 0, nil
}

// ListOrders returns paginated orders with filters
func (r *OrderRepository) ListOrders(ctx context.Context, filters map[string]interface{}, page, limit int) ([]models.AmazonOrder, int, error) {
	log.Printf("📋 Repository list amazon orders started (page=%d limit=%d filters=%d)", page, limit, len(filters))
	offset := (page - 1) * limit

	whereConditions := []string{}
	productWhereConditions := []string{}
	productJoinConditions := []string{"o.amazon_order_id = p.amazon_order_id"}
	args := []interface{}{}
	argPos := 1
	addProductCondition := func(filterCondition string, joinCondition string) {
		productWhereConditions = append(productWhereConditions, filterCondition)
		productJoinConditions = append(productJoinConditions, joinCondition)
	}

	// Build WHERE clause
	if val, ok := filters["amazon_order_id"].(string); ok && val != "" {
		addILikeCondition(&whereConditions, &args, &argPos, "o.amazon_order_id", val)
	}

	if val, ok := filters["baselinker_order_id"].(int64); ok && val > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf("o.baselinker_order_id = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["order_status_id"].(int64); ok && val > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf("o.order_status_id = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["confirmed"].(bool); ok {
		whereConditions = append(whereConditions, fmt.Sprintf("o.confirmed = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["sku"].(string); ok && val != "" {
		addProductCondition(
			fmt.Sprintf("p_filter.sku ILIKE $%d", argPos),
			fmt.Sprintf("p.sku ILIKE $%d", argPos),
		)
		args = append(args, wildcardPattern(val))
		argPos++
	}

	if val, ok := filters["thickness"].(string); ok && val != "" {
		addProductCondition(
			fmt.Sprintf("REPLACE(LOWER(COALESCE(p_filter.thickness, '')), ' ', '') ILIKE $%d", argPos),
			fmt.Sprintf("REPLACE(LOWER(COALESCE(p.thickness, '')), ' ', '') ILIKE $%d", argPos),
		)
		args = append(args, "%"+strings.ReplaceAll(strings.ToLower(strings.TrimSpace(val)), " ", "")+"%")
		argPos++
	}

	if val, ok := filters["phone"].(string); ok && val != "" {
		addILikeCondition(&whereConditions, &args, &argPos, "o.phone", val)
	}

	if val, ok := filters["mobile"].(string); ok && val != "" {
		addILikeCondition(&whereConditions, &args, &argPos, "o.phone", val)
	}

	if val, ok := filters["customer"].(string); ok && val != "" {
		pattern := wildcardPattern(val)
		whereConditions = append(whereConditions, fmt.Sprintf("(o.delivery_fullname ILIKE $%d OR o.user_login ILIKE $%d)", argPos, argPos))
		args = append(args, pattern)
		argPos++
	}

	if val, ok := filters["quantity"].(float64); ok {
		operator := "="
		if rawOperator, ok := filters["quantity_operator"].(string); ok {
			switch rawOperator {
			case "gt":
				operator = ">"
			case "gte":
				operator = ">="
			case "lt":
				operator = "<"
			case "lte":
				operator = "<="
			}
		}
		addProductCondition(
			fmt.Sprintf("p_filter.quantity %s $%d", operator, argPos),
			fmt.Sprintf("p.quantity %s $%d", operator, argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["default_width_in_inches"].(float64); ok {
		operator := "="
		if rawOperator, ok := filters["default_width_in_inches_operator"].(string); ok {
			switch rawOperator {
			case "gt":
				operator = ">"
			case "gte":
				operator = ">="
			case "lt":
				operator = "<"
			case "lte":
				operator = "<="
			}
		}
		addProductCondition(
			fmt.Sprintf("p_filter.default_width_in_inches %s $%d", operator, argPos),
			fmt.Sprintf("p.default_width_in_inches %s $%d", operator, argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["default_length_in_inches"].(float64); ok {
		operator := "="
		if rawOperator, ok := filters["default_length_in_inches_operator"].(string); ok {
			switch rawOperator {
			case "gt":
				operator = ">"
			case "gte":
				operator = ">="
			case "lt":
				operator = "<"
			case "lte":
				operator = "<="
			}
		}
		addProductCondition(
			fmt.Sprintf("p_filter.default_length_in_inches %s $%d", operator, argPos),
			fmt.Sprintf("p.default_length_in_inches %s $%d", operator, argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["delivery_city"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("o.delivery_city ILIKE $%d", argPos))
		args = append(args, "%"+val+"%")
		argPos++
	}

	if val, ok := filters["delivery_state"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("o.delivery_state = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["return_initiated"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("p_filter.return_initiated = $%d", argPos),
			fmt.Sprintf("p.return_initiated = $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["return_initiated_exclude"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("COALESCE(p_filter.return_initiated, FALSE) <> $%d", argPos),
			fmt.Sprintf("COALESCE(p.return_initiated, FALSE) <> $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["return_initiated_compromised"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("p_filter.return_initiated_compromised = $%d", argPos),
			fmt.Sprintf("p.return_initiated_compromised = $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["other_issues"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("p_filter.other_issues = $%d", argPos),
			fmt.Sprintf("p.other_issues = $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["other_issues_exclude"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("COALESCE(p_filter.other_issues, FALSE) <> $%d", argPos),
			fmt.Sprintf("COALESCE(p.other_issues, FALSE) <> $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["safety_claimed"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("p_filter.safety_claimed = $%d", argPos),
			fmt.Sprintf("p.safety_claimed = $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["safety_claimed_exclude"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("COALESCE(p_filter.safety_claimed, FALSE) <> $%d", argPos),
			fmt.Sprintf("COALESCE(p.safety_claimed, FALSE) <> $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["priority"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("o.priority = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["order_status"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("o.order_status = $%d", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["round_product"].(bool); ok {
		addProductCondition(
			fmt.Sprintf("p_filter.is_round = $%d", argPos),
			fmt.Sprintf("p.is_round = $%d", argPos),
		)
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["missing_customer_inputs"].(bool); ok && val {
		addProductCondition(
			`p_filter.customer_width_in_inches IS NULL
			AND p_filter.customer_length_in_inches IS NULL
			AND p_filter.customer_width_in_mm IS NULL
			AND p_filter.customer_length_in_mm IS NULL
			AND COALESCE(NULLIF(BTRIM(p_filter.corner_radius_and_notes), ''), '') = ''`,
			`p.customer_width_in_inches IS NULL
			AND p.customer_length_in_inches IS NULL
			AND p.customer_width_in_mm IS NULL
			AND p.customer_length_in_mm IS NULL
			AND COALESCE(NULLIF(BTRIM(p.corner_radius_and_notes), ''), '') = ''`,
		)
	}

	if val, ok := filters["has_customer_inputs"].(bool); ok && val {
		addProductCondition(
			`(p_filter.customer_width_in_inches IS NOT NULL
			OR p_filter.customer_length_in_inches IS NOT NULL
			OR COALESCE(NULLIF(BTRIM(p_filter.corner_radius_and_notes), ''), '') <> '')`,
			`(p.customer_width_in_inches IS NOT NULL
			OR p.customer_length_in_inches IS NOT NULL
			OR COALESCE(NULLIF(BTRIM(p.corner_radius_and_notes), ''), '') <> '')`,
		)
	}

	if val, ok := filters["date_from"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("COALESCE(o.date_confirmed, o.date_add) >= $%d::timestamp", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["date_to"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("COALESCE(o.date_confirmed, o.date_add) < $%d::timestamp", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["confirmed_date_from"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("COALESCE(o.date_confirmed, o.date_add) >= $%d::timestamp", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["confirmed_date_to"].(string); ok && val != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("COALESCE(o.date_confirmed, o.date_add) < $%d::timestamp", argPos))
		args = append(args, val)
		argPos++
	}

	if val, ok := filters["search"].(string); ok && val != "" {
		searchCondition := fmt.Sprintf(`(
			o.amazon_order_id ILIKE $%d OR
			o.phone ILIKE $%d OR
			o.user_login ILIKE $%d OR
			o.delivery_fullname ILIKE $%d OR
			o.delivery_city ILIKE $%d OR
			EXISTS (
				SELECT 1 FROM amazon_order_products p_filter
				WHERE p_filter.amazon_order_id = o.amazon_order_id
				AND (p_filter.sku ILIKE $%d OR p_filter.name ILIKE $%d)
			)
			)`, argPos, argPos, argPos, argPos, argPos, argPos, argPos)
		whereConditions = append(whereConditions, searchCondition)
		args = append(args, "%"+val+"%")
		argPos++
	}

	if len(productWhereConditions) > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM amazon_order_products p_filter
			WHERE p_filter.amazon_order_id = o.amazon_order_id
			AND %s
		)`, strings.Join(productWhereConditions, " AND ")))
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}
	productJoinClause := strings.Join(productJoinConditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM amazon_orders o %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Get orders with products
	query := fmt.Sprintf(`
		SELECT 
			o.amazon_order_id, o.baselinker_order_id, o.shop_order_id,
			o.order_source, o.order_source_id, o.order_source_info,
			o.order_status_id, o.confirmed, o.date_confirmed, o.date_add, o.date_in_status,
			o.user_login, o.phone, o.email, o.user_comments, o.admin_comments,
			o.currency, o.payment_method, o.payment_method_cod, o.payment_done,
			o.delivery_method_id, o.delivery_method, o.delivery_price,
			o.delivery_fullname, o.delivery_city, o.delivery_state, o.delivery_postcode,
			o.main_sku, o.main_product_name,
			o.default_width_in_inches, o.default_length_in_inches,
			o.default_width_in_mm, o.default_length_in_mm,
			o.customer_width_in_mm, o.customer_length_in_mm,
			o.corner_radius_and_notes, o.is_round,
			o.priority, o.order_status, o.order_status_updated_at, o.internal_notes, o.updated_by,
			COALESCE(
				json_agg(
					json_build_object(
						'order_product_id', p.order_product_id,
						'amazon_order_id', p.amazon_order_id,
						'name', p.name,
						'sku', p.sku,
						'auction_id', p.auction_id,
						'price_brutto', p.price_brutto,
						'thickness', p.thickness,
						'quantity', p.quantity,
						'default_width_in_inches', p.default_width_in_inches,
						'default_length_in_inches', p.default_length_in_inches,
						'customer_width_in_inches', p.customer_width_in_inches,
						'customer_length_in_inches', p.customer_length_in_inches,
						'default_width_in_mm', p.default_width_in_mm,
						'default_length_in_mm', p.default_length_in_mm,
						'customer_width_in_mm', p.customer_width_in_mm,
						'customer_length_in_mm', p.customer_length_in_mm,
						'corner_radius_and_notes', p.corner_radius_and_notes,
						'safety_claimed', p.safety_claimed,
						'safety_claimed_updated_at', p.safety_claimed_updated_at,
						'safety_claim_issues', p.safety_claim_issues,
						'return_initiated', p.return_initiated,
						'return_initiated_updated_at', p.return_initiated_updated_at,
						'return_initiated_reason', p.return_initiated_reason,
						'return_initiated_followup_action', p.return_initiated_followup_action,
						'return_initiated_compromised', p.return_initiated_compromised,
						'return_initiated_compromised_reason', p.return_initiated_compromised_reason,
						'return_initiated_compromised_updated_at', p.return_initiated_compromised_updated_at,
						'other_issues', p.other_issues,
						'other_issues_reason', p.other_issues_reason,
						'other_issue_updated_at', p.other_issue_updated_at,
						'is_round', p.is_round,
						'is_discount_line', p.is_discount_line,
						'updated_by', p.updated_by
					) ORDER BY p.order_product_id
				) FILTER (WHERE p.order_product_id IS NOT NULL),
				'[]'
			) as products
		FROM amazon_orders o
		LEFT JOIN amazon_order_products p ON %s
		%s
		GROUP BY o.amazon_order_id
		ORDER BY o.date_confirmed DESC NULLS LAST, o.date_add DESC
		LIMIT $%d OFFSET $%d
	`, productJoinClause, whereClause, argPos, argPos+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	orders := []models.AmazonOrder{}
	for rows.Next() {
		var order models.AmazonOrder
		var productsJSON []byte

		err := rows.Scan(
			&order.AmazonOrderID, &order.BaseLinkerOrderID, &order.ShopOrderID,
			&order.OrderSource, &order.OrderSourceID, &order.OrderSourceInfo,
			&order.OrderStatusID, &order.Confirmed, &order.DateConfirmed, &order.DateAdd, &order.DateInStatus,
			&order.UserLogin, &order.Phone, &order.Email, &order.UserComments, &order.AdminComments,
			&order.Currency, &order.PaymentMethod, &order.PaymentMethodCOD, &order.PaymentDone,
			&order.DeliveryMethodID, &order.DeliveryMethod, &order.DeliveryPrice,
			&order.DeliveryFullname, &order.DeliveryCity, &order.DeliveryState, &order.DeliveryPostcode,
			&order.MainSKU, &order.MainProductName,
			&order.DefaultWidthInInches, &order.DefaultLengthInInches,
			&order.DefaultWidthInMM, &order.DefaultLengthInMM,
			&order.CustomerWidthInMM, &order.CustomerLengthInMM,
			&order.CornerRadiusAndNotes, &order.IsRound,
			&order.Priority, &order.OrderStatus, &order.OrderStatusUpdatedAt, &order.InternalNotes, &order.UpdatedBy,
			&productsJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}

		// Parse list products via a JSON-friendly shape, then map into sql.Null* fields.
		var productItems []orderProductListItem
		if err := json.Unmarshal(productsJSON, &productItems); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal products: %w", err)
		}
		order.Products = make([]models.OrderProduct, 0, len(productItems))
		for _, item := range productItems {
			product := models.OrderProduct{
				OrderProductID: item.OrderProductID,
				AmazonOrderID:  item.AmazonOrderID,
				IsDiscountLine: item.IsDiscountLine,
			}
			if item.Name != nil {
				product.Name = sql.NullString{String: *item.Name, Valid: true}
			}
			if item.SKU != nil {
				product.SKU = sql.NullString{String: *item.SKU, Valid: true}
			}
			if item.AuctionID != nil {
				product.AuctionID = sql.NullString{String: *item.AuctionID, Valid: true}
			}
			if item.PriceBrutto != nil {
				product.PriceBrutto = sql.NullFloat64{Float64: *item.PriceBrutto, Valid: true}
			}
			if item.Thickness != nil {
				product.Thickness = sql.NullString{String: *item.Thickness, Valid: true}
			}
			if item.Quantity != nil {
				product.Quantity = sql.NullFloat64{Float64: *item.Quantity, Valid: true}
			}
			if item.DefaultWidthInInches != nil {
				product.DefaultWidthInInches = sql.NullFloat64{Float64: *item.DefaultWidthInInches, Valid: true}
			}
			if item.DefaultLengthInInches != nil {
				product.DefaultLengthInInches = sql.NullFloat64{Float64: *item.DefaultLengthInInches, Valid: true}
			}
			if item.CustomerWidthInInches != nil {
				product.CustomerWidthInInches = sql.NullFloat64{Float64: *item.CustomerWidthInInches, Valid: true}
			}
			if item.CustomerLengthInInches != nil {
				product.CustomerLengthInInches = sql.NullFloat64{Float64: *item.CustomerLengthInInches, Valid: true}
			}
			if item.DefaultWidthInMM != nil {
				product.DefaultWidthInMM = sql.NullFloat64{Float64: *item.DefaultWidthInMM, Valid: true}
			}
			if item.DefaultLengthInMM != nil {
				product.DefaultLengthInMM = sql.NullFloat64{Float64: *item.DefaultLengthInMM, Valid: true}
			}
			if item.CustomerWidthInMM != nil {
				product.CustomerWidthInMM = sql.NullFloat64{Float64: *item.CustomerWidthInMM, Valid: true}
			}
			if item.CustomerLengthInMM != nil {
				product.CustomerLengthInMM = sql.NullFloat64{Float64: *item.CustomerLengthInMM, Valid: true}
			}
			if item.CornerRadiusAndNotes != nil {
				product.CornerRadiusAndNotes = sql.NullString{String: *item.CornerRadiusAndNotes, Valid: true}
			}
			if item.SafetyClaimed != nil {
				product.SafetyClaimed = sql.NullBool{Bool: *item.SafetyClaimed, Valid: true}
			}
			if item.SafetyClaimedUpdatedAt != nil {
				if parsed, err := time.Parse(time.RFC3339, *item.SafetyClaimedUpdatedAt); err == nil {
					product.SafetyClaimedUpdatedAt = sql.NullTime{Time: parsed, Valid: true}
				}
			}
			if item.SafetyClaimIssues != nil {
				product.SafetyClaimIssues = sql.NullString{String: *item.SafetyClaimIssues, Valid: true}
			}
			if item.ReturnInitiated != nil {
				product.ReturnInitiated = sql.NullBool{Bool: *item.ReturnInitiated, Valid: true}
			}
			if item.ReturnInitiatedUpdatedAt != nil {
				if parsed, err := time.Parse(time.RFC3339, *item.ReturnInitiatedUpdatedAt); err == nil {
					product.ReturnInitiatedUpdatedAt = sql.NullTime{Time: parsed, Valid: true}
				}
			}
			if item.ReturnInitiatedReason != nil {
				product.ReturnInitiatedReason = sql.NullString{String: *item.ReturnInitiatedReason, Valid: true}
			}
			if item.ReturnInitiatedFollowupAction != nil {
				product.ReturnInitiatedFollowupAction = sql.NullString{String: *item.ReturnInitiatedFollowupAction, Valid: true}
			}
			if item.ReturnInitiatedCompromised != nil {
				product.ReturnInitiatedCompromised = sql.NullBool{Bool: *item.ReturnInitiatedCompromised, Valid: true}
			}
			if item.ReturnInitiatedCompromisedReason != nil {
				product.ReturnInitiatedCompromisedReason = sql.NullString{String: *item.ReturnInitiatedCompromisedReason, Valid: true}
			}
			if item.ReturnInitiatedCompromisedUpdatedAt != nil {
				if parsed, err := time.Parse(time.RFC3339, *item.ReturnInitiatedCompromisedUpdatedAt); err == nil {
					product.ReturnInitiatedCompromisedUpdatedAt = sql.NullTime{Time: parsed, Valid: true}
				}
			}
			if item.OtherIssues != nil {
				product.OtherIssues = sql.NullBool{Bool: *item.OtherIssues, Valid: true}
			}
			if item.OtherIssuesReason != nil {
				product.OtherIssuesReason = sql.NullString{String: *item.OtherIssuesReason, Valid: true}
			}
			if item.OtherIssueUpdatedAt != nil {
				if parsed, err := time.Parse(time.RFC3339, *item.OtherIssueUpdatedAt); err == nil {
					product.OtherIssueUpdatedAt = sql.NullTime{Time: parsed, Valid: true}
				}
			}
			if item.UpdatedBy != nil {
				product.UpdatedBy = sql.NullString{String: *item.UpdatedBy, Valid: true}
			}
			product.IsRound = item.IsRound
			order.Products = append(order.Products, product)
		}
		if err := r.backfillDerivedDimensions(ctx, &order); err != nil {
			log.Printf("⚠️  Failed to backfill derived dimensions during list for order %s: %v", order.AmazonOrderID, err)
		}

		orders = append(orders, order)
	}
	log.Printf("✅ Repository list amazon orders completed: returned=%d total=%d", len(orders), total)

	return orders, total, nil
}

// GetOrderByID returns a single order with products
func (r *OrderRepository) GetOrderByID(ctx context.Context, amazonOrderID string) (*models.AmazonOrder, error) {
	log.Printf("🔎 Repository get amazon order by id=%s", amazonOrderID)
	query := `
		SELECT 
			amazon_order_id, baselinker_order_id, shop_order_id,
			order_source, order_source_id, order_source_info,
			order_status_id, confirmed, date_confirmed, date_add, date_in_status,
			user_login, phone, email, user_comments, admin_comments,
			currency, payment_method, payment_method_cod, payment_done,
			delivery_method_id, delivery_method, delivery_price,
			delivery_package_module, delivery_package_nr,
			delivery_fullname, delivery_company, delivery_address,
			delivery_city, delivery_state, delivery_postcode,
			delivery_country_code, delivery_country,
			delivery_point_id, delivery_point_name, delivery_point_address,
			delivery_point_postcode, delivery_point_city,
			invoice_fullname, invoice_company, invoice_nip, invoice_address,
			invoice_city, invoice_state, invoice_postcode,
			invoice_country_code, invoice_country, want_invoice,
			extra_field_1, extra_field_2, order_page,
			pick_state, pack_state, star, crm_client_id,
			main_order_product_id, main_product_name, main_sku, main_asin,
			main_price_brutto, main_tax_rate, main_quantity,
			default_width_in_inches, default_length_in_inches,
			default_width_in_mm, default_length_in_mm,
			customer_width_in_mm, customer_length_in_mm,
			corner_radius_and_notes,
			internal_notes, priority, order_status, order_status_updated_at, is_round, updated_by, created_at, updated_at
		FROM amazon_orders
		WHERE amazon_order_id = $1
	`

	var order models.AmazonOrder
	err := r.pool.QueryRow(ctx, query, amazonOrderID).Scan(
		&order.AmazonOrderID, &order.BaseLinkerOrderID, &order.ShopOrderID,
		&order.OrderSource, &order.OrderSourceID, &order.OrderSourceInfo,
		&order.OrderStatusID, &order.Confirmed, &order.DateConfirmed, &order.DateAdd, &order.DateInStatus,
		&order.UserLogin, &order.Phone, &order.Email, &order.UserComments, &order.AdminComments,
		&order.Currency, &order.PaymentMethod, &order.PaymentMethodCOD, &order.PaymentDone,
		&order.DeliveryMethodID, &order.DeliveryMethod, &order.DeliveryPrice,
		&order.DeliveryPackageModule, &order.DeliveryPackageNr,
		&order.DeliveryFullname, &order.DeliveryCompany, &order.DeliveryAddress,
		&order.DeliveryCity, &order.DeliveryState, &order.DeliveryPostcode,
		&order.DeliveryCountryCode, &order.DeliveryCountry,
		&order.DeliveryPointID, &order.DeliveryPointName, &order.DeliveryPointAddress,
		&order.DeliveryPointPostcode, &order.DeliveryPointCity,
		&order.InvoiceFullname, &order.InvoiceCompany, &order.InvoiceNip, &order.InvoiceAddress,
		&order.InvoiceCity, &order.InvoiceState, &order.InvoicePostcode,
		&order.InvoiceCountryCode, &order.InvoiceCountry, &order.WantInvoice,
		&order.ExtraField1, &order.ExtraField2, &order.OrderPage,
		&order.PickState, &order.PackState, &order.Star, &order.CRMClientID,
		&order.MainOrderProductID, &order.MainProductName, &order.MainSKU, &order.MainASIN,
		&order.MainPriceBrutto, &order.MainTaxRate, &order.MainQuantity,
		&order.DefaultWidthInInches, &order.DefaultLengthInInches,
		&order.DefaultWidthInMM, &order.DefaultLengthInMM,
		&order.CustomerWidthInMM, &order.CustomerLengthInMM,
		&order.CornerRadiusAndNotes,
		&order.InternalNotes, &order.Priority, &order.OrderStatus, &order.OrderStatusUpdatedAt, &order.IsRound, &order.UpdatedBy, &order.CreatedAt, &order.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Get products
	products, err := r.getProductsByOrderID(ctx, amazonOrderID)
	if err != nil {
		return nil, err
	}
	order.Products = products
	if err := r.backfillDerivedDimensions(ctx, &order); err != nil {
		log.Printf("⚠️  Failed to backfill derived dimensions for order %s: %v", amazonOrderID, err)
	}
	log.Printf("✅ Repository get amazon order completed (amazon_order_id=%s products=%d)", amazonOrderID, len(order.Products))

	return &order, nil
}

func (r *OrderRepository) GetChangedOrderIDsByIDsSince(ctx context.Context, amazonOrderIDs []string, since time.Time, until time.Time) ([]string, []string, error) {
	query := `
		WITH requested AS (
			SELECT DISTINCT ON (amazon_order_id)
				amazon_order_id,
				ord
			FROM (
				SELECT NULLIF(BTRIM(id), '') AS amazon_order_id, ord
				FROM unnest($1::text[]) WITH ORDINALITY AS t(id, ord)
			) normalized
			WHERE amazon_order_id IS NOT NULL
			ORDER BY amazon_order_id, ord
		),
		order_updates AS (
			SELECT
				r.amazon_order_id,
				r.ord,
				o.amazon_order_id IS NULL AS is_missing,
				GREATEST(
					COALESCE(o.updated_at, to_timestamp(0)),
					COALESCE(MAX(p.updated_at), COALESCE(o.updated_at, to_timestamp(0)))
				) AS latest_updated_at
			FROM requested r
			LEFT JOIN amazon_orders o ON o.amazon_order_id = r.amazon_order_id
			LEFT JOIN amazon_order_products p ON p.amazon_order_id = o.amazon_order_id
			GROUP BY r.amazon_order_id, r.ord, o.amazon_order_id, o.updated_at
		)
		SELECT amazon_order_id, is_missing, latest_updated_at
		FROM order_updates
		WHERE is_missing OR (latest_updated_at > $2 AND latest_updated_at <= $3)
		ORDER BY ord
	`

	rows, err := r.pool.Query(ctx, query, amazonOrderIDs, since, until)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query changed order ids: %w", err)
	}
	defer rows.Close()

	changedOrderIDs := make([]string, 0)
	missingOrderIDs := make([]string, 0)

	for rows.Next() {
		var amazonOrderID string
		var isMissing bool
		var latestUpdatedAt time.Time

		if err := rows.Scan(&amazonOrderID, &isMissing, &latestUpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("failed to scan changed order row: %w", err)
		}

		if isMissing {
			missingOrderIDs = append(missingOrderIDs, amazonOrderID)
			continue
		}

		changedOrderIDs = append(changedOrderIDs, amazonOrderID)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate changed order rows: %w", err)
	}

	return changedOrderIDs, missingOrderIDs, nil
}

func (r *OrderRepository) backfillDerivedDimensions(ctx context.Context, order *models.AmazonOrder) error {
	if !order.MainSKU.Valid || order.MainSKU.String == "" {
		return nil
	}

	if order.DefaultWidthInInches.Valid &&
		order.DefaultLengthInInches.Valid &&
		order.DefaultWidthInMM.Valid &&
		order.DefaultLengthInMM.Valid {
		return nil
	}

	skuData, found := utils.GetSKUMapper().GetBySKU(order.MainSKU.String)
	if !found {
		log.Printf("ℹ️  No SKU mapper dimensions found for order %s main_sku=%s", order.AmazonOrderID, order.MainSKU.String)
		return nil
	}

	log.Printf(
		"🔧 Backfilling missing derived dimensions for order %s from SKU %s (width_in=%.2f length_in=%.2f width_mm=%.2f length_mm=%.2f is_round=%v)",
		order.AmazonOrderID,
		order.MainSKU.String,
		skuData.WidthInInches,
		skuData.LengthInInches,
		skuData.WidthInMM,
		skuData.LengthInMM,
		skuData.IsRound,
	)

	order.DefaultWidthInInches = sql.NullFloat64{Float64: skuData.WidthInInches, Valid: true}
	order.DefaultLengthInInches = sql.NullFloat64{Float64: skuData.LengthInInches, Valid: true}
	order.DefaultWidthInMM = sql.NullFloat64{Float64: skuData.WidthInMM, Valid: true}
	order.DefaultLengthInMM = sql.NullFloat64{Float64: skuData.LengthInMM, Valid: true}
	order.IsRound = skuData.IsRound

	const query = `
		UPDATE amazon_orders
		SET default_width_in_inches = $2,
			default_length_in_inches = $3,
			default_width_in_mm = $4,
			default_length_in_mm = $5,
			is_round = $6,
			updated_at = NOW()
		WHERE amazon_order_id = $1
	`

	if _, err := r.pool.Exec(
		ctx,
		query,
		order.AmazonOrderID,
		skuData.WidthInInches,
		skuData.LengthInInches,
		skuData.WidthInMM,
		skuData.LengthInMM,
		skuData.IsRound,
	); err != nil {
		return fmt.Errorf("persisting backfilled dimensions failed: %w", err)
	}

	log.Printf("✅ Backfilled derived dimensions for order %s from SKU %s", order.AmazonOrderID, order.MainSKU.String)
	return nil
}

func (r *OrderRepository) getProductsByOrderID(ctx context.Context, amazonOrderID string) ([]models.OrderProduct, error) {
	query := `
		SELECT 
			order_product_id, amazon_order_id,
			storage, storage_id, product_id, variant_id,
			name, attributes, sku, ean, location,
			warehouse_id, auction_id,
			price_brutto, thickness, tax_rate, quantity,
			default_width_in_inches, default_length_in_inches,
			customer_width_in_inches, customer_length_in_inches,
			default_width_in_mm, default_length_in_mm,
			customer_width_in_mm, customer_length_in_mm,
			corner_radius_and_notes,
			safety_claimed, safety_claimed_updated_at, safety_claim_issues,
			return_initiated, return_initiated_updated_at, return_initiated_reason, return_initiated_followup_action,
			return_initiated_compromised, return_initiated_compromised_reason, return_initiated_compromised_updated_at,
			other_issues, other_issues_reason, other_issue_updated_at,
			is_round,
			weight,
			bundle_id, is_discount_line, updated_by, created_at, updated_at
		FROM amazon_order_products
		WHERE amazon_order_id = $1
		ORDER BY order_product_id
	`

	rows, err := r.pool.Query(ctx, query, amazonOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	products := []models.OrderProduct{}
	for rows.Next() {
		var product models.OrderProduct
		err := rows.Scan(
			&product.OrderProductID, &product.AmazonOrderID,
			&product.Storage, &product.StorageID, &product.ProductID, &product.VariantID,
			&product.Name, &product.Attributes, &product.SKU, &product.EAN, &product.Location,
			&product.WarehouseID, &product.AuctionID,
			&product.PriceBrutto, &product.Thickness, &product.TaxRate, &product.Quantity,
			&product.DefaultWidthInInches, &product.DefaultLengthInInches,
			&product.CustomerWidthInInches, &product.CustomerLengthInInches,
			&product.DefaultWidthInMM, &product.DefaultLengthInMM,
			&product.CustomerWidthInMM, &product.CustomerLengthInMM,
			&product.CornerRadiusAndNotes,
			&product.SafetyClaimed, &product.SafetyClaimedUpdatedAt, &product.SafetyClaimIssues,
			&product.ReturnInitiated, &product.ReturnInitiatedUpdatedAt, &product.ReturnInitiatedReason, &product.ReturnInitiatedFollowupAction,
			&product.ReturnInitiatedCompromised, &product.ReturnInitiatedCompromisedReason, &product.ReturnInitiatedCompromisedUpdatedAt,
			&product.OtherIssues, &product.OtherIssuesReason, &product.OtherIssueUpdatedAt,
			&product.IsRound,
			&product.Weight,
			&product.BundleID, &product.IsDiscountLine, &product.UpdatedBy, &product.CreatedAt, &product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// UpdateManualFields updates only manual business fields
func (r *OrderRepository) UpdateManualFields(ctx context.Context, amazonOrderID string, req *models.UpdateManualFieldsRequest, actor string) (*models.AmazonOrder, error) {
	log.Printf("🛠️  Repository manual field update started (amazon_order_id=%s)", amazonOrderID)
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.DefaultWidthInInches != nil {
		updates = append(updates, fmt.Sprintf("default_width_in_inches = $%d", argPos))
		args = append(args, req.DefaultWidthInInches)
		argPos++
	}

	if req.DefaultLengthInInches != nil {
		updates = append(updates, fmt.Sprintf("default_length_in_inches = $%d", argPos))
		args = append(args, req.DefaultLengthInInches)
		argPos++
	}

	if req.DefaultWidthInMM != nil {
		updates = append(updates, fmt.Sprintf("default_width_in_mm = $%d", argPos))
		args = append(args, req.DefaultWidthInMM)
		argPos++
	}

	if req.DefaultLengthInMM != nil {
		updates = append(updates, fmt.Sprintf("default_length_in_mm = $%d", argPos))
		args = append(args, req.DefaultLengthInMM)
		argPos++
	}

	if req.CustomerWidthInMM != nil {
		updates = append(updates, fmt.Sprintf("customer_width_in_mm = $%d", argPos))
		args = append(args, req.CustomerWidthInMM)
		argPos++
	}

	if req.CustomerLengthInMM != nil {
		updates = append(updates, fmt.Sprintf("customer_length_in_mm = $%d", argPos))
		args = append(args, req.CustomerLengthInMM)
		argPos++
	}

	if req.CornerRadiusAndNotes != nil {
		updates = append(updates, fmt.Sprintf("corner_radius_and_notes = $%d", argPos))
		args = append(args, req.CornerRadiusAndNotes)
		argPos++
	}

	if req.InternalNotes != nil {
		updates = append(updates, fmt.Sprintf("internal_notes = $%d", argPos))
		args = append(args, req.InternalNotes)
		argPos++
	}

	if req.Priority != nil {
		updates = append(updates, fmt.Sprintf("priority = $%d", argPos))
		args = append(args, req.Priority)
		argPos++
	}

	if req.OrderStatus != nil {
		updates = append(updates, fmt.Sprintf("order_status = $%d", argPos))
		args = append(args, req.OrderStatus)
		argPos++
		updates = append(updates, "order_status_updated_at = NOW()")
	}

	if req.IsRound != nil {
		updates = append(updates, fmt.Sprintf("is_round = $%d", argPos))
		args = append(args, req.IsRound)
		argPos++
	}

	if len(updates) == 0 {
		log.Printf("ℹ️  Repository manual field update skipped: no updates provided (amazon_order_id=%s)", amazonOrderID)
		return r.GetOrderByID(ctx, amazonOrderID)
	}

	query := fmt.Sprintf(`
		UPDATE amazon_orders
		SET %s, updated_by = NULLIF($%d, ''), updated_at = NOW()
		WHERE amazon_order_id = $%d
	`, strings.Join(updates, ", "), argPos, argPos+1)

	args = append(args, actor, amazonOrderID)
	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update manual fields: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil, sql.ErrNoRows
	}
	log.Printf("✅ Repository manual field update completed (amazon_order_id=%s updated_fields=%d)", amazonOrderID, len(updates))

	return r.GetOrderByID(ctx, amazonOrderID)
}

// UpdateProductManualFields updates manual fields for a specific product row.
func (r *OrderRepository) UpdateProductManualFields(ctx context.Context, amazonOrderID string, orderProductID int64, req *models.UpdateProductManualFieldsRequest, actor string) (*models.AmazonOrder, error) {
	log.Printf("🛠️  Repository product manual update started (amazon_order_id=%s order_product_id=%d)", amazonOrderID, orderProductID)
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.DefaultWidthInInches != nil {
		updates = append(updates, fmt.Sprintf("default_width_in_inches = $%d", argPos))
		args = append(args, req.DefaultWidthInInches)
		argPos++
	}
	if req.DefaultLengthInInches != nil {
		updates = append(updates, fmt.Sprintf("default_length_in_inches = $%d", argPos))
		args = append(args, req.DefaultLengthInInches)
		argPos++
	}
	if req.CustomerWidthInInches != nil {
		updates = append(updates, fmt.Sprintf("customer_width_in_inches = $%d", argPos))
		args = append(args, req.CustomerWidthInInches)
		argPos++
	}
	if req.CustomerLengthInInches != nil {
		updates = append(updates, fmt.Sprintf("customer_length_in_inches = $%d", argPos))
		args = append(args, req.CustomerLengthInInches)
		argPos++
	}
	if req.DefaultWidthInMM != nil {
		updates = append(updates, fmt.Sprintf("default_width_in_mm = $%d", argPos))
		args = append(args, req.DefaultWidthInMM)
		argPos++
	}
	if req.DefaultLengthInMM != nil {
		updates = append(updates, fmt.Sprintf("default_length_in_mm = $%d", argPos))
		args = append(args, req.DefaultLengthInMM)
		argPos++
	}
	if req.CustomerWidthInMM != nil {
		updates = append(updates, fmt.Sprintf("customer_width_in_mm = $%d", argPos))
		args = append(args, req.CustomerWidthInMM)
		argPos++
	}
	if req.CustomerLengthInMM != nil {
		updates = append(updates, fmt.Sprintf("customer_length_in_mm = $%d", argPos))
		args = append(args, req.CustomerLengthInMM)
		argPos++
	}
	if req.CornerRadiusAndNotes != nil {
		updates = append(updates, fmt.Sprintf("corner_radius_and_notes = $%d", argPos))
		args = append(args, req.CornerRadiusAndNotes)
		argPos++
	}
	if req.SafetyClaimed != nil {
		updates = append(updates, fmt.Sprintf("safety_claimed = $%d", argPos))
		args = append(args, req.SafetyClaimed)
		argPos++
		updates = append(updates, "safety_claimed_updated_at = NOW()")
	}
	if req.SafetyClaimIssues != nil {
		updates = append(updates, fmt.Sprintf("safety_claim_issues = $%d", argPos))
		args = append(args, req.SafetyClaimIssues)
		argPos++
	}
	if req.ReturnInitiated != nil {
		updates = append(updates, fmt.Sprintf("return_initiated = $%d", argPos))
		args = append(args, req.ReturnInitiated)
		argPos++
		updates = append(updates, "return_initiated_updated_at = NOW()")
	}
	if req.ReturnInitiatedReason != nil {
		updates = append(updates, fmt.Sprintf("return_initiated_reason = $%d", argPos))
		args = append(args, req.ReturnInitiatedReason)
		argPos++
	}
	if req.ReturnInitiatedFollowupAction != nil {
		updates = append(updates, fmt.Sprintf("return_initiated_followup_action = $%d", argPos))
		args = append(args, req.ReturnInitiatedFollowupAction)
		argPos++
	}
	if req.ReturnInitiatedCompromised != nil {
		updates = append(updates, fmt.Sprintf("return_initiated_compromised = $%d", argPos))
		args = append(args, req.ReturnInitiatedCompromised)
		argPos++
		updates = append(updates, "return_initiated_compromised_updated_at = NOW()")
	}
	if req.ReturnInitiatedCompromisedReason != nil {
		updates = append(updates, fmt.Sprintf("return_initiated_compromised_reason = $%d", argPos))
		args = append(args, req.ReturnInitiatedCompromisedReason)
		argPos++
	}
	if req.OtherIssues != nil {
		updates = append(updates, fmt.Sprintf("other_issues = $%d", argPos))
		args = append(args, req.OtherIssues)
		argPos++
		updates = append(updates, "other_issue_updated_at = NOW()")
	}
	if req.OtherIssuesReason != nil {
		updates = append(updates, fmt.Sprintf("other_issues_reason = $%d", argPos))
		args = append(args, req.OtherIssuesReason)
		argPos++
	}
	if req.IsRound != nil {
		updates = append(updates, fmt.Sprintf("is_round = $%d", argPos))
		args = append(args, req.IsRound)
		argPos++
	}

	if len(updates) == 0 {
		return r.GetOrderByID(ctx, amazonOrderID)
	}

	args = append(args, actor, amazonOrderID, orderProductID)
	query := fmt.Sprintf(`
		UPDATE amazon_order_products
		SET %s, updated_by = NULLIF($%d, ''), updated_at = NOW()
		WHERE amazon_order_id = $%d AND order_product_id = $%d
	`, strings.Join(updates, ", "), argPos, argPos+1, argPos+2)

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update product manual fields: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, sql.ErrNoRows
	}

	log.Printf("✅ Repository product manual update completed (amazon_order_id=%s order_product_id=%d updated_fields=%d)", amazonOrderID, orderProductID, len(updates))
	return r.GetOrderByID(ctx, amazonOrderID)
}

// GetLatestOrderIDs returns the most recent N amazon_order_ids from the database,
// ordered by date_confirmed DESC (then date_add DESC). Used to de-duplicate an
// incoming BaseLinker batch in a single round-trip.
func (r *OrderRepository) GetLatestOrderIDs(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	log.Printf("🔎 Repository fetching latest order ids (limit=%d)", limit)

	query := `
		SELECT amazon_order_id
		FROM amazon_orders
		ORDER BY date_confirmed DESC NULLS LAST, date_add DESC NULLS LAST
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest order ids: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan order id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating latest order ids: %w", err)
	}
	log.Printf("✅ Repository fetched latest order ids: count=%d", len(ids))

	return ids, nil
}
