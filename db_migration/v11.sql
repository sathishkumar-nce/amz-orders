ALTER TABLE amazon_orders
DROP COLUMN IF EXISTS whatsapp_sent,
DROP COLUMN IF EXISTS email_sent;
