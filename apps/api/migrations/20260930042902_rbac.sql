-- P2.2 — phân quyền: vai trò, quyền của vai trò, gán vai trò cho người dùng.
--
-- Danh sách QUYỀN không nằm ở đây mà trong code (domain/permission.go) — chủ dự
-- án chốt "quyền trong code, vai trò trong DB". role_permissions.permission vì
-- vậy là TEXT không có khóa ngoại; domain kiểm giá trị hợp lệ lúc ghi.

-- +goose Up
CREATE TABLE roles (
    id         UUID        PRIMARY KEY,
    code       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    -- Vai trò hệ thống (super_admin): có MỌI quyền kể cả quyền thêm sau này, và
    -- không sửa/xóa được qua API. Đặc tả P2.2 mục 2.2.
    is_system  BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT roles_code_key UNIQUE (code),
    CONSTRAINT roles_code_format CHECK (code ~ '^[a-z0-9]+(_[a-z0-9]+)*$')
);

CREATE TABLE role_permissions (
    role_id    UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- RESTRICT: xóa vai trò còn người giữ phải bị chặn và báo rõ (ROLE_IN_USE),
    -- không âm thầm tước quyền của cả loạt người.
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, role_id)
);

-- Đổi quyền của một vai trò phải xóa cache quyền của MỌI người giữ nó — tìm
-- theo role_id; khóa chính bắt đầu bằng user_id nên không giúp được.
CREATE INDEX user_roles_role_idx ON user_roles (role_id);

-- ID cố định (UUIDv7 sinh một lần) để migration chạy lại ra đúng dữ liệu. Code
-- nhận ra vai trò này qua cờ is_system, không qua ID.
INSERT INTO roles (id, code, name, is_system, created_at, updated_at)
VALUES ('01a0f092-d79b-7120-983e-2cb4bc12d66e', 'super_admin', 'Quản trị cao nhất', true, now(), now());

-- +goose Down
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
