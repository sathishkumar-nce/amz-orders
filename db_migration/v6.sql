ALTER TABLE direct_orders
ADD COLUMN IF NOT EXISTS courier_type VARCHAR(50) NOT NULL DEFAULT 'manual';

UPDATE direct_orders
SET courier_type = CASE
    WHEN LOWER(COALESCE(courier_name, '')) LIKE '%delhivery%' THEN 'delhivery'
    WHEN COALESCE(NULLIF(awb, '')) IS NOT NULL THEN 'manual'
    ELSE COALESCE(NULLIF(courier_type, ''), 'manual')
END
WHERE courier_type IS NULL
   OR courier_type = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'direct_orders_valid_courier_type'
    ) THEN
        ALTER TABLE direct_orders
        ADD CONSTRAINT direct_orders_valid_courier_type CHECK (
            courier_type IN ('manual', 'delhivery', 'bluedart', 'dtdc', 'xpressbees', 'other')
        );
    END IF;
END $$;
