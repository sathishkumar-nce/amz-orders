import re

# Read the file
with open('internal/repository/order_repository.go', 'r') as f:
    content = f.read()

# Fix the order upsert - replace everything from ON CONFLICT to the closing query
order_pattern = r'(ON CONFLICT \(amazon_order_id\) DO) NOTHING\s+baselinker_order_id.*?updated_at = NOW\(\)'
order_replacement = r'\1 NOTHING'
content = re.sub(order_pattern, order_replacement, content, flags=re.DOTALL)

# Fix the product upsert
product_pattern = r'(ON CONFLICT \(order_product_id\) DO) NOTHING\s+amazon_order_id.*?updated_at = NOW\(\)'
product_replacement = r'\1 NOTHING'
content = re.sub(product_pattern, product_replacement, content, flags=re.DOTALL)

# Write back
with open('internal/repository/order_repository.go', 'w') as f:
    f.write(content)

print("Fixed ON CONFLICT clauses successfully")
