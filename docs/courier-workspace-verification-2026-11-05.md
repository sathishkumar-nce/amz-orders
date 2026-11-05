# Courier workspace verification — 2026-11-05

Implemented runtime DTDC service modes and label preferences, a combined courier
shipment screen, and a direct-order customer tracking / WhatsApp Web workspace.
Migration `db-migrations/00020_courier_settings.sql` must be applied before running
the new backend. Frontend and backend builds must both be deployed.

## Checks completed

- `go test ./...` passed with an isolated local `codex_dtdc_test` PostgreSQL database.
  The integration suite applied all real migrations in its own schema and verified
  runtime mode persistence, booking with a newly added mode without a restart,
  disabled-mode rejection, normalized phone search, both courier shipment filters,
  shared tracking persistence, cancellation preservation, and existing booking,
  reconciliation and concurrency protections.
- Frontend TypeScript checks and the Vite production build passed.
- `npm run test:tracking` passed all five recipient, message, URL and cached
  tracking tests. New courier components, views, helpers and tests passed ESLint.
  Existing lint errors remain in unrelated portions of `DirectOrdersView.vue`
  and `DirectOrderFormView.vue` (unused variables and explicit `any` catches).
- Headless Chrome browser checks passed with mocked API responses: runtime mode
  add/save/reload; booking using the new mode; automatic PDF download in the saved
  size; label failure/retry without another booking; both-courier shipment filters;
  customer name, formatted phone and AWB search; live tracking panels; recipient and
  message encoding; WhatsApp Web popup; unshipped order sharing; slow-response
  protection; and 390px mobile layouts without horizontal overflow.
- The existing courier browser regression suite passed: DTDC and Delhivery
  selection and scheduling, single and bulk booking, multiple parcels, tracking,
  labels, cancellation, configuration failures and uncertain-booking recovery.

Browser courier calls and the WhatsApp popup were intercepted by test fixtures.
No WhatsApp message was sent, and these checks did not create production shipments.
The earlier authorized live DTDC booking/cancellation is recorded separately in
[the production check](dtdc-production-check-2026-11-05.md). Staging credentials
remain rejected, so a successful real staging lifecycle is not claimed.

Local validation used temporary services and a separate test database. Existing
application containers and their database were not rebuilt or migrated.
