-- 1. Create Enums
CREATE TYPE user_role AS ENUM ('admin', 'cashier');
CREATE TYPE payment_method AS ENUM ('qris', 'va', 'ewallet', 'cash');
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'expired');
CREATE TYPE order_source AS ENUM ('qr', 'cashier');
CREATE TYPE order_status AS ENUM ('pending', 'confirmed', 'preparing', 'ready', 'completed', 'cancelled');

-- 2. Create Tables
CREATE TABLE "users" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" varchar(100),
  "username" varchar(50) UNIQUE,
  "password" varchar(255),
  "role" user_role,
  "is_active" boolean DEFAULT true,
  "created_at" timestamp DEFAULT now()
);

CREATE TABLE "tables" (
  "id" BIGSERIAL PRIMARY KEY,
  "number" varchar(20) UNIQUE,
  "qr_token" varchar(64) UNIQUE,
  "is_active" boolean DEFAULT true
);

CREATE TABLE "categories" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" varchar(100)
);

CREATE TABLE "products" (
  "id" BIGSERIAL PRIMARY KEY,
  "category_id" bigint REFERENCES "categories"("id"),
  "name" varchar(150),
  "base_price" decimal(12,2),
  "stock" int,
  "is_active" boolean DEFAULT true
);

CREATE TABLE "product_addons" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" varchar(100),
  "price" decimal(12,2),
  "stock" int,
  "is_active" boolean DEFAULT true
);

CREATE TABLE "product_addon_map" (
  "id" BIGSERIAL PRIMARY KEY,
  "product_id" bigint REFERENCES "products"("id"),
  "product_addon_id" bigint REFERENCES "product_addons"("id")
);

CREATE TABLE "payment_transactions" (
  "id" BIGSERIAL PRIMARY KEY,
  "payment_ref" varchar(100) UNIQUE,
  "external_id" varchar(100),
  "amount" decimal(14,2),
  "payment_method" payment_method,
  "provider" varchar(50),
  "status" payment_status,
  "handled_by_user_id" bigint REFERENCES "users"("id"),
  "user_agent" text,
  "ip_address" varchar(45),
  "payload" text,
  "paid_at" timestamp,
  "created_at" timestamp DEFAULT now()
);

CREATE TABLE "orders" (
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

CREATE TABLE "order_items" (
  "id" BIGSERIAL PRIMARY KEY,
  "order_id" bigint REFERENCES "orders"("id"),
  "product_id" bigint REFERENCES "products"("id"),
  "product_name" varchar(150),
  "unit_price" decimal(12,2),
  "quantity" int,
  "subtotal" decimal(14,2)
);

CREATE TABLE "order_item_addons" (
  "id" BIGSERIAL PRIMARY KEY,
  "order_item_id" bigint REFERENCES "order_items"("id"),
  "product_addon_id" bigint REFERENCES "product_addons"("id"),
  "addon_name" varchar(100),
  "addon_price" decimal(12,2),
  "quantity" int,
  "subtotal" decimal(14,2)
);