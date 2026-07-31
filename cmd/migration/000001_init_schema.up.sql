-- 1. Create Enums
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('admin', 'cashier');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_method') THEN
        CREATE TYPE payment_method AS ENUM ('qris', 'va', 'ewallet', 'cash');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_status') THEN
        CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'expired');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_source') THEN
        CREATE TYPE order_source AS ENUM ('qr', 'cashier');
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_status') THEN
        CREATE TYPE order_status AS ENUM ('pending', 'confirmed', 'preparing', 'ready', 'completed', 'cancelled');
    END IF;
END$$;

-- 2. Create Tables
CREATE TABLE IF NOT EXISTS "users" (
  "id" BIGSERIAL PRIMARY KEY,
  "email" varchar(100),
  "username" varchar(50) UNIQUE,
  "password" varchar(255),
  "role" user_role,
  "is_active" boolean DEFAULT true,
  "created_at" timestamp DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "tables" (
  "id" BIGSERIAL PRIMARY KEY,
  "number" varchar(20) UNIQUE,
  "qr_token" varchar(64) UNIQUE,
  "is_active" boolean DEFAULT true
);

CREATE TABLE IF NOT EXISTS "categories" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" varchar(100)
);

CREATE TABLE IF NOT EXISTS "products" (
  "id" BIGSERIAL PRIMARY KEY,
  "category_id" bigint REFERENCES "categories"("id"),
  "name" varchar(150),
  "base_price" decimal(12,2),
  "stock" int,
  "is_active" boolean DEFAULT true
);

CREATE TABLE IF NOT EXISTS "product_addons" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" varchar(100),
  "price" decimal(12,2),
  "stock" int,
  "is_active" boolean DEFAULT true
);

CREATE TABLE IF NOT EXISTS "product_addon_map" (
  "id" BIGSERIAL PRIMARY KEY,
  "product_id" bigint REFERENCES "products"("id"),
  "product_addon_id" bigint REFERENCES "product_addons"("id")
);

CREATE TABLE IF NOT EXISTS "payment_transactions" (
  "id" BIGSERIAL PRIMARY KEY,
  "payment_ref" varchar(100) UNIQUE,
  "external_id" varchar(100),
  "amount" decimal(14,2),
  "payment_method" payment_method,
  "provider" varchar(50),
  "status" payment_status,
  "customer_email" varchar(255),
  "handled_by_user_id" bigint REFERENCES "users"("id"),
  "user_agent" text,
  "ip_address" varchar(45),
  "payload" text,
  "paid_at" timestamp,
  "created_at" timestamp DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "orders" (
  "id" BIGSERIAL PRIMARY KEY,
  "order_number" varchar(20) UNIQUE,
  "table_id" bigint REFERENCES "tables"("id"),
  "customer_name" varchar(100),
  "payment_id" bigint REFERENCES "payment_transactions"("id"),
  "source" order_source,
  "created_by_user_id" bigint REFERENCES "users"("id"),
  "status" order_status,
  "subtotal" decimal(14,2),
  "created_at" timestamp DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "order_items" (
  "id" BIGSERIAL PRIMARY KEY,
  "order_id" bigint REFERENCES "orders"("id"),
  "product_id" bigint REFERENCES "products"("id"),
  "product_name" varchar(150),
  "unit_price" decimal(12,2),
  "quantity" int,
  "subtotal" decimal(14,2)
);

CREATE TABLE IF NOT EXISTS "order_item_addons" (
  "id" BIGSERIAL PRIMARY KEY,
  "order_item_id" bigint REFERENCES "order_items"("id"),
  "product_addon_id" bigint REFERENCES "product_addons"("id"),
  "addon_name" varchar(100),
  "addon_price" decimal(12,2),
  "quantity" int,
  "subtotal" decimal(14,2)
);

CREATE SEQUENCE IF NOT EXISTS order_number_seq START 1;
CREATE SEQUENCE IF NOT EXISTS payment_ref_seq  START 1;