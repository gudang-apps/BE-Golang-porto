# Small Company API

Backend API for the ERD-based small-company application. It uses Go, Fiber, and PostgreSQL.

## Database setup

Apply the migrations in order:

- `migrations/001_erd_database.sql` creates:
- `users`: `users`, `roles`, and `user_has_roles` for accounts and role assignment.
- `web_db`: `products`, `product_photos`, `sales`, and `stock_movements`.
- `mobile_db`: `devices`, linked to `users.users`.
- `migrations/002_inventory_workflow.sql` adds scan history and SKU approval requests.

```powershell
psql -h localhost -p 5432 -U <db-user> -d <db-name> -f .\migrations\001_erd_database.sql
psql -h localhost -p 5432 -U <db-user> -d <db-name> -f .\migrations\002_inventory_workflow.sql
```

For an existing database created from an older warehouse schema, back up data and apply these migrations deliberately. They do not drop existing schemas or data.

Configure `.env` with `APP_ENV`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `JWT_SECRET` (at least 32 characters). `DB_SSLMODE` is optional and defaults to `disable`.

## Initial accounts

There is no public registration route. Create one admin (`owner`) and one warehouse employee (`staff`) account after applying both migrations. Set unique emails and strong passwords in the current PowerShell session:

```powershell
$env:GUKIT_ADMIN_EMAIL = "admin@example.com"
$env:GUKIT_ADMIN_PASSWORD = "<strong-password-at-least-12-characters>"
$env:GUKIT_STAFF_EMAIL = "gudang@example.com"
$env:GUKIT_STAFF_PASSWORD = "<another-strong-password>"
go run .\cmd\bootstrap-accounts
```

The bootstrap command hashes passwords with bcrypt and fails if an email already exists. It creates no public registration endpoint. For manual account provisioning, generate hashes with:

```powershell
go run .\cmd\hash-password
```

Emails are stored in lowercase. Supported roles are `owner` and `staff`.

## Authentication

- `POST /api/v1/auth/login` accepts `{"email":"admin@example.com","password":"..."}`.
- On success, the response includes an 8-hour bearer token and the user's `id`, `name`, `email`, and `roles`.
- Login reads roles through `users.user_has_roles`; role names are not stored directly on the user row.
- The Android app requires the `staff` role. The web dashboard supports `staff` and `owner`; only `owner` can approve SKU requests or manage products.

## Inventory workflow

- `POST /api/v1/scans` records a barcode and quantity for the signed-in employee.
- `POST /api/v1/sku-requests` submits a new SKU and product photo. Requests remain pending until an owner reviews them.
- `GET /api/v1/sku-requests` returns all requests to owners and only the signed-in employee's requests to staff.
- `PATCH /api/v1/sku-requests/{id}/review` accepts `{"status":"approved"}` or `{"status":"rejected","review_note":"..."}`. Approving creates the product and stores its photo atomically.

## Run

```powershell
go run .\cmd\server
```

`GET /healthz` checks database availability. There are no public registration or OTP endpoints.
