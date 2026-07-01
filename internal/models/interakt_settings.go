package models

type InteraktSettings struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	TemplateName string `json:"template_name"`
	TestNumber   string `json:"test_number"`
}

type UpdateInteraktSettingsRequest struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode" binding:"required"`
	TemplateName string `json:"template_name" binding:"required"`
	TestNumber   string `json:"test_number"`
}
