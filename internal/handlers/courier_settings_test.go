package handlers

import (
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"testing"
)

func TestCustomRuntimeModesAndDefaultsValidation(t *testing.T) {
	valid := func() models.CourierSettings {
		return models.CourierSettings{DTDCModes: []models.CourierServiceMode{{Code: "b2b premium", Label: "Business premium", Enabled: true}}, DTDCDefaultService: "b2b premium", DTDCDefaultCommodity: "84", DTDCLabelCode: "SHIP_LABEL_4X6", AutoDownloadDTDCLabels: true}
	}
	settings := valid()
	if err := validateCourierSettings(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.DTDCModes[0].Code != "B2B PREMIUM" {
		t.Fatal("service code not normalized")
	}
	for _, change := range []func(*models.CourierSettings){func(s *models.CourierSettings) {
		s.DTDCModes = append(s.DTDCModes, models.CourierServiceMode{Code: "B2B PREMIUM", Enabled: true})
	}, func(s *models.CourierSettings) { s.DTDCModes[0].Enabled = false }, func(s *models.CourierSettings) { s.DTDCDefaultCommodity = "" }, func(s *models.CourierSettings) { s.DTDCLabelCode = "UNSUPPORTED" }} {
		s := valid()
		change(&s)
		if err := validateCourierSettings(&s); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
}
func TestCarrierDateWithoutTimeDoesNotInventDeliveryHour(t *testing.T) {
	if got := trackingTime("07112026", ""); got != "2026-11-07" {
		t.Fatalf("date-only value changed: %s", got)
	}
}
