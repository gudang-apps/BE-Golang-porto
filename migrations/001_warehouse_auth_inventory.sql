BEGIN;

CREATE SCHEMA IF NOT EXISTS warehouse;

CREATE TABLE IF NOT EXISTS warehouse.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone varchar(16) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role varchar(16) NOT NULL DEFAULT 'staff' CHECK (role IN ('admin', 'staff')),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS warehouse.user_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone varchar(16) NOT NULL,
    role varchar(16) NOT NULL DEFAULT 'staff' CHECK (role = 'staff'),
    invited_by uuid NOT NULL REFERENCES warehouse.users(id),
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS user_invitations_phone_pending_idx
    ON warehouse.user_invitations (phone, expires_at) WHERE accepted_at IS NULL;

CREATE TABLE IF NOT EXISTS warehouse.otp_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone varchar(16) NOT NULL,
    purpose varchar(16) NOT NULL CHECK (purpose IN ('invite', 'login')),
    code_hash bytea NOT NULL,
    invitation_id uuid REFERENCES warehouse.user_invitations(id),
    user_id uuid REFERENCES warehouse.users(id),
    expires_at timestamptz NOT NULL,
    attempts smallint NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS otp_challenges_phone_pending_idx
    ON warehouse.otp_challenges (phone, purpose, expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS warehouse.products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sku varchar(100) NOT NULL UNIQUE,
    name varchar(200) NOT NULL,
    category varchar(100) NOT NULL DEFAULT '',
    unit varchar(40) NOT NULL,
    min_stock numeric(18,3) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS products_name_search_idx ON warehouse.products (lower(name));

CREATE TABLE IF NOT EXISTS warehouse.warehouses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(40) NOT NULL UNIQUE,
    name varchar(150) NOT NULL,
    location text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS warehouse.suppliers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(200) NOT NULL,
    contact_name varchar(150) NOT NULL DEFAULT '',
    phone varchar(40) NOT NULL DEFAULT '',
    email varchar(254) NOT NULL DEFAULT '',
    address text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS suppliers_name_search_idx ON warehouse.suppliers (lower(name));

CREATE TABLE IF NOT EXISTS warehouse.stock_balances (
    product_id uuid NOT NULL REFERENCES warehouse.products(id),
    warehouse_id uuid NOT NULL REFERENCES warehouse.warehouses(id),
    quantity numeric(18,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, warehouse_id)
);

CREATE TABLE IF NOT EXISTS warehouse.stock_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    movement_type varchar(16) NOT NULL CHECK (movement_type IN ('inbound', 'outbound', 'transfer', 'adjustment')),
    product_id uuid NOT NULL REFERENCES warehouse.products(id),
    quantity numeric(18,3) NOT NULL,
    from_warehouse_id uuid REFERENCES warehouse.warehouses(id),
    to_warehouse_id uuid REFERENCES warehouse.warehouses(id),
    supplier_id uuid REFERENCES warehouse.suppliers(id),
    reference varchar(100) NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES warehouse.users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (movement_type = 'inbound' AND quantity > 0 AND from_warehouse_id IS NULL AND to_warehouse_id IS NOT NULL)
        OR (movement_type = 'outbound' AND quantity > 0 AND from_warehouse_id IS NOT NULL AND to_warehouse_id IS NULL)
        OR (movement_type = 'transfer' AND quantity > 0 AND from_warehouse_id IS NOT NULL AND to_warehouse_id IS NOT NULL AND from_warehouse_id <> to_warehouse_id)
        OR (movement_type = 'adjustment' AND quantity <> 0 AND from_warehouse_id IS NOT NULL AND to_warehouse_id IS NULL)
    )
);
CREATE INDEX IF NOT EXISTS stock_movements_product_time_idx
    ON warehouse.stock_movements (product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS stock_movements_warehouse_time_idx
    ON warehouse.stock_movements (from_warehouse_id, to_warehouse_id, created_at DESC);

COMMIT;
