package utils

import (
	"database/sql"
	"strings"
	"time"
)

// UnixToNullTime converts Unix timestamp to sql.NullTime
// If timestamp is 0, returns NULL
func UnixToNullTime(timestamp int64) sql.NullTime {
	if timestamp == 0 {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{
		Time:  time.Unix(timestamp, 0),
		Valid: true,
	}
}

// StringToNullString converts string to sql.NullString
func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// Int64ToNullInt64 converts int64 to sql.NullInt64
func Int64ToNullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

// Float64ToNullFloat64 converts float64 to sql.NullFloat64
func Float64ToNullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

// IsDiscountLine determines if a product is a discount line
func IsDiscountLine(sku, name string, priceBrutto float64) bool {
	if sku == "" {
		return true
	}
	if priceBrutto < 0 {
		return true
	}
	if strings.Contains(strings.ToLower(name), "discount") {
		return true
	}
	return false
}
