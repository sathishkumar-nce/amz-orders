ALTER TABLE amazon_orders
DROP CONSTRAINT IF EXISTS chk_return_status,
DROP CONSTRAINT IF EXISTS chk_issue_status,
DROP CONSTRAINT IF EXISTS chk_safety_claim,
DROP CONSTRAINT IF EXISTS chk_order_workflow_status,
DROP CONSTRAINT IF EXISTS valid_order_status;

ALTER TABLE amazon_order_products
DROP CONSTRAINT IF EXISTS chk_amazon_order_products_return_status,
DROP CONSTRAINT IF EXISTS chk_amazon_order_products_issue_status,
DROP CONSTRAINT IF EXISTS chk_amazon_order_products_safety_claim;

ALTER TABLE amazon_orders
ADD COLUMN IF NOT EXISTS internal_notes TEXT;

UPDATE amazon_orders
SET internal_notes = COALESCE(internal_notes, notes);

ALTER TABLE amazon_orders
DROP COLUMN IF EXISTS return_status,
DROP COLUMN IF EXISTS return_reason,
DROP COLUMN IF EXISTS return_follow_up_actions,
DROP COLUMN IF EXISTS return_notes,
DROP COLUMN IF EXISTS issue_status,
DROP COLUMN IF EXISTS issue_reason,
DROP COLUMN IF EXISTS issue_follow_up_actions,
DROP COLUMN IF EXISTS safety_claim,
DROP COLUMN IF EXISTS safety_claim_updated_at,
DROP COLUMN IF EXISTS safety_claim_notes,
DROP COLUMN IF EXISTS notes;

ALTER TABLE amazon_order_products
ADD COLUMN IF NOT EXISTS safety_claimed BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS safety_claimed_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS safety_claim_issues TEXT,
ADD COLUMN IF NOT EXISTS return_initiated BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS return_initiated_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS return_initiated_reason TEXT,
ADD COLUMN IF NOT EXISTS return_initiated_followup_action TEXT,
ADD COLUMN IF NOT EXISTS return_initiated_compromised BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS return_initiated_compromised_reason TEXT,
ADD COLUMN IF NOT EXISTS return_initiated_compromised_updated_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS other_issues BOOLEAN,
ADD COLUMN IF NOT EXISTS other_issues_reason TEXT,
ADD COLUMN IF NOT EXISTS other_issue_updated_at TIMESTAMP;

UPDATE amazon_order_products
SET
    safety_claimed = CASE
        WHEN safety_claim IN ('pending', 'done', 'issues') THEN TRUE
        WHEN safety_claim = 'not_needed' THEN FALSE
        ELSE safety_claimed
    END,
    safety_claimed_updated_at = COALESCE(safety_claimed_updated_at, safety_claim_updated_at),
    safety_claim_issues = COALESCE(safety_claim_issues, safety_claim_notes),
    return_initiated = CASE
        WHEN return_status <> 'none' THEN TRUE
        ELSE COALESCE(return_initiated, FALSE)
    END,
    return_initiated_updated_at = COALESCE(return_initiated_updated_at, return_status_updated_at),
    return_initiated_reason = COALESCE(return_initiated_reason, return_reason),
    return_initiated_followup_action = COALESCE(return_initiated_followup_action, return_follow_up_actions),
    return_initiated_compromised = CASE
        WHEN issue_status = 'has_issues' THEN TRUE
        ELSE return_initiated_compromised
    END,
    return_initiated_compromised_reason = COALESCE(return_initiated_compromised_reason, issue_reason),
    return_initiated_compromised_updated_at = COALESCE(return_initiated_compromised_updated_at, issue_status_updated_at),
    other_issues = CASE
        WHEN issue_status <> 'none' THEN TRUE
        ELSE other_issues
    END,
    other_issues_reason = COALESCE(other_issues_reason, issue_follow_up_actions, issue_reason),
    other_issue_updated_at = COALESCE(other_issue_updated_at, issue_status_updated_at);

ALTER TABLE amazon_order_products
ALTER COLUMN return_initiated SET DEFAULT FALSE;

ALTER TABLE amazon_order_products
DROP COLUMN IF EXISTS return_status,
DROP COLUMN IF EXISTS return_status_updated_at,
DROP COLUMN IF EXISTS return_reason,
DROP COLUMN IF EXISTS return_follow_up_actions,
DROP COLUMN IF EXISTS return_notes,
DROP COLUMN IF EXISTS issue_status,
DROP COLUMN IF EXISTS issue_status_updated_at,
DROP COLUMN IF EXISTS issue_reason,
DROP COLUMN IF EXISTS issue_follow_up_actions,
DROP COLUMN IF EXISTS safety_claim,
DROP COLUMN IF EXISTS safety_claim_updated_at,
DROP COLUMN IF EXISTS safety_claim_notes;

UPDATE amazon_orders
SET order_status = CASE
    WHEN order_status IN ('processing', 'sent', 'manufactured') THEN 'manufactured'
    WHEN order_status IN ('returned') THEN 'returned'
    WHEN order_status = 'cancelled' THEN 'cancelled'
    ELSE 'received'
END;

ALTER TABLE amazon_orders
ALTER COLUMN order_status SET DEFAULT 'received';

ALTER TABLE amazon_orders
ADD CONSTRAINT valid_order_status CHECK (
    order_status IN ('received', 'manufactured', 'cancelled', 'returned')
);

DROP INDEX IF EXISTS idx_amazon_orders_return_status;
DROP INDEX IF EXISTS idx_amazon_orders_issue_status;
DROP INDEX IF EXISTS idx_amazon_orders_safety_claim;
DROP INDEX IF EXISTS idx_amazon_order_products_return_status;
DROP INDEX IF EXISTS idx_amazon_order_products_issue_status;
DROP INDEX IF EXISTS idx_amazon_order_products_safety_claim;

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_return_initiated
ON amazon_order_products(return_initiated);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_return_initiated_updated_at
ON amazon_order_products(return_initiated_updated_at);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_safety_claimed
ON amazon_order_products(safety_claimed);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_safety_claimed_updated_at
ON amazon_order_products(safety_claimed_updated_at);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_other_issues
ON amazon_order_products(other_issues);

CREATE INDEX IF NOT EXISTS idx_amazon_order_products_other_issue_updated_at
ON amazon_order_products(other_issue_updated_at);
