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

Copy `.env.example` to `.env` and configure `APP_ENV`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `JWT_SECRET` (at least 32 characters). `DB_SSLMODE` is optional and defaults to `disable`. Cloudinary credentials are required in production; local image uploads also require them. Keep `.env` out of source control.

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
- `POST /api/v1/sku-requests` accepts a JSON `photo` data URI (JPEG, PNG, or WebP), uploads the image to Cloudinary, and stores the HTTPS URL. Requests remain pending until an owner reviews them.
- `GET /api/v1/sku-requests` returns all requests to owners and only the signed-in employee's requests to staff.
- `PATCH /api/v1/sku-requests/{id}/review` accepts `{"status":"approved"}` or `{"status":"rejected","review_note":"..."}`. Approving creates the product and stores its photo atomically.
- The web dashboard can read these photo URLs through the existing GET endpoints; image uploads are performed by the mobile app when it submits a SKU request.

## Run

```powershell
if (!(Test-Path .env)) { Copy-Item .env.example .env }
# Fill the database and JWT settings, then:
go run .\cmd\server
```

`GET /healthz` checks database availability. There are no public registration or OTP endpoints.

## Docker

With Docker Desktop running and `.env` filled with local database/JWT values:

```powershell
docker compose up --build
```

Compose and `go run` both read the `.env` file in `BE_web`. The API is at `http://localhost:8080`; PostgreSQL is available to host tools at port `5434`. On the first start, Compose initializes a fresh PostgreSQL volume using both SQL files under `migrations`. `GET http://localhost:8080/healthz` reports backend/database readiness. Stop containers with `docker compose down`; `docker compose down -v` also removes the local database volume and its data.

## Render

The `render.yaml` Blueprint deploys the Docker service and prompts for database and Cloudinary values that must remain private. Render supplies `PORT`; `SERVER_PORT` can still be used locally. Set `CORS_ORIGIN` to the deployed web dashboard's origin, and apply both SQL migrations to the configured PostgreSQL database before using the API. Set `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, and `CLOUDINARY_API_SECRET` in Render's environment settings. The upload folder defaults to `crud_image`.

Do not put API secrets in this repository. Since an API secret was shared in chat, rotate it in Cloudinary before deployment and use the replacement only in local ignored `.env` and Render's secret environment settings.

For the complete local Docker and Render checklist, see [DEPLOYMENT-NEXT-STEPS.md](./DEPLOYMENT-NEXT-STEPS.md).
