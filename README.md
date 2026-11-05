# Small Company API

Backend API for the ERD-based small-company application. It uses Go, Fiber, and PostgreSQL.

## Database setup

The database schema is defined in one migration file: `migrations/001_erd_database.sql`. It creates:

- `users`: `users`, `roles`, and `user_has_roles` for accounts and role assignment.
- `web_db`: `products`, `product_photos`, `sales`, and `stock_movements`.
- `mobile_db`: `devices`, linked to `users.users`.

Apply it to the target PostgreSQL database:

```powershell
psql -h localhost -p 5432 -U <db-user> -d <db-name> -f .\migrations\001_erd_database.sql
```

For an existing database created from the old warehouse schema, use a fresh database for this ERD or back up and migrate the old data separately. This SQL intentionally does not drop existing schemas or data.

Configure `.env` with `APP_ENV`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `JWT_SECRET` (at least 32 characters). `DB_SSLMODE` is optional and defaults to `disable`.

There is no public account-registration route. Create the initial owner account using a bcrypt hash:

```powershell
go run .\cmd\hash-password
```

Then insert the account and attach the `owner` role, replacing the example values:

```sql
WITH new_user AS (
    INSERT INTO users.users (name, email, password_hash)
    VALUES ('Owner', 'owner@example.com', '<bcrypt-hash>')
    RETURNING id
)
INSERT INTO users.user_has_roles (user_id, role_id)
SELECT new_user.id, roles.id
FROM new_user
JOIN users.roles ON roles.name = 'owner';
```

Emails are stored in lowercase. Supported roles are `owner` and `staff`.

## Authentication

- `POST /api/v1/auth/login` accepts `{"email":"owner@example.com","password":"..."}`.
- On success, the response includes a 15-minute bearer token and the user's `id`, `name`, `email`, and `roles`.
- Login reads roles through `users.user_has_roles`; role names are not stored directly on the user row.

## Run

```powershell
go run .\cmd\server
```

`GET /healthz` checks database availability. Legacy warehouse, invitation, registration, and OTP endpoints are not registered because they are outside the supplied ERD.
