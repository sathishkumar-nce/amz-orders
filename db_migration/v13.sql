CREATE TABLE IF NOT EXISTS amazon_row_highlight_rules (
    rule_key VARCHAR(64) PRIMARY KEY,
    label VARCHAR(120) NOT NULL,
    field_name VARCHAR(64) NOT NULL,
    operator VARCHAR(16) NOT NULL,
    threshold_number DOUBLE PRECISION NULL,
    match_text VARCHAR(64) NULL,
    color_mode VARCHAR(16) NOT NULL DEFAULT 'solid',
    color_start VARCHAR(32) NOT NULL,
    color_end VARCHAR(32) NULL,
    sort_order INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_amazon_row_highlight_rules_operator CHECK (operator IN ('gt', 'eq')),
    CONSTRAINT chk_amazon_row_highlight_rules_color_mode CHECK (color_mode IN ('solid', 'gradient'))
);

INSERT INTO amazon_row_highlight_rules (
    rule_key, label, field_name, operator, threshold_number, match_text, color_mode, color_start, color_end, sort_order, enabled
) VALUES
    ('priority_p1', 'Priority P1', 'priority', 'eq', NULL, 'p1', 'gradient', '#fecaca', '#fef08a', 10, TRUE),
    ('priority_p2', 'Priority P2', 'priority', 'eq', NULL, 'p2', 'gradient', '#fed7aa', '#fef9c3', 20, TRUE),
    ('priority_p3', 'Priority P3', 'priority', 'eq', NULL, 'p3', 'gradient', '#fbcfe8', '#fef9c3', 30, TRUE),
    ('payment_done_high', 'Payment Done Above', 'payment_done', 'gt', 4000, NULL, 'solid', '#dcfce7', NULL, 40, TRUE),
    ('quantity_above_one', 'Quantity Above One', 'quantity', 'gt', 1, NULL, 'solid', '#dbeafe', NULL, 50, TRUE),
    ('is_round_true', 'Round Product', 'is_round', 'eq', NULL, 'true', 'solid', '#ede9fe', NULL, 60, TRUE)
ON CONFLICT (rule_key) DO UPDATE SET
    label = EXCLUDED.label,
    field_name = EXCLUDED.field_name,
    operator = EXCLUDED.operator,
    threshold_number = EXCLUDED.threshold_number,
    match_text = EXCLUDED.match_text,
    color_mode = EXCLUDED.color_mode,
    color_start = EXCLUDED.color_start,
    color_end = EXCLUDED.color_end,
    sort_order = EXCLUDED.sort_order,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();
