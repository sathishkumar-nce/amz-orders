CREATE TABLE IF NOT EXISTS shipping_date_filter_settings (
    filter_key VARCHAR(64) PRIMARY KEY,
    label VARCHAR(120) NOT NULL,
    start_day_offset INT NOT NULL,
    start_hour INT NOT NULL,
    start_minute INT NOT NULL,
    end_day_offset INT NOT NULL,
    end_hour INT NOT NULL,
    end_minute INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_shipping_date_filter_settings_start_hour CHECK (start_hour BETWEEN 0 AND 23),
    CONSTRAINT chk_shipping_date_filter_settings_start_minute CHECK (start_minute BETWEEN 0 AND 59),
    CONSTRAINT chk_shipping_date_filter_settings_end_hour CHECK (end_hour BETWEEN 0 AND 23),
    CONSTRAINT chk_shipping_date_filter_settings_end_minute CHECK (end_minute BETWEEN 0 AND 59)
);

INSERT INTO shipping_date_filter_settings (
    filter_key, label, start_day_offset, start_hour, start_minute, end_day_offset, end_hour, end_minute
) VALUES
    ('prime_ship_today', 'Prime - Ship Today', -1, 13, 50, 0, 13, 50),
    ('prime_ship_tomorrow', 'Prime - Ship Tomorrow', 0, 13, 50, 1, 13, 50),
    ('one_day_handle_ship_today', '1 Day Handle - Ship Today', -2, 10, 0, -1, 10, 0),
    ('one_day_handle_ship_tomorrow', '1 Day Handle - Ship Tomorrow', -1, 10, 0, 0, 10, 0),
    ('custom', 'Custom', 0, 0, 0, 1, 0, 0)
ON CONFLICT (filter_key) DO UPDATE SET
    label = EXCLUDED.label,
    start_day_offset = EXCLUDED.start_day_offset,
    start_hour = EXCLUDED.start_hour,
    start_minute = EXCLUDED.start_minute,
    end_day_offset = EXCLUDED.end_day_offset,
    end_hour = EXCLUDED.end_hour,
    end_minute = EXCLUDED.end_minute,
    updated_at = NOW();
