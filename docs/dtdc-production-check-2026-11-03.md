# DTDC production booking check — 2026-11-03

Customer code supplied by the seller: `EO2440`. The existing API key was used.

Tests called the application's real DTDC client against the production booking endpoint:
`https://pxapi.dtdc.in/api/customer/integration/consignment/softdata`

Both tests were prepaid, one 100 g parcel, 10 × 10 × 1 cm, declared value ₹1,
commodity `84` (PLASTIC ITEMS). The seller's Namakkal pickup address was also the
test destination. Successful bookings were set to be cancelled immediately, before
any other carrier operation. No application database orders were created.

| Service | Customer reference | HTTP | Shipment result | AWB |
| --- | --- | --- | --- | --- |
| B2C PREMIUM | `DTDC-TEST-20261103112919` | 200 | `NO_SERIES_AVAILABLE` (`success: false`) | None |
| B2C PRIORITY | `DTDC-PRIORITY-TEST-20261103113012` | 200 | `NO_SERIES_AVAILABLE` (`success: false`) | None |

DTDC accepted the HTTP request but explicitly rejected each consignment with
`reason: WRONG_INPUT`, `message: NO_SERIES_AVAILABLE`, and an empty
`reference_number`. No shipment was created, so cancellation was not invoked.
Booking/cancellation together remain unverified.

Ask DTDC support to check AWB/number-series allocation for customer `EO2440` and
confirm which service type can allocate an AWB automatically. If this account
requires preallocated AWBs, request its allocation process before another test.

The supplied tracking credential returned HTTP 206 (no data for a nonexistent
reference) in a separate production check. Actual shipment scan history has not
been verified. Delhivery configuration and the application database were unchanged.
