-- Drop dari ujung relasi ke induknya (anak dulu, baru master).

-- 1. Tabel transaksi
DROP TABLE IF EXISTS order_item_addons CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS payment_transactions CASCADE;

-- 2. Tabel jembatan produk
DROP TABLE IF EXISTS product_addon_map CASCADE;
DROP TABLE IF EXISTS product_addons CASCADE;

-- 3. Master data
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS tables CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 4. Enum types (harus setelah semua tabel yang memakainya dihapus)
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_source;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS user_role;