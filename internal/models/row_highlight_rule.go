package models

type AmazonRowHighlightRule struct {
	RuleKey         string   `json:"rule_key"`
	Label           string   `json:"label"`
	FieldName       string   `json:"field_name"`
	Operator        string   `json:"operator"`
	ThresholdNumber *float64 `json:"threshold_number,omitempty"`
	MatchText       *string  `json:"match_text,omitempty"`
	ColorMode       string   `json:"color_mode"`
	ColorStart      string   `json:"color_start"`
	ColorEnd        *string  `json:"color_end,omitempty"`
	SortOrder       int      `json:"sort_order"`
	Enabled         bool     `json:"enabled"`
}

type UpdateAmazonRowHighlightRulesRequest struct {
	Rules []AmazonRowHighlightRule `json:"rules" binding:"required,min=1"`
}
