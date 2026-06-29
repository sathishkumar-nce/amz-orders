ALTER TABLE direct_orders
DROP COLUMN IF EXISTS invoice_number,
DROP COLUMN IF EXISTS tracking_url,
DROP COLUMN IF EXISTS ewaybill;
