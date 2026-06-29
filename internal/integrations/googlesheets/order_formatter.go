package googlesheets

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

// FormatOrderDetails formats order products into the required string format
// Single product: (Qty:1, 32.5 x 48.5 Inch, 1.5mm thick)
// Multiple products: (Qty:1, 36.5 x 60.5 Inch, 1mm thick), (Qty:1, 24.5 x 36.5 Inch, 1mm thick)
func FormatOrderDetails(products []models.OrderProduct) string {
	var details []string
	skuMapper := utils.GetSKUMapper()

	for _, p := range products {
		// Skip discount lines
		if p.IsDiscountLine {
			continue
		}

		sku := nullStringToString(p.SKU)
		if sku == "" {
			continue
		}

		quantity := nullFloat64ToFloat(p.Quantity)
		if quantity == 0 {
			quantity = 1
		}

		// Get dimension and thickness from SKU mapper
		dimension := ""
		thickness := ""

		if skuData, found := skuMapper.GetBySKU(sku); found {
			// Format dimension as "width x length Inch"
			// Use smart formatting: show decimals only if needed
			if skuData.WidthInInches > 0 && skuData.LengthInInches > 0 {
				width := formatNumber(skuData.WidthInInches)
				length := formatNumber(skuData.LengthInInches)
				dimension = fmt.Sprintf("%s x %s Inch", width, length)
			}
			
			// Get thickness directly from CSV (e.g., "1mm", "1.5mm", "2mm")
			if skuData.Thickness != "" {
				thickness = skuData.Thickness
			}
		}

		// Build the detail string in exact format: (Qty:X, dimension, thickness thick)
		if dimension != "" && thickness != "" {
			// Format: (Qty:1, 32.5 x 48.5 Inch, 1.5mm thick)
			qtyStr := formatNumber(quantity)
			details = append(details, fmt.Sprintf("(Qty:%s, %s, %s thick)", qtyStr, dimension, thickness))
		} else if dimension != "" {
			// No thickness available
			qtyStr := formatNumber(quantity)
			details = append(details, fmt.Sprintf("(Qty:%s, %s)", qtyStr, dimension))
		} else {
			// Fallback if no SKU data found
			qtyStr := formatNumber(quantity)
			details = append(details, fmt.Sprintf("(Qty:%s, SKU: %s)", qtyStr, sku))
		}
	}

	// Join multiple products with ", "
	return strings.Join(details, ", ")
}

// formatNumber formats a number intelligently:
// - 1.0 → "1"
// - 1.5 → "1.5"
// - 32.5 → "32.5"
// - 48.0 → "48"
func formatNumber(num float64) string {
	// Check if the number is a whole number
	if num == math.Floor(num) {
		return fmt.Sprintf("%.0f", num)
	}
	// Has decimals - format with 1 decimal place
	return fmt.Sprintf("%.1f", num)
}

// GetAllSKUs returns comma-separated list of SKUs
func GetAllSKUs(products []models.OrderProduct) string {
	var skus []string

	for _, p := range products {
		// Skip discount lines
		if p.IsDiscountLine {
			continue
		}

		sku := nullStringToString(p.SKU)
		if sku != "" {
			quantity := nullFloat64ToFloat(p.Quantity)
			if quantity == 0 {
				quantity = 1
			}
			// Include quantity if > 1
			if quantity > 1 {
				qtyStr := formatNumber(quantity)
				skus = append(skus, fmt.Sprintf("%s (x%s)", sku, qtyStr))
			} else {
				skus = append(skus, sku)
			}
		}
	}

	return strings.Join(skus, ", ")
}

// Helper functions to handle sql.Null types
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullFloat64ToFloat(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}
