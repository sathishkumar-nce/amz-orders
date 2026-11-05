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

The initial goose migration `db_migration/00001_initial_schema.sql` creates:

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
- direct-order tables in `db_migration/00001_initial_schema.sql`
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
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir db_migration postgres "postgres://postgres:postgres@localhost:5432/amz_orders?sslmode=disable" up
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

## DTDC and Delhivery direct-order scheduling

Create and save a direct order, then choose **Schedule Order** (or select orders and
choose **Schedule Selected Orders**). The booking dialog offers **Delhivery** and
**DTDC**. It defaults to the order's selected courier when DTDC is selected;
other orders default to Delhivery. Each bulk booking reports its own success or
failure, and successful bookings are not retried.

Configure the seller's DTDC account and pickup location using the variables in
[.env.dtdc.example](.env.dtdc.example). Add these to the backend `.env` for local
runs or the deployment environment used by Docker Compose, then restart/recreate
the backend. The API key and tracking token stay on the backend. Customer names,
addresses, pincodes, and phones are read from each direct order, so no environment
configuration is required per customer. `DTDC_SERVICE_TYPE` is the account's
approved service; it may be overridden in the DTDC booking dialog.

DTDC booking requires a full destination address, positive package dimensions in
cm, total weight in **grams**, and declared amount. The backend converts total
weight to kg. For multiple parcels the dialog collects each parcel's weight in
**kg**, dimensions, description, and declared value. COD uses `cod_amount`; zero
means prepaid. An optional 12-digit e-way bill and risk surcharge flag can be sent.
This creates the forward consignment and saves its AWB; a separate pickup-request
API is not part of the supplied DTDC integration.

Open **Courier Shipments** (`/courier-shipments`) from the Direct navigation or
Direct Orders. This paginated screen lists both DTDC and Delhivery shipments,
including cancelled shipments and bookings awaiting reconciliation. Search by
order, customer, mobile or AWB and filter by courier, saved courier status, or
created-date range. Click an AWB to fetch official tracking into the details panel.
Tracking scans and the last refresh time are saved. The existing DTDC-only screen
at `/dtdc-shipments` remains available, including page-wide tracking refresh.
Labels are available in 4x6, A4 and A6 PDF sizes. A successful DTDC booking downloads
its label automatically by default, with a manual download/retry button in the
results. Label failures do not retry booking. Browsers may require permission for
multiple downloads during bulk booking. Cancellation is only recorded
when DTDC confirms the matching AWB. Tracking refresh is on demand, not a
background polling job. `DTDC_TRACKING_TOKEN` is required only for live tracking.

An atomic database claim prevents duplicate bookings across both couriers,
including simultaneous clicks from different browsers. A timeout or ambiguous
carrier response sets `booking_review` (or leaves `booking_pending` after a crash)
and blocks retries. Verify the shipment with DTDC/Delhivery using the direct order
ID as the customer reference before reconciling the local record. Do not simply
clear this status and rebook. DTDC AWBs and carrier identity cannot be overwritten
with manual edits; DTDC shipments remain in the list when cancelled.

Authenticated DTDC APIs:

- `GET /api/v1/direct-orders/dtdc/settings` — configuration readiness, no secrets
- `GET /api/v1/direct-orders/dtdc/shipments` — filters and pagination
- `POST /api/v1/direct-orders/:order_id/dtdc/book` — JSON booking options (`{}` for defaults)
- `POST /api/v1/direct-orders/:order_id/dtdc/track` — refresh and persist tracking
- `POST /api/v1/direct-orders/:order_id/dtdc/reconcile` — recover a confirmed AWB by exact order reference
- `POST /api/v1/direct-orders/:order_id/dtdc/cancel` — cancel and persist confirmation
- `GET /api/v1/direct-orders/:order_id/dtdc/label?label_code=SHIP_LABEL_4X6` — PDF label
- `GET /api/v1/direct-orders/courier-settings` — saved runtime service modes and label preferences
- `PUT /api/v1/direct-orders/courier-settings` — update runtime preferences
- `GET /api/v1/direct-orders/shipments?courier=all` — DTDC and Delhivery shipments
- `POST /api/v1/direct-orders/:order_id/tracking` — official tracking and shared saved summary

Apply all current migrations before deploying the courier flows, including
`db-migrations/00020_courier_settings.sql` for persistent runtime preferences.
Booking and tracking continue to use the existing direct-order shipment fields.

### Runtime settings and customer tracking

Open **Tools → Courier Settings** (`/courier-settings`) to add, rename, disable or
remove DTDC service modes and choose the default service, commodity, label format,
and automatic label-download preference. Codes are normalized to uppercase and
must be unique; the default must be enabled. Changes are stored in PostgreSQL and
used by new bookings immediately, across backend instances. An already-open booking
dialog has a **Refresh service list** button. Adding a mode does not provision it
with DTDC: the account must be approved for the exact code. API credentials and
pickup configuration remain backend environment variables.

Open **Direct → Tracking & WhatsApp** (`/order-tracking`) to search **direct orders
only** by customer mobile, name, tracking ID or order ID. Formatted Indian phone
queries are supported and all matching orders are paginated, including unshipped
orders. Select an order or click its AWB to fetch the official DTDC or Delhivery
tracking API. The nearby panel shows status, location, scan history, available ETA,
and the last check time. Failed refreshes retain saved data and show an error.

Review the recipient and editable status message, then click **Open WhatsApp Web**.
The message includes the order, available tracking details, last check time and an
official courier tracking URL. The app opens WhatsApp Web's compose screen; the
staff member clicks Send there. No WhatsApp API or automatic message send is used.
Orders without AWBs can share their saved order status without inventing a tracking
link. These screens do not search or modify Amazon-imported orders.

### Run a DTDC staging lifecycle test

The DTDC pickup configuration is set to **Tekgien Softwares, 34-h, fourth lane,
swami nagar, salem road, Namakkal, Tamil Nadu, 637001**. The pickup phone is configured as **9345266494**. DTDC
account-specific values still need to be supplied. These DTDC values do not replace
Delhivery's configured pickup location or seller address.

Copy `.env.dtdc.staging.example` to `.env.dtdc.staging.local` and fill the
**staging-enabled** API key/token, customer code, service type, commodity and
pickup phone. From the backend directory run:

```bash
DTDC_RUN_STAGING_TESTS=1 go test ./internal/integrations/dtdc -run '^TestStagingLifecycle$' -count=1 -v
```

This opt-in test uses the real DTDC client. It creates a uniquely named staging
consignment with the seller as its test recipient, checks its PDF label, queries
tracking, and cancels the staging AWB in cleanup even if a later check fails.
A newly booked staging AWB may have no scans; use `DTDC_STAGE_TRACKING_AWB` for a
DTDC-provided staging AWB with tracking history. Missing scans are reported, not
claimed as a successful history check.

The test refuses production or unrecognized endpoints and redirects. It reads
only the dedicated staging file / supplied process environment, does not load the
application's production environment files, and does not touch the application
database. Normal `go test ./...` skips the external lifecycle test.

On 2026-11-03, non-booking requests to the two documented staging endpoints
rejected the credentials available in the workspace: booking returned HTTP 401
(`WRONG_API_KEY`), and tracking returned HTTP 401 (invalid authentication token).
The documented staging token-authentication endpoint also returned HTTP 403
(`Not Authorized`) for the supplied tracking credential pair.
Consequently no staging shipment was created and the live staging lifecycle has
not been verified. Valid staging credentials and the remaining seller account
fields are required to finish that check.

### DTDC booking dropdowns

The DTDC booking dialog uses the enabled modes saved in Courier Settings. Initial
modes include the environment's configured service, `B2C SMART EXPRESS`,
`B2C EXPRESS`, `B2B PREMIUM`, and the guide's `B2C PREMIUM` / `B2C PRIORITY` examples.
These examples do not prove that a service is enabled on the seller account.

The commodity dropdown uses the 87 IDs/names from DTDC's linked commodity
catalogue, retrieved on 2026-11-03. Examples include `84` (PLASTIC ITEMS), `43`
(HOUSEHOLD GOODS), and `7` (OTHERS). Source:
https://docs.google.com/spreadsheets/d/158LuKmF8mHXSQfXcSE-U_NVeUpz-O1LuNlc1ualKEeI/edit

`DTDC_SERVICE_TYPE` and `DTDC_COMMODITY` seed the seller defaults on first use;
saved runtime settings then take precedence. Selecting a service or commodity in
the booking dialog overrides the default for that booking. Account credentials,
customer code, and pickup details are still required. Customer codes and API
credentials are account-specific and have no generic dropdown values. Delhivery
does not use these DTDC selections.

### Verified production service for EO2440

On 2026-11-05, `B2C SMART EXPRESS` successfully created production AWB
`7D128718632`. DTDC then confirmed cancellation for that exact AWB. The backend
environment now defaults to `DTDC_SERVICE_TYPE=B2C SMART EXPRESS` and
`DTDC_COMMODITY=84` (PLASTIC ITEMS). Restart the backend to load these defaults.
The booking dialog includes the configured service in its service dropdown.

`B2C EXPRESS` was not attempted because the first requested service worked.
See [the production test record](docs/dtdc-production-check-2026-11-05.md) for
confirmation details and the limits of this test.

### Recover an uncertain DTDC booking

On DTDC Shipments, an order with an uncertain booking offers **Check booking with
DTDC**. This queries tracking by the saved order reference and attaches an AWB only
when DTDC returns that exact reference and a matching customer account (when the
account is present in the tracking response). It does not create another shipment.
A currently pending booking must be at least two minutes old before recovery.
Missing scans, mismatched references, and unavailable tracking leave it locked.

The original booking payload, selected service, carrier response, and failure reason
are retained for review. Confirmed cancellation remains authoritative while tracking
scans can still be refreshed: DTDC's tracking header may temporarily show an earlier
status after its cancellation API has confirmed success.

### Database integration tests

Set `DTDC_TEST_DATABASE_URL` to a separate local PostgreSQL database named
`codex_dtdc_test`, then run `go test ./...`. The tests create and remove their own
schema, apply the real migrations, and use local mocked carrier endpoints. They
verify booking persistence, tracking, labels, cancellation, recovery, duplicate
protection across both couriers, and existing Delhivery payload mappings. They
refuse any other database name or a non-local database host. Without this variable,
these database tests are skipped.

The Makefile uses `ENV_FILE=.env.docker` by default. Supply DTDC credentials in the
selected deployment environment before recreating the app container. The local
production credentials are stored in the Git-ignored `.env`; a deployment may use
that file explicitly with `ENV_FILE=.env`, provided its database/port values match
the deployment. Service restarts and remote deployment are separate from these
source changes.
