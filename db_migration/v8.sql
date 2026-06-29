ALTER TABLE direct_orders
ALTER COLUMN order_status SET DEFAULT 'confirmed';

UPDATE direct_orders
SET order_status = CASE
    WHEN order_status IN ('pending', 'confirmed') THEN 'confirmed'
    WHEN order_status = 'packed' THEN 'packed'
    WHEN order_status IN ('on-hold', 'hold', 'on_hold') THEN 'on-hold'
    WHEN order_status IN ('shipped', 'delivered', 'cutting', 'forwarded') THEN 'forwarded'
    WHEN order_status = 'cancelled' THEN 'cancelled'
    WHEN order_status = 'returned' THEN 'returned'
    WHEN order_status IN ('issues', 'other-issues') THEN 'other-issues'
    ELSE 'confirmed'
END
WHERE order_status IS NULL
   OR order_status NOT IN ('confirmed', 'packed', 'on-hold', 'forwarded', 'cancelled', 'returned', 'other-issues');

ALTER TABLE direct_orders
DROP CONSTRAINT IF EXISTS valid_order_status;

ALTER TABLE direct_orders
ADD CONSTRAINT valid_order_status CHECK (
    order_status IN ('confirmed', 'packed', 'on-hold', 'forwarded', 'cancelled', 'returned', 'other-issues')
);
