package models

import "time"

type CourierServiceMode struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}
type CourierSettings struct {
	DTDCModes              []CourierServiceMode `json:"dtdc_modes"`
	DTDCDefaultService     string               `json:"dtdc_default_service"`
	DTDCDefaultCommodity   string               `json:"dtdc_default_commodity"`
	AutoDownloadDTDCLabels bool                 `json:"auto_download_dtdc_labels"`
	DTDCLabelCode          string               `json:"dtdc_label_code"`
	UpdatedAt              time.Time            `json:"updated_at"`
}
type ShipmentTrackingEvent struct {
	Status      string `json:"status"`
	Location    string `json:"location,omitempty"`
	Destination string `json:"destination,omitempty"`
	OccurredAt  string `json:"occurred_at,omitempty"`
	Remarks     string `json:"remarks,omitempty"`
	Code        string `json:"code,omitempty"`
}
type ShipmentTracking struct {
	Courier          string                  `json:"courier"`
	AWB              string                  `json:"awb"`
	Status           string                  `json:"status"`
	ReportedStatus   string                  `json:"reported_status,omitempty"`
	CurrentLocation  string                  `json:"current_location,omitempty"`
	Origin           string                  `json:"origin,omitempty"`
	Destination      string                  `json:"destination,omitempty"`
	ExpectedDelivery string                  `json:"expected_delivery,omitempty"`
	LastEventAt      string                  `json:"last_event_at,omitempty"`
	Remarks          string                  `json:"remarks,omitempty"`
	TrackingURL      string                  `json:"tracking_url"`
	CheckedAt        time.Time               `json:"checked_at"`
	Events           []ShipmentTrackingEvent `json:"events"`
}
