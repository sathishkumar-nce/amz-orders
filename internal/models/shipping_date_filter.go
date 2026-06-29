package models

type ShippingDateFilterSetting struct {
	FilterKey      string `json:"filter_key"`
	Label          string `json:"label"`
	StartDayOffset int    `json:"start_day_offset"`
	StartHour      int    `json:"start_hour"`
	StartMinute    int    `json:"start_minute"`
	EndDayOffset   int    `json:"end_day_offset"`
	EndHour        int    `json:"end_hour"`
	EndMinute      int    `json:"end_minute"`
	IsRangeEnabled bool   `json:"is_range_enabled"`
}

type UpdateShippingDateFilterSettingsRequest struct {
	Filters []ShippingDateFilterSetting `json:"filters" binding:"required,min=1"`
}

type ShippingDateFilterStateResponse struct {
	Filters         []ShippingDateFilterSetting `json:"filters"`
	ActiveFilterKey string                      `json:"active_filter_key"`
}

type UpdateActiveShippingDateFilterRequest struct {
	FilterKey string `json:"filter_key" binding:"required"`
}
