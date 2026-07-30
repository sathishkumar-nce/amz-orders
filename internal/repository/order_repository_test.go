package repository

import (
	"database/sql"
	"testing"

	"github.com/sathishkumar-nce/amz-orders/internal/config"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
)

func TestHasExcludedInteraktSKU(t *testing.T) {
	products := []models.OrderProduct{
		{SKU: sql.NullString{String: " V6-03QB-MW8P ", Valid: true}},
	}

	gotSKU, excluded := hasExcludedInteraktSKU(products, config.DefaultInteraktExcludedSKUs)
	if !excluded {
		t.Fatal("expected default excluded SKU to block Interakt send")
	}
	if gotSKU != "V6-03QB-MW8P" {
		t.Fatalf("expected normalized SKU V6-03QB-MW8P, got %q", gotSKU)
	}
}

func TestHasExcludedInteraktSKUUsesExtraExcludedSKUs(t *testing.T) {
	products := []models.OrderProduct{
		{SKU: sql.NullString{String: "custom-sku", Valid: true}},
	}

	gotSKU, excluded := hasExcludedInteraktSKU(products, config.DefaultInteraktExcludedSKUs, []string{"CUSTOM-SKU"})
	if !excluded {
		t.Fatal("expected extra excluded SKU to block Interakt send")
	}
	if gotSKU != "CUSTOM-SKU" {
		t.Fatalf("expected normalized SKU CUSTOM-SKU, got %q", gotSKU)
	}
}

func TestHasExcludedInteraktSKUIgnoresAllowedSKUs(t *testing.T) {
	products := []models.OrderProduct{
		{SKU: sql.NullString{String: "allowed-sku", Valid: true}},
	}

	if gotSKU, excluded := hasExcludedInteraktSKU(products, config.DefaultInteraktExcludedSKUs); excluded {
		t.Fatalf("expected allowed SKU not to block Interakt send, got %q", gotSKU)
	}
}
