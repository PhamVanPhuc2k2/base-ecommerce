-- +goose Up
CREATE TABLE categories (
    id         UUID PRIMARY KEY,
    parent_id  UUID REFERENCES categories(id) ON DELETE RESTRICT,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    position   INT  NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX categories_parent_idx ON categories (parent_id);

CREATE TABLE brands (
    id         UUID PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE products (
    id                UUID PRIMARY KEY,
    sku               TEXT NOT NULL,
    slug              TEXT NOT NULL,
    name              TEXT NOT NULL,
    short_description TEXT NOT NULL DEFAULT '',
    category_id       UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    brand_id          UUID NOT NULL REFERENCES brands(id)     ON DELETE RESTRICT,
    price             NUMERIC(15,2) NOT NULL CHECK (price >= 0),
    currency          TEXT NOT NULL DEFAULT 'VND',
    status            TEXT NOT NULL CHECK (status IN ('draft','live','archived')),
    attributes        JSONB NOT NULL DEFAULT '{}',
    images            TEXT[] NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    deleted_at        TIMESTAMPTZ
);

-- Partial unique: ràng buộc UNIQUE thường sẽ khóa luôn slug/sku của sản phẩm đã
-- xóa mềm, nghĩa là xóa xong không tạo lại được sản phẩm cùng slug.
CREATE UNIQUE INDEX products_slug_uq ON products (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX products_sku_uq  ON products (sku)  WHERE deleted_at IS NULL;

-- Truy vấn chính của trang danh mục.
CREATE INDEX products_category_live_idx ON products (category_id, price)
    WHERE deleted_at IS NULL AND status = 'live';

-- Lọc theo thuộc tính động: attributes @> '{"ram":"16GB"}'::jsonb
CREATE INDEX products_attrs_idx ON products USING gin (attributes);

-- +goose Down
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS categories;
