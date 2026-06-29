# Amazon Orders Service - Backend

Go backend for importing Amazon orders from BaseLinker, storing them in PostgreSQL, enriching them with SKU metadata, optionally syncing new orders to Google Sheets, and exposing JWT-protected APIs for operational workflows like issues, returns, and manual order updates.

## What This Service Does

- Starts a Gin HTTP API with JWT authentication
- Connects to PostgreSQL through `pgx/v5`
- Pulls confirmed orders from BaseLinker
- Converts BaseLinker payloads into local `amazon_orders` and `amazon_order_products` records
- Auto-populates size fields from `SKU_V8.csv`
- Preserves manual workflow fields so imports do not overwrite them
- Optionally pushes newly created orders to Google Sheets
- Runs a background scheduler to sync orders automatically
- Ensures a default admin user exists at startup

## Main Runtime Flow

```mermaid
flowchart TD
    A[Process starts<br/>cmd/server/main.go] --> B[Load .env + config]
    B --> C[Load SKU_V8.csv into in-memory SKU mapper]
    C --> D[Connect to PostgreSQL]
    D --> E[Create BaseLinker client]
    E --> F[Create repositories]
    F --> G[Ensure default admin user exists]
    G --> H[Try to create Google Sheets client]
    H --> I[Create order service + handlers]
    I --> J[Build Gin router + middleware]
    J --> K[Start HTTP server]
    J --> L[Start order sync scheduler]

    L --> M[First run: fetch last 6 hours]
    L --> N[Later runs: fetch last 3 hours]
    M --> O[BaseLinker getOrders]
    N --> O

    P[Manual POST /api/v1/orders/import] --> O
    Q[Manual POST /api/v1/orders/import-sample] --> R[Read local JSON sample]

    O --> S[Filter out recently imported order IDs]
    R --> T[Convert sample payload]
    S --> U[Convert BaseLinker order to AmazonOrder]
    T --> U
    U --> V[Pick first non-discount product as main SKU]
    V --> W[Enrich dimensions + is_round from SKU mapper]
    W --> X[Upsert order + products in DB transaction]
    X --> Y{Was this a new order?}
    Y -->|Yes + Sheets enabled| Z[Append row to Google Sheet]
    Y -->|No| AA[Finish import]
    Z --> AA

    AB[GET /api/v1/orders, /issues, /returns] --> AC[Parse query filters + pagination]
    AC --> AD[Repository builds SQL WHERE clause]
    AD --> AE[Fetch orders from PostgreSQL]
    AE --> AF[Return JSON response]

    AG[PATCH /api/v1/orders/:id/manual] --> AH[Validate enums]
    AH --> AI[Update manual workflow fields only]
    AI --> AF

    AJ[POST /api/v1/auth/login] --> AK[Verify bcrypt password]
    AK --> AL[Generate JWT]
    AL --> AF

    AM[Protected route request] --> AN[JWT middleware validates Bearer token]
    AN --> AB
```

## Architecture

### Entry Point

- `cmd/server/main.go`
- Loads config and environment variables
- Loads the SKU mapper from `SKU_V8.csv`
- Connects to Postgres
- Creates BaseLinker and Google Sheets integrations
- Builds repositories, services, handlers, router, and scheduler
- Starts the HTTP server and graceful shutdown flow

### Layers

- `internal/router`: route registration
- `internal/middleware`: logging, CORS, JWT auth
- `internal/handlers`: HTTP request/response layer
- `internal/service`: import/business orchestration
- `internal/repository`: database access and persistence logic
- `internal/integrations`: BaseLinker and Google Sheets clients
- `internal/utils`: JWT helpers, converters, SKU mapper
- `internal/models`: DB/API payload models

## Startup Sequence

At boot, the server does this in order:

1. Loads config from `.env` plus defaults.
2. Loads `SKU_V8.csv` into a singleton SKU mapper.
3. Opens a PostgreSQL pool and verifies connectivity.
4. Creates the BaseLinker API client.
5. Creates repositories.
6. Ensures the default admin account exists, and repairs a known legacy seed hash mismatch if needed.
7. Tries to enable Google Sheets sync using `spreadsheet.json` and `GOOGLE_SHEETS_ID`.
8. Creates the order service and auth/order handlers.
9. Registers routes and middleware.
10. Starts the scheduler and HTTP server.

## Import Pipeline

### Source

- BaseLinker live import: `POST /api/v1/orders/import`
- Local sample import: `POST /api/v1/orders/import-sample`

### Processing steps

1. Fetch or read raw orders.
2. Skip orders already found in the latest 200 stored `amazon_order_id` values.
3. Convert each BaseLinker order into one `amazon_orders` row.
4. Convert each BaseLinker product into `amazon_order_products` rows.
5. Ignore discount lines when deciding the “main product”.
6. Use the first non-discount product to fill `main_sku`, main product summary, and SKU-derived dimensions.
7. Write order + products in one DB transaction.
8. If the order is brand new and Google Sheets is enabled, append a row to the sheet.

### Import behavior worth knowing

- Imports only request confirmed orders from BaseLinker.
- Duplicate prevention is lightweight: it checks against the most recent 200 stored order IDs before processing.
- Order inserts use `ON CONFLICT ... DO NOTHING`, so existing orders are not overwritten during import.
- Manual business fields live in the order row and are updated through a separate endpoint.

## Manual Workflow Fields

These fields are managed by humans after import and are intentionally separated from BaseLinker ingestion:

- `default_width_in_inches`
- `default_length_in_inches`
- `default_width_in_mm`
- `default_length_in_mm`
- `customer_width_in_mm`
- `customer_length_in_mm`
- `corner_radius_and_notes`
- `return_status`
- `return_reason`
- `return_follow_up_actions`
- `return_notes`
- `issue_status`
- `issue_reason`
- `issue_follow_up_actions`
- `notes`
- `priority`
- `safety_claim`
- `safety_claim_notes`
- `is_round`

The service validates enum-style fields before writing:

- `return_status`: `none`, `returned`, `replacement_placed`, `converted_direct`, `refunded`
- `issue_status`: `none`, `has_issues`, `replacement_placed`, `converted_direct`, `refunded`
- `priority`: `p1`, `p2`, `p3`, `p4`
- `safety_claim`: `none`, `pending`, `done`, `not_needed`, `issues`

## Scheduler

The background scheduler lives in `internal/scheduler/order_sync_scheduler.go`.

- Starts automatically when the server starts
- Runs immediately once on boot
- First sync window: last 6 hours
- Later sync window: last 3 hours
- Repeats every `SYNC_INTERVAL_MINUTES` minutes
- Uses the same BaseLinker import pipeline as the manual import endpoint

## Google Sheets Sync

Google Sheets integration is optional.

- Enabled when credentials and sheet ID are available
- Triggered only for newly inserted orders
- Appends one row per new order to `Sheet1!A:G`

Row shape:

1. Customer name
2. Country code
3. Clean phone number
4. Amazon order ID
5. Formatted address
6. Formatted order details
7. Comma-separated SKU list

Relevant files:

- `internal/integrations/googlesheets/client.go`
- `internal/integrations/googlesheets/order_formatter.go`
- `internal/integrations/googlesheets/phone_cache.go`

## Authentication Flow

- Public routes: register and login
- Passwords are hashed with `bcrypt`
- Login returns a JWT signed with `JWT_SECRET`
- Protected routes require `Authorization: Bearer <token>`
- JWT middleware places `user_id` and `username` into Gin context
- `GET /api/v1/auth/me` returns the current user

## Active API Surface

These routes are currently registered by `internal/router/router.go`.

### Public

- `GET /health`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`

### Protected

- `GET /api/v1/auth/me`
- `POST /api/v1/orders/import`
- `POST /api/v1/orders/import-sample`
- `GET /api/v1/orders`
- `GET /api/v1/orders/:amazon_order_id`
- `PATCH /api/v1/orders/:amazon_order_id/manual`
- `GET /api/v1/issues`
- `GET /api/v1/returns`

## Filtering and Query Behavior

### `GET /api/v1/orders`

Supported filters in the current handler:

- `page`
- `limit`
- `amazon_order_id`
- `baselinker_order_id`
- `order_status_id`
- `confirmed`
- `main_sku`
- `phone`
- `delivery_city`
- `delivery_state`
- `return_status`
- `issue_status`
- `priority`
- `search`

Ordering:

- `date_confirmed DESC NULLS LAST`
- then `date_add DESC`

Pagination:

- default `page=1`
- default `limit=100`
- max `limit=200`

### `GET /api/v1/issues`

Defaults to `issue_status=has_issues`, with optional:

- `page`
- `limit`
- `issue_status`
- `priority`
- `delivery_state`

### `GET /api/v1/returns`

Supports:

- `page`
- `limit`
- `return_status`
- `priority`
- `delivery_state`

If `return_status` is not provided, the handler removes rows where `return_status = 'none'` from the result set after fetching.

## Database Shape

The migration file `db_migration/v1.sql` creates:

- `amazon_orders`
- `amazon_order_products`
- `users`
- `direct_orders`
- `direct_order_items`

It also adds:

- indexes for common filters
- triggers to auto-update `updated_at`
- enum-like check constraints for workflow fields

## Code Present But Not Currently Wired

The repo includes additional backend code that is not registered in the current router started by `cmd/server/main.go`:

- `internal/handlers/direct_order_handler.go`
- `internal/repository/direct_order_repository.go`
- direct-order tables in `db_migration/v1.sql`
- `internal/handlers/sku_handler.go`

So today:

- the codebase contains direct-order CRUD and CSV export support
- the schema contains direct-order tables
- but the running router does not expose `/api/v1/direct-orders` or SKU upload/download endpoints

## Configuration

### Required

```env
APP_PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/amz_orders?sslmode=disable
BASELINKER_API_URL=https://api.baselinker.com/connector.php
BASELINKER_TOKEN=your-token
JWT_SECRET=change-this
```

### Optional

```env
SYNC_INTERVAL_MINUTES=5
JWT_EXPIRATION_HOURS=24
DEFAULT_ADMIN_USERNAME=admin
DEFAULT_ADMIN_PASSWORD=admin123
DEFAULT_ADMIN_EMAIL=admin@example.com
SKU_CSV_PATH=./SKU_V8.csv
GOOGLE_SHEETS_CREDENTIALS=./spreadsheet.json
GOOGLE_SHEETS_ID=your-sheet-id
```

## Local Setup

### 1. Create environment file

```bash
cp .env.example .env
```

If `.env.example` is not present in your checkout, create `.env` manually with the variables above.

### 2. Start PostgreSQL

```bash
docker run --name amz-orders-db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=amz_orders \
  -p 5432:5432 \
  -d postgres:16-alpine
```

### 3. Run migration

```bash
docker exec -i amz-orders-db psql -U postgres -d amz_orders < db_migration/v1.sql
```

### 4. Install dependencies

```bash
go mod download
```

### 5. Run the service

```bash
go run cmd/server/main.go
```

## Useful API Examples

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

### Import from BaseLinker

```bash
curl -X POST http://localhost:8080/api/v1/orders/import \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"date_confirmed_from": 0}'
```

### Import from sample file

```bash
curl -X POST http://localhost:8080/api/v1/orders/import-sample \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"file_path":"./sample_orders.json"}'
```

### List orders

```bash
curl "http://localhost:8080/api/v1/orders?page=1&limit=100" \
  -H "Authorization: Bearer <token>"
```

### Search orders

```bash
curl "http://localhost:8080/api/v1/orders?search=MRC-MR" \
  -H "Authorization: Bearer <token>"
```

### Get one order

```bash
curl http://localhost:8080/api/v1/orders/403-0840172-6155531 \
  -H "Authorization: Bearer <token>"
```

### Update manual fields

```bash
curl -X PATCH http://localhost:8080/api/v1/orders/403-0840172-6155531/manual \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"priority":"p1","notes":"Urgent delivery","issue_status":"has_issues"}'
```

### List issues

```bash
curl "http://localhost:8080/api/v1/issues?priority=p1" \
  -H "Authorization: Bearer <token>"
```

### List returns

```bash
curl "http://localhost:8080/api/v1/returns?return_status=returned" \
  -H "Authorization: Bearer <token>"
```

## Verification

Current quick verification on this checkout:

- `go test ./...` passes
