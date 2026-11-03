# Warehouse API

Portfolio backend for a warehouse and inventory management system. It uses Go, Fiber, and PostgreSQL. Local development does not require Docker, Redis, RustFS, or an external WhatsApp/OCR service.

## Requirements

- Go 1.25 or newer
- PostgreSQL 13 or newer
- Tesseract OCR with Indonesian and English language data for the SKU image endpoint

## Local setup

1. Create a local PostgreSQL database, for example `warehouse`.
2. Copy `.env.example` to `.env` and set `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and a unique `JWT_SECRET` (at least 32 characters). `.env` is local-only and ignored by Git.
3. Apply the schema migration:

   ```powershell
   psql -h localhost -p 5432 -U warehouse -d warehouse -f .\migrations\001_warehouse_auth_inventory.sql
   ```

4. Create the first admin manually. Generate a bcrypt password hash with `go run .\cmd\hash-password`, then insert the account:

   ```sql
   INSERT INTO warehouse.users (phone, password_hash, role)
   VALUES ('+628123456789', '<bcrypt-hash>', 'admin');
   ```

   Use an E.164 phone number. There is intentionally no public admin-registration endpoint.
5. Start the API:

   ```powershell
   go run .\cmd\server
   ```

The service reads `.env` from the backend root; operating-system environment variables override values in that file. The legacy `config/.env` is not loaded by the new service and is left untouched.

## Authentication

- Admin invitations: `POST /api/v1/auth/invitations` with `{ "phone": "+628..." }`.
- Registration: `POST /api/v1/auth/register` with `invitation_id`, `phone`, `otp`, and `password`.
- Login: `POST /api/v1/auth/login` with `phone` and `password`; this returns a login `challenge_id`.
- Verify login OTP: `POST /api/v1/auth/otp/verify` with `challenge_id`, `phone`, and `otp`. The response contains a 15-minute bearer token.
- All warehouse APIs require `Authorization: Bearer <token>`. Only admins can invite users; invited users receive the `staff` role.
- Passwords must be 10-72 bytes. Phone numbers use E.164 format, for example `+628123456789`.

No WhatsApp provider has been selected yet. In `development`, the OTP sender writes the code and invitation reference to the application console. It is explicitly disabled in `production`, where startup fails until a real provider implements the `auth.Sender` interface. This prevents a production deployment from silently pretending to send WhatsApp messages.

## Warehouse endpoints

All endpoints below are under `/api/v1` and require a bearer token.

| Method | Path | Purpose |
| --- | --- | --- |
| GET, POST | `/products` | List/search and create products |
| GET, PUT, DELETE | `/products/:id` | Read, update, and deactivate a product |
| GET | `/products/sku/:sku` | Find a product by SKU |
| POST | `/products/scan` | Read SKU candidates from a JPEG/PNG image using local Tesseract |
| GET, POST | `/warehouses` | List and create warehouses |
| GET, PUT, DELETE | `/warehouses/:id` | Read, update, and deactivate a warehouse |
| GET, POST | `/suppliers` | List and create suppliers |
| GET, PUT, DELETE | `/suppliers/:id` | Read, update, and deactivate a supplier |
| GET | `/stock` | Read stock balances; accepts `product_id` and `warehouse_id` filters |
| GET | `/stock/low` | List balances below a product's minimum stock |
| GET | `/stock-movements` | Read the stock movement ledger |
| POST | `/stock-movements/inbound` | Receive stock into a warehouse |
| POST | `/stock-movements/outbound` | Issue stock from a warehouse |
| POST | `/stock-movements/transfer` | Transfer stock atomically between warehouses |
| POST | `/stock-movements/adjustment` | Set counted stock and record the difference |

Stock-changing operations use a PostgreSQL transaction, reject negative balances, and accept quantities with at most three decimal places. Product images sent to `/products/scan` are processed in memory and are not stored. Install Tesseract and its `ind` and `eng` trained data on the host; OCR returns recognized text and SKU-like candidates, which the client can use with `/products/sku/:sku`.

Example outbound request:

```json
{
  "product_id": "00000000-0000-0000-0000-000000000001",
  "from_warehouse_id": "00000000-0000-0000-0000-000000000002",
  "quantity": 3,
  "reference": "OUT-2026-001",
  "note": "Order fulfillment"
}
```

For inbound movements, provide `to_warehouse_id` and optionally `supplier_id`. Transfers require both `from_warehouse_id` and `to_warehouse_id`. Adjustments use `from_warehouse_id` and `counted_quantity`; the API records the delta as a stock movement.

## Operations

- `GET /healthz` checks the database connection.
- The PostgreSQL migration is explicit and is not run automatically at startup.
- The schema is `warehouse`; the configured `DB_SCHEMA` must match it.
- The API does not include container configuration yet. Containerization and a real WhatsApp sender can be added when those deployment choices are ready.
