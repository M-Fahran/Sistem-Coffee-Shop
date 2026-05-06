-- Hapus tabel jembatan (mapping) terlebih dahulu karena dia memiliki Foreign Key
DROP TABLE IF EXISTS product_addon_map CASCADE;

-- Baru hapus tabel masternya
DROP TABLE IF EXISTS product_addons CASCADE;

-- (Jika di file .up.sql Anda juga ada pembuatan tabel products, sertakan juga di sini)
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS tables CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS payment_transactions CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS order_item_addons CASCADE;