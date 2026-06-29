CREATE TABLE IF NOT EXISTS order_priority_rules (
    priority_key VARCHAR(10) PRIMARY KEY,
    sku_values TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_order_priority_rules_key CHECK (
        priority_key IN ('p1', 'p2', 'p3', 'p4')
    )
);

INSERT INTO order_priority_rules (priority_key, sku_values)
VALUES
    ('p1', '{}'),
    ('p2', '{}'),
    ('p3', '{}'),
    ('p4', '{}')
ON CONFLICT (priority_key) DO NOTHING;
