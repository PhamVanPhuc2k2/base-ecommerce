-- P1.3 — định nghĩa thuộc tính và gán thuộc tính cho danh mục.
--
-- CHỈ THÊM bảng mới. Giá trị thuộc tính vẫn nằm trong products.attributes và
-- product_variants.options (JSONB) — bảng ở đây chỉ nói khóa nào hợp lệ ở danh
-- mục nào, kiểu gì. Xem đặc tả P1.3 mục 2.1 về vì sao không chuyển sang EAV.

-- +goose Up
CREATE TABLE attribute_definitions (
    id         UUID        PRIMARY KEY,
    -- code là khóa trong JSONB: {"ram": "16"}. Chữ thường, số, gạch dưới —
    -- đúng thứ an toàn để làm tên tham số URL (?attr.ram=16).
    code       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    type       TEXT        NOT NULL,
    unit       TEXT        NOT NULL DEFAULT '',
    -- Giá trị cho phép của kiểu enum. Mảng rỗng với mọi kiểu khác.
    options    TEXT[]      NOT NULL DEFAULT '{}',
    filterable BOOLEAN     NOT NULL DEFAULT false,
    variant    BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT attribute_definitions_code_key UNIQUE (code),
    CONSTRAINT attribute_definitions_code_format CHECK (code ~ '^[a-z0-9]+(_[a-z0-9]+)*$'),
    CONSTRAINT attribute_definitions_type_check CHECK (type IN ('text', 'number', 'boolean', 'enum')),
    -- enum PHẢI có danh sách giá trị, kiểu khác KHÔNG được có. Domain kiểm
    -- trước; ràng buộc ở đây chặn SQL viết tay.
    CONSTRAINT attribute_definitions_enum_options CHECK (
        (type = 'enum') = (cardinality(options) > 0)
    )
);

CREATE TABLE category_attributes (
    -- CASCADE: xóa danh mục (chỉ xóa được khi rỗng) thì phép gán không còn
    -- nghĩa gì. Ngược lại định nghĩa thì RESTRICT: xóa một thuộc tính đang dùng
    -- ở danh mục nào đó phải bị chặn và báo rõ (ATTRIBUTE_IN_USE).
    category_id  UUID    NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    attribute_id UUID    NOT NULL REFERENCES attribute_definitions(id) ON DELETE RESTRICT,
    required     BOOLEAN NOT NULL DEFAULT false,
    position     INT     NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, attribute_id)
);

-- Khóa ngoại RESTRICT kiểm "còn ai trỏ tới định nghĩa này không" bằng cách tìm
-- theo attribute_id; khóa chính bắt đầu bằng category_id nên không giúp được.
CREATE INDEX category_attributes_attribute_idx ON category_attributes (attribute_id);

-- Lọc theo tùy chọn biến thể (EXISTS ... options @> ...) — cùng loại index với
-- products_attrs_idx đã có cho thuộc tính sản phẩm.
CREATE INDEX product_variants_options_idx ON product_variants USING gin (options);

-- +goose Down
DROP INDEX IF EXISTS product_variants_options_idx;
DROP TABLE IF EXISTS category_attributes;
DROP TABLE IF EXISTS attribute_definitions;
