CREATE TABLE IF NOT EXISTS amazon_orders (
    -- Primary key from Amazon
    amazon_order_id VARCHAR(50) PRIMARY KEY,

    -- BaseLinker order identity
    baselinker_order_id BIGINT NOT NULL UNIQUE,
    shop_order_id BIGINT DEFAULT 0,

    -- Source
    order_source VARCHAR(50),
    order_source_id BIGINT,
    order_source_info VARCHAR(100),

    -- BaseLinker status
    order_status_id BIGINT NOT NULL,
    confirmed BOOLEAN DEFAULT TRUE,

    -- Dates from BaseLinker
    date_confirmed TIMESTAMP,
    date_add TIMESTAMP,
    date_in_status TIMESTAMP,

    -- Customer
    user_login VARCHAR(255),
    phone VARCHAR(50),
    email VARCHAR(255),

    -- Comments from BaseLinker
    user_comments TEXT,
    admin_comments TEXT,

    -- Payment
    currency VARCHAR(10),
    payment_method VARCHAR(100),
    payment_method_cod VARCHAR(10),
    payment_done NUMERIC(12,2) DEFAULT 0,

    -- Delivery
    delivery_method_id BIGINT,
    delivery_method VARCHAR(255),
    delivery_price NUMERIC(12,2) DEFAULT 0,
    delivery_package_module VARCHAR(100),
    delivery_package_nr VARCHAR(100),

    delivery_fullname VARCHAR(255),
    delivery_company VARCHAR(255),
    delivery_address TEXT,
    delivery_city VARCHAR(100),
    delivery_state VARCHAR(100),
    delivery_postcode VARCHAR(20),
    delivery_country_code VARCHAR(10),
    delivery_country VARCHAR(100),

    -- Delivery point
    delivery_point_id VARCHAR(100),
    delivery_point_name VARCHAR(255),
    delivery_point_address TEXT,
    delivery_point_postcode VARCHAR(20),
    delivery_point_city VARCHAR(100),

    -- Invoice
    invoice_fullname VARCHAR(255),
    invoice_company VARCHAR(255),
    invoice_nip VARCHAR(100),
    invoice_address TEXT,
    invoice_city VARCHAR(100),
    invoice_state VARCHAR(100),
    invoice_postcode VARCHAR(20),
    invoice_country_code VARCHAR(10),
    invoice_country VARCHAR(100),
    want_invoice VARCHAR(10),

    -- Extra fields
    extra_field_1 VARCHAR(255),
    extra_field_2 VARCHAR(255),

    -- BaseLinker page
    order_page TEXT,

    -- BaseLinker internal states
    pick_state INT DEFAULT 0,
    pack_state INT DEFAULT 0,
    star INT DEFAULT 0,
    crm_client_id BIGINT DEFAULT 0,

    -- Main product summary
    main_order_product_id BIGINT,
    main_product_name TEXT,
    main_sku VARCHAR(100),
    main_asin VARCHAR(50),
    main_price_brutto NUMERIC(12,2),
    main_tax_rate NUMERIC(6,2),
    main_quantity NUMERIC(10,2),

    -- Your extra size columns
    default_width_in_inches NUMERIC(10,2),
    default_length_in_inches NUMERIC(10,2),
    default_width_in_mm NUMERIC(10,2),
    default_length_in_mm NUMERIC(10,2),
    customer_width_in_mm NUMERIC(10,2),
    customer_length_in_mm NUMERIC(10,2),
    corner_radius_and_notes TEXT,

    -- Return workflow
    return_status VARCHAR(50) DEFAULT 'none',
    return_reason TEXT,
    return_follow_up_actions TEXT,
    return_notes TEXT,

    -- Issue workflow
    issue_status VARCHAR(50) DEFAULT 'none',
    issue_reason TEXT,
    issue_follow_up_actions TEXT,

    -- General notes
    notes TEXT,

    -- Priority
    priority VARCHAR(10) DEFAULT 'p3',
    order_status VARCHAR(30) DEFAULT 'received',
    order_status_updated_at TIMESTAMP,
    safety_claim_updated_at TIMESTAMP,

    -- Raw API response for reference
    raw_payload JSONB,

    -- Audit
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_return_status CHECK (
        return_status IN (
            'none',
            'returned',
            'replacement_placed',
            'converted_direct',
            'refunded'
        )
    ),

    CONSTRAINT chk_issue_status CHECK (
        issue_status IN (
            'none',
            'has_issues',
            'replacement_placed',
            'converted_direct',
            'refunded'
        )
    ),

    CONSTRAINT chk_priority CHECK (
        priority IN ('p1', 'p2', 'p3', 'p4')
    ),

    CONSTRAINT chk_order_workflow_status CHECK (
        order_status IN ('received', 'processing', 'sent', 'cancelled')
    )
);


CREATE TABLE IF NOT EXISTS amazon_order_products (
    order_product_id BIGINT PRIMARY KEY,

    amazon_order_id VARCHAR(50) NOT NULL
        REFERENCES amazon_orders(amazon_order_id)
        ON DELETE CASCADE,

    storage VARCHAR(50),
    storage_id BIGINT,

    product_id VARCHAR(100),
    variant_id VARCHAR(100),

    name TEXT,
    attributes TEXT,
    sku VARCHAR(100),
    ean VARCHAR(100),
    location VARCHAR(100),
    warehouse_id BIGINT,
    auction_id VARCHAR(100),

    price_brutto NUMERIC(12,2),
    thickness VARCHAR(50),
    tax_rate NUMERIC(6,2),
    quantity NUMERIC(10,2),
    weight NUMERIC(10,3),

    default_width_in_inches NUMERIC(10,2),
    default_length_in_inches NUMERIC(10,2),
    customer_width_in_inches NUMERIC(10,2),
    customer_length_in_inches NUMERIC(10,2),
    default_width_in_mm NUMERIC(10,2),
    default_length_in_mm NUMERIC(10,2),
    customer_width_in_mm NUMERIC(10,2),
    customer_length_in_mm NUMERIC(10,2),
    corner_radius_and_notes TEXT,

    return_status VARCHAR(50) DEFAULT 'none',
    return_status_updated_at TIMESTAMP,
    return_reason TEXT,
    return_follow_up_actions TEXT,
    return_notes TEXT,

    issue_status VARCHAR(50) DEFAULT 'none',
    issue_status_updated_at TIMESTAMP,
    issue_reason TEXT,
    issue_follow_up_actions TEXT,

    safety_claim VARCHAR(50) DEFAULT 'none',
    safety_claim_updated_at TIMESTAMP,
    safety_claim_notes TEXT,
    is_round BOOLEAN DEFAULT FALSE,

    bundle_id BIGINT,

    is_discount_line BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_amazon_order_products_return_status CHECK (
        return_status IN ('none', 'returned', 'replacement_placed', 'converted_direct', 'refunded')
    ),

    CONSTRAINT chk_amazon_order_products_issue_status CHECK (
        issue_status IN ('none', 'has_issues', 'replacement_placed', 'converted_direct', 'refunded')
    ),

    CONSTRAINT chk_amazon_order_products_safety_claim CHECK (
        safety_claim IN ('none', 'pending', 'done', 'not_needed', 'issues')
    )
);



CREATE INDEX IF NOT EXISTS idx_amazon_orders_baselinker_order_id
ON amazon_orders(baselinker_order_id);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_order_status_id
ON amazon_orders(order_status_id);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_confirmed
ON amazon_orders(confirmed);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_date_confirmed
ON amazon_orders(date_confirmed);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_date_add
ON amazon_orders(date_add);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_main_sku
ON amazon_orders(main_sku);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_phone
ON amazon_orders(phone);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_delivery_city
ON amazon_orders(delivery_city);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_delivery_state
ON amazon_orders(delivery_state);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_return_status
ON amazon_orders(return_status);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_issue_status
ON amazon_orders(issue_status);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_priority
ON amazon_orders(priority);

CREATE INDEX IF NOT EXISTS idx_amazon_orders_order_status
ON amazon_orders(order_status);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_amazon_order_id
ON amazon_order_products(amazon_order_id);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_sku
ON amazon_order_products(sku);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_auction_id
ON amazon_order_products(auction_id);


CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_amazon_orders_updated_at ON amazon_orders;
CREATE TRIGGER update_amazon_orders_updated_at
BEFORE UPDATE ON amazon_orders
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_amazon_order_products_updated_at ON amazon_order_products;
CREATE TRIGGER update_amazon_order_products_updated_at
BEFORE UPDATE ON amazon_order_products
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE amazon_orders
ADD COLUMN IF NOT EXISTS order_status VARCHAR(30) DEFAULT 'received',
ADD COLUMN IF NOT EXISTS order_status_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS safety_claim_updated_at TIMESTAMP;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'amazon_orders' AND column_name = 'status'
    ) THEN
        EXECUTE $sql$
            UPDATE amazon_orders
            SET order_status = CASE
                WHEN status = 'sent' THEN 'sent'
                WHEN status = 'cancelled' THEN 'cancelled'
                ELSE 'received'
            END
            WHERE order_status IS NULL OR order_status = ''
        $sql$;
    END IF;
END $$;

UPDATE amazon_orders
SET order_status = 'received'
WHERE order_status IS NULL OR order_status = '';

ALTER TABLE amazon_orders
DROP CONSTRAINT IF EXISTS chk_order_workflow_status;

ALTER TABLE amazon_orders
ADD CONSTRAINT chk_order_workflow_status CHECK (
    order_status IN ('received', 'processing', 'sent', 'cancelled')
);

-- Migration v2: Add users table for authentication

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Insert a default admin user (password: admin123)
-- Password hash for 'admin123' using bcrypt
INSERT INTO users (username, password, email, created_at, updated_at)
VALUES (
    'admin',
    '$2a$12$D.oUfKAOZl6JBGNhy1juWeOAMjebSjAxhI138p32TlpK8YOZe0xq2',
    'celentris.ops@gmail.com',
    NOW(),
    NOW()
) ON CONFLICT (username) DO NOTHING;

-- Migration v3: Add safety claim fields

-- Add safety claim workflow columns
ALTER TABLE amazon_orders 
ADD COLUMN IF NOT EXISTS safety_claim VARCHAR(50) DEFAULT 'none',
ADD COLUMN IF NOT EXISTS safety_claim_notes TEXT;

-- Add constraint for safety_claim
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_safety_claim'
    ) THEN
        ALTER TABLE amazon_orders
        ADD CONSTRAINT chk_safety_claim CHECK (
            safety_claim IN ('none', 'pending', 'done', 'not_needed', 'issues')
        );
    END IF;
END $$;

-- Create indexes for the new columns
CREATE INDEX IF NOT EXISTS idx_amazon_orders_safety_claim ON amazon_orders(safety_claim);

-- Comments for documentation
COMMENT ON COLUMN amazon_orders.safety_claim IS 'Safety claim status: none, pending, done, not_needed, issues';
COMMENT ON COLUMN amazon_orders.safety_claim_notes IS 'Notes related to safety claim processing';

-- Migration v4: Add is_round column to amazon_orders table

-- Add is_round column
ALTER TABLE amazon_orders 
ADD COLUMN IF NOT EXISTS is_round BOOLEAN DEFAULT FALSE;

-- Create index for the new column
CREATE INDEX IF NOT EXISTS idx_amazon_orders_is_round ON amazon_orders(is_round);

-- Comment for documentation
COMMENT ON COLUMN amazon_orders.is_round IS 'Indicates if the order is for a round product (true) or not (false)';


-- Migration v5: Create direct_orders table for managing direct orders
-- This table handles orders that don't come from BaseLinker/Amazon

CREATE TABLE IF NOT EXISTS direct_orders (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Order identification
    source VARCHAR(255),                    -- Source of the order
    -- Allowed values: 'website', 'phone', 'whatsapp', 'email', 'meta', 'amz-replacement', 'issue-replacement', 'other'
    order_id VARCHAR(255) NOT NULL UNIQUE,  -- Unique order identifier
    
    -- Order status tracking
    order_status VARCHAR(50) DEFAULT 'pending',
    -- Allowed values: pending, confirmed, cutting, packed, shipped, delivered, cancelled, issues
    
    -- Shipping details
    courier_name VARCHAR(255),              -- Name of courier service (e.g., 'Delhivery', 'Blue Dart')
    tracking_url TEXT,                      -- Full tracking URL
    awb VARCHAR(255),                       -- Air Waybill number / tracking number
    
    -- Payment tracking
    payment_status VARCHAR(50) DEFAULT 'pending',
    -- Allowed values: pending, paid-full, paid-advance, refunded
    amount DECIMAL(10, 2),                  -- Total order amount
    advance_amount DECIMAL(10, 2),          -- Advance payment amount
    cod_amount DECIMAL(10, 2),              -- Cash on Delivery amount
    
    -- Customer information
    customer_name VARCHAR(255),
    address TEXT,
    pincode VARCHAR(20),
    mobile VARCHAR(50),
    alternate_mobile VARCHAR(50),           -- Alternate mobile number
    email VARCHAR(255),
    alternate_email VARCHAR(255),           -- Alternate email address
    
    -- Business fields
    remarks TEXT,                           -- General remarks/notes
    priority VARCHAR(10) DEFAULT 'P4',      -- Priority level: P1, P2, P3, P4
    issues TEXT,                            -- Issues/problems with the order
    
    -- Constraints for data validation
    CONSTRAINT valid_order_status CHECK (
        order_status IN ('pending', 'confirmed', 'cutting', 'packed', 'shipped', 'delivered', 'cancelled', 'issues')
    ),
    CONSTRAINT valid_payment_status CHECK (
        payment_status IN ('pending', 'paid-full', 'paid-advance', 'refunded')
    ),
    CONSTRAINT valid_priority CHECK (
        priority IN ('P1', 'P2', 'P3', 'P4')
    ),
    CONSTRAINT valid_source CHECK (
        source IN ('website', 'phone', 'whatsapp', 'email', 'meta', 'amz-replacement', 'issue-replacement', 'other')
    )
);

ALTER TABLE direct_orders
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

-- Create direct_order_items table for multiple items per order
CREATE TABLE IF NOT EXISTS direct_order_items (
    -- Primary key
    id BIGSERIAL PRIMARY KEY,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key to direct_orders
    order_id VARCHAR(255) NOT NULL,
    
    -- Item details
    item VARCHAR(500),                      -- Product/item description
    quantity INTEGER DEFAULT 1,             -- Quantity of items
    dimension VARCHAR(255),                 -- Product dimensions (e.g., "24 x 36 Inch")
    thickness VARCHAR(50),                  -- Product thickness (e.g., "1mm", "1.5mm")
    weight DECIMAL(10, 3),                  -- Product weight in kg
    amount DECIMAL(10, 2),                  -- Item amount
    remark TEXT,                            -- Item-specific remarks
    
    -- Foreign key constraint
    CONSTRAINT fk_direct_order_items_order_id 
        FOREIGN KEY (order_id) 
        REFERENCES direct_orders(order_id) 
        ON DELETE CASCADE
);

-- Create indexes for frequently queried columns on direct_orders
CREATE INDEX IF NOT EXISTS idx_direct_orders_order_id ON direct_orders(order_id);
CREATE INDEX IF NOT EXISTS idx_direct_orders_order_status ON direct_orders(order_status);
CREATE INDEX IF NOT EXISTS idx_direct_orders_payment_status ON direct_orders(payment_status);
CREATE INDEX IF NOT EXISTS idx_direct_orders_priority ON direct_orders(priority);
CREATE INDEX IF NOT EXISTS idx_direct_orders_mobile ON direct_orders(mobile);
CREATE INDEX IF NOT EXISTS idx_direct_orders_created_at ON direct_orders(created_at);
CREATE INDEX IF NOT EXISTS idx_direct_orders_source ON direct_orders(source);
CREATE INDEX IF NOT EXISTS idx_direct_orders_deleted_at ON direct_orders(deleted_at);

-- Create indexes for direct_order_items
CREATE INDEX IF NOT EXISTS idx_direct_order_items_order_id ON direct_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_direct_order_items_created_at ON direct_order_items(created_at);

-- Create a trigger to automatically update the updated_at timestamp for direct_orders
CREATE OR REPLACE FUNCTION update_direct_orders_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_direct_orders_updated_at ON direct_orders;
CREATE TRIGGER trigger_update_direct_orders_updated_at
    BEFORE UPDATE ON direct_orders
    FOR EACH ROW
    EXECUTE FUNCTION update_direct_orders_updated_at();

-- Create a trigger to automatically update the updated_at timestamp for direct_order_items
CREATE OR REPLACE FUNCTION update_direct_order_items_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_direct_order_items_updated_at ON direct_order_items;
CREATE TRIGGER trigger_update_direct_order_items_updated_at
    BEFORE UPDATE ON direct_order_items
    FOR EACH ROW
    EXECUTE FUNCTION update_direct_order_items_updated_at();

-- Add comments to document the direct_orders table and columns
COMMENT ON TABLE direct_orders IS 'Stores direct orders that do not come from BaseLinker/Amazon integration';
COMMENT ON COLUMN direct_orders.id IS 'Unique identifier for the direct order';
COMMENT ON COLUMN direct_orders.created_at IS 'Timestamp when the order was created';
COMMENT ON COLUMN direct_orders.updated_at IS 'Timestamp when the order was last updated';
COMMENT ON COLUMN direct_orders.source IS 'Source of the order: website, phone, whatsapp, email, meta, amz-replacement, issue-replacement, other';
COMMENT ON COLUMN direct_orders.order_id IS 'Unique order identifier (user-defined or auto-generated)';
COMMENT ON COLUMN direct_orders.order_status IS 'Current status: pending, confirmed, cutting, packed, shipped, delivered, cancelled, issues';
COMMENT ON COLUMN direct_orders.courier_name IS 'Name of the courier service';
COMMENT ON COLUMN direct_orders.awb IS 'Air Waybill number / tracking number';
COMMENT ON COLUMN direct_orders.payment_status IS 'Payment status: pending, paid-full, paid-advance, refunded';
COMMENT ON COLUMN direct_orders.amount IS 'Total order amount';
COMMENT ON COLUMN direct_orders.advance_amount IS 'Advance payment amount';
COMMENT ON COLUMN direct_orders.cod_amount IS 'Cash on Delivery amount';
COMMENT ON COLUMN direct_orders.customer_name IS 'Full name of the customer';
COMMENT ON COLUMN direct_orders.address IS 'Complete delivery address';
COMMENT ON COLUMN direct_orders.pincode IS 'PIN/ZIP code';
COMMENT ON COLUMN direct_orders.mobile IS 'Customer primary mobile/phone number';
COMMENT ON COLUMN direct_orders.alternate_mobile IS 'Customer alternate mobile/phone number';
COMMENT ON COLUMN direct_orders.email IS 'Customer primary email address';
COMMENT ON COLUMN direct_orders.alternate_email IS 'Customer alternate email address';
COMMENT ON COLUMN direct_orders.remarks IS 'General remarks and notes about the order';
COMMENT ON COLUMN direct_orders.priority IS 'Order priority: P1 (highest), P2, P3, P4 (lowest)';
COMMENT ON COLUMN direct_orders.issues IS 'Description of any issues or problems with the order';

-- Add comments to document the direct_order_items table and columns
COMMENT ON TABLE direct_order_items IS 'Stores individual items for each direct order (supports multiple items per order)';
COMMENT ON COLUMN direct_order_items.id IS 'Unique identifier for the order item';
COMMENT ON COLUMN direct_order_items.created_at IS 'Timestamp when the item was created';
COMMENT ON COLUMN direct_order_items.updated_at IS 'Timestamp when the item was last updated';
COMMENT ON COLUMN direct_order_items.order_id IS 'Reference to the parent order in direct_orders table';
COMMENT ON COLUMN direct_order_items.item IS 'Product/item description';
COMMENT ON COLUMN direct_order_items.quantity IS 'Quantity of this item';
COMMENT ON COLUMN direct_order_items.dimension IS 'Product dimensions (e.g., "24 x 36 Inch")';
COMMENT ON COLUMN direct_order_items.thickness IS 'Product thickness (e.g., "1mm", "1.5mm")';
COMMENT ON COLUMN direct_order_items.weight IS 'Product weight in kilograms';
COMMENT ON COLUMN direct_order_items.amount IS 'Amount for this item';
COMMENT ON COLUMN direct_order_items.remark IS 'Item-specific remarks or notes';

-- Product-first operational fields on amazon_order_products
ALTER TABLE amazon_order_products
ADD COLUMN IF NOT EXISTS default_width_in_inches NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS default_length_in_inches NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS customer_width_in_inches NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS customer_length_in_inches NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS default_width_in_mm NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS default_length_in_mm NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS customer_width_in_mm NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS customer_length_in_mm NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS thickness VARCHAR(50),
ADD COLUMN IF NOT EXISTS corner_radius_and_notes TEXT,
ADD COLUMN IF NOT EXISTS return_status VARCHAR(50) DEFAULT 'none',
ADD COLUMN IF NOT EXISTS return_status_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS return_reason TEXT,
ADD COLUMN IF NOT EXISTS return_follow_up_actions TEXT,
ADD COLUMN IF NOT EXISTS return_notes TEXT,
ADD COLUMN IF NOT EXISTS issue_status VARCHAR(50) DEFAULT 'none',
ADD COLUMN IF NOT EXISTS issue_status_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS issue_reason TEXT,
ADD COLUMN IF NOT EXISTS issue_follow_up_actions TEXT,
ADD COLUMN IF NOT EXISTS safety_claim VARCHAR(50) DEFAULT 'none',
ADD COLUMN IF NOT EXISTS safety_claim_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS safety_claim_notes TEXT,
ADD COLUMN IF NOT EXISTS is_round BOOLEAN DEFAULT FALSE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_amazon_order_products_return_status'
    ) THEN
        ALTER TABLE amazon_order_products
        ADD CONSTRAINT chk_amazon_order_products_return_status CHECK (
            return_status IN ('none', 'returned', 'replacement_placed', 'converted_direct', 'refunded')
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_amazon_order_products_issue_status'
    ) THEN
        ALTER TABLE amazon_order_products
        ADD CONSTRAINT chk_amazon_order_products_issue_status CHECK (
            issue_status IN ('none', 'has_issues', 'replacement_placed', 'converted_direct', 'refunded')
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_amazon_order_products_safety_claim'
    ) THEN
        ALTER TABLE amazon_order_products
        ADD CONSTRAINT chk_amazon_order_products_safety_claim CHECK (
            safety_claim IN ('none', 'pending', 'done', 'not_needed', 'issues')
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_sku ON amazon_order_products(sku);
CREATE INDEX IF NOT EXISTS idx_amazon_order_products_return_status ON amazon_order_products(return_status);
CREATE INDEX IF NOT EXISTS idx_amazon_order_products_issue_status ON amazon_order_products(issue_status);
CREATE INDEX IF NOT EXISTS idx_amazon_order_products_safety_claim ON amazon_order_products(safety_claim);

-- Backfill existing product rows from older order-level columns so no existing work is lost.
UPDATE amazon_order_products p
SET
    default_width_in_inches = COALESCE(p.default_width_in_inches, o.default_width_in_inches),
    default_length_in_inches = COALESCE(p.default_length_in_inches, o.default_length_in_inches),
    customer_width_in_inches = COALESCE(p.customer_width_in_inches, o.customer_width_in_mm / 25.4),
    customer_length_in_inches = COALESCE(p.customer_length_in_inches, o.customer_length_in_mm / 25.4),
    default_width_in_mm = COALESCE(p.default_width_in_mm, o.default_width_in_mm),
    default_length_in_mm = COALESCE(p.default_length_in_mm, o.default_length_in_mm),
    customer_width_in_mm = COALESCE(p.customer_width_in_mm, o.customer_width_in_mm),
    customer_length_in_mm = COALESCE(p.customer_length_in_mm, o.customer_length_in_mm),
    corner_radius_and_notes = COALESCE(p.corner_radius_and_notes, o.corner_radius_and_notes),
    return_status = COALESCE(NULLIF(p.return_status, ''), NULLIF(o.return_status, ''), 'none'),
    return_reason = COALESCE(p.return_reason, o.return_reason),
    return_follow_up_actions = COALESCE(p.return_follow_up_actions, o.return_follow_up_actions),
    return_notes = COALESCE(p.return_notes, o.return_notes),
    issue_status = COALESCE(NULLIF(p.issue_status, ''), NULLIF(o.issue_status, ''), 'none'),
    issue_reason = COALESCE(p.issue_reason, o.issue_reason),
    issue_follow_up_actions = COALESCE(p.issue_follow_up_actions, o.issue_follow_up_actions),
    safety_claim = COALESCE(NULLIF(p.safety_claim, ''), NULLIF(o.safety_claim, ''), 'none'),
    safety_claim_notes = COALESCE(p.safety_claim_notes, o.safety_claim_notes),
    is_round = COALESCE(p.is_round, o.is_round, FALSE)
FROM amazon_orders o
WHERE o.amazon_order_id = p.amazon_order_id;
