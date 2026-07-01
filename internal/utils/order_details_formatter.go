package utils

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

// FormatOrderDetails formats order products into the required string format.
// Single product: (Qty:1, 32.5 x 48.5 Inch, 1.5mm thick)
// Multiple products: (Qty:1, 36.5 x 60.5 Inch, 1mm thick), (Qty:1, 24.5 x 36.5 Inch, 1mm thick)
func FormatOrderDetails(products []models.OrderProduct) string {
	var details []string
	skuMapper := GetSKUMapper()

	for _, p := range products {
		if p.IsDiscountLine {
			continue
		}

		sku := formatNullString(p.SKU)
		if sku == "" {
			continue
		}

		quantity := formatNullFloat64(p.Quantity)
		if quantity == 0 {
			quantity = 1
		}

		dimension := ""
		thickness := ""

		if skuData, found := skuMapper.GetBySKU(sku); found {
			if skuData.WidthInInches > 0 && skuData.LengthInInches > 0 {
				width := formatOrderNumber(skuData.WidthInInches)
				length := formatOrderNumber(skuData.LengthInInches)
				dimension = fmt.Sprintf("%s x %s Inch", width, length)
			}
			if skuData.Thickness != "" {
				thickness = skuData.Thickness
			}
		}

		qtyStr := formatOrderNumber(quantity)
		if dimension != "" && thickness != "" {
			details = append(details, fmt.Sprintf("(Qty:%s, %s, %s thick)", qtyStr, dimension, thickness))
		} else if dimension != "" {
			details = append(details, fmt.Sprintf("(Qty:%s, %s)", qtyStr, dimension))
		} else {
			details = append(details, fmt.Sprintf("(Qty:%s, SKU: %s)", qtyStr, sku))
		}
	}

	return strings.Join(details, ", ")
}

func formatOrderNumber(num float64) string {
	if num == math.Floor(num) {
		return fmt.Sprintf("%.0f", num)
	}
	return fmt.Sprintf("%.1f", num)
}

func formatNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func formatNullFloat64(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}
