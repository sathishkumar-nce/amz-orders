ALTER TABLE shipping_date_filter_settings
    ADD COLUMN IF NOT EXISTS is_range_enabled BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE shipping_date_filter_settings
SET is_range_enabled = TRUE
WHERE filter_key <> 'all_date';

INSERT INTO shipping_date_filter_settings (
    filter_key, label, start_day_offset, start_hour, start_minute, end_day_offset, end_hour, end_minute, is_range_enabled
) VALUES (
    'all_date', 'All Date', 0, 0, 0, 0, 0, 0, FALSE
)
ON CONFLICT (filter_key) DO UPDATE SET
    label = EXCLUDED.label,
    start_day_offset = EXCLUDED.start_day_offset,
    start_hour = EXCLUDED.start_hour,
    start_minute = EXCLUDED.start_minute,
    end_day_offset = EXCLUDED.end_day_offset,
    end_hour = EXCLUDED.end_hour,
    end_minute = EXCLUDED.end_minute,
    is_range_enabled = EXCLUDED.is_range_enabled,
    updated_at = NOW();

CREATE TABLE IF NOT EXISTS shipping_date_filter_state (
    state_key VARCHAR(64) PRIMARY KEY,
    active_filter_key VARCHAR(64) NOT NULL REFERENCES shipping_date_filter_settings(filter_key) ON DELETE RESTRICT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO shipping_date_filter_state (state_key, active_filter_key)
VALUES ('global', 'prime_ship_today')
ON CONFLICT (state_key) DO NOTHING;
