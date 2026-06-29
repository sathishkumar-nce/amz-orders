package utils

import (
	"testing"
	"time"
)

func TestUnixToNullTime(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   int64
		expectValid bool
		expectTime  time.Time
	}{
		{
			name:        "Zero timestamp should be NULL",
			timestamp:   0,
			expectValid: false,
		},
		{
			name:        "Positive timestamp should be valid",
			timestamp:   1735689600, // 2026-01-01
			expectValid: true,
			expectTime:  time.Unix(1735689600, 0),
		},
		{
			name:        "Negative timestamp should be valid",
			timestamp:   -1000,
			expectValid: true,
			expectTime:  time.Unix(-1000, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnixToNullTime(tt.timestamp)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected Valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if tt.expectValid && !result.Time.Equal(tt.expectTime) {
				t.Errorf("Expected time %v, got %v", tt.expectTime, result.Time)
			}
		})
	}
}

func TestStringToNullString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectValid bool
		expectValue string
	}{
		{
			name:        "Empty string should be NULL",
			input:       "",
			expectValid: false,
		},
		{
			name:        "Non-empty string should be valid",
			input:       "test",
			expectValid: true,
			expectValue: "test",
		},
		{
			name:        "String with spaces should be valid",
			input:       "  test  ",
			expectValid: true,
			expectValue: "  test  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToNullString(tt.input)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected Valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if tt.expectValid && result.String != tt.expectValue {
				t.Errorf("Expected %q, got %q", tt.expectValue, result.String)
			}
		})
	}
}

func TestInt64ToNullInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       int64
		expectValid bool
		expectValue int64
	}{
		{
			name:        "Zero should be NULL",
			input:       0,
			expectValid: false,
		},
		{
			name:        "Positive number should be valid",
			input:       12345,
			expectValid: true,
			expectValue: 12345,
		},
		{
			name:        "Negative number should be valid",
			input:       -12345,
			expectValid: true,
			expectValue: -12345,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Int64ToNullInt64(tt.input)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected Valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if tt.expectValid && result.Int64 != tt.expectValue {
				t.Errorf("Expected %d, got %d", tt.expectValue, result.Int64)
			}
		})
	}
}

func TestFloat64ToNullFloat64(t *testing.T) {
	tests := []struct {
		name        string
		input       float64
		expectValid bool
		expectValue float64
	}{
		{
			name:        "Zero should be NULL",
			input:       0.0,
			expectValid: false,
		},
		{
			name:        "Positive number should be valid",
			input:       123.45,
			expectValid: true,
			expectValue: 123.45,
		},
		{
			name:        "Negative number should be valid",
			input:       -123.45,
			expectValid: true,
			expectValue: -123.45,
		},
		{
			name:        "Very small positive number should be valid",
			input:       0.001,
			expectValid: true,
			expectValue: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Float64ToNullFloat64(tt.input)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected Valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if tt.expectValid && result.Float64 != tt.expectValue {
				t.Errorf("Expected %f, got %f", tt.expectValue, result.Float64)
			}
		})
	}
}

func TestIsDiscountLine(t *testing.T) {
	tests := []struct {
		name           string
		sku            string
		productName    string
		priceBrutto    float64
		expectDiscount bool
	}{
		{
			name:           "Empty SKU should be discount",
			sku:            "",
			productName:    "Regular Product",
			priceBrutto:    100.0,
			expectDiscount: true,
		},
		{
			name:           "Negative price should be discount",
			sku:            "SKU-123",
			productName:    "Product",
			priceBrutto:    -50.0,
			expectDiscount: true,
		},
		{
			name:           "Discount in name should be discount",
			sku:            "SKU-123",
			productName:    "DISCOUNT Line",
			priceBrutto:    100.0,
			expectDiscount: true,
		},
		{
			name:           "Discount lowercase in name should be discount",
			sku:            "SKU-123",
			productName:    "special discount item",
			priceBrutto:    100.0,
			expectDiscount: true,
		},
		{
			name:           "Valid product should not be discount",
			sku:            "SKU-123",
			productName:    "Regular Product",
			priceBrutto:    100.0,
			expectDiscount: false,
		},
		{
			name:           "Zero price should not be discount (not negative)",
			sku:            "SKU-123",
			productName:    "Product",
			priceBrutto:    0.0,
			expectDiscount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDiscountLine(tt.sku, tt.productName, tt.priceBrutto)

			if result != tt.expectDiscount {
				t.Errorf("Expected %v, got %v", tt.expectDiscount, result)
			}
		})
	}
}
