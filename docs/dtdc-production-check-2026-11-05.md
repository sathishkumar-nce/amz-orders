# DTDC production booking and cancellation — 2026-11-05

The seller authorized a production test with customer code `EO2440`, trying
`B2C SMART EXPRESS` or `B2C EXPRESS`. The first service succeeded, so a second
shipment was not attempted.

- Service: **B2C SMART EXPRESS**
- Customer reference: `DTDC-TEST-20261105060038-S`
- AWB: **7D128718632**
- Booking: HTTP 200, `status: OK`, consignment `success: true`
- Cancellation: HTTP 200, `status: OK`, `success: true`, matching AWB `7D128718632`
- Result: **Booking confirmed, then cancellation confirmed**

The test used one prepaid parcel, 100 g, 10 × 10 × 1 cm, declared value ₹1,
commodity `84` (PLASTIC ITEMS), with Tekgien Softwares' Namakkal pickup address
also used as the recipient. Cancellation was the very next carrier call after
the successful booking. No application database order was created and no separate pickup API was called.

This verifies production consignment creation and cancellation for this service
and account. Shipping labels and shipment scan history were not tested in this run.

`B2C SMART EXPRESS` and commodity `84` are saved as DTDC defaults in the backend
environment configuration. Restart the backend to load them. The booking dialog
already includes the configured service in its service dropdown. Delhivery settings
were not changed.

The earlier `B2C PREMIUM` and `B2C PRIORITY` tests returned
`NO_SERIES_AVAILABLE`; their results remain in the November 3 report. This
successful test establishes a working service for EO2440 without establishing
that every service name is enabled for the account.

## Read-only follow-up checks

The production shipping-label endpoint returned HTTP 200 and a valid PDF (8,005
bytes) for cancelled AWB `7D128718632`. The production tracking endpoint returned
HTTP 200 with `statusFlag: true`, `status: SUCCESS`, the correct order reference,
and four scan events, including **Delivery Cancelled**. No new shipment was created.

The tracking header still showed **Pickup scheduled** after cancellation had
already been confirmed by the cancellation API. The application therefore retains
a confirmed cancellation while storing refreshed tracking scans.
