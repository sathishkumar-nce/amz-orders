CREATE INDEX IF NOT EXISTS idx_direct_orders_awb ON direct_orders (awb);
CREATE INDEX IF NOT EXISTS idx_direct_orders_pincode ON direct_orders (pincode);
CREATE INDEX IF NOT EXISTS idx_direct_orders_invoice_date ON direct_orders (invoice_date);
CREATE INDEX IF NOT EXISTS idx_direct_orders_deleted_created_at ON direct_orders (deleted_at, created_at DESC);
