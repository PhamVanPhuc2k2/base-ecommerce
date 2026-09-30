-- name: AllRoles :many
SELECT id, code, name, is_system, created_at, updated_at FROM roles ORDER BY is_system DESC, code;

-- name: AllRolePermissions :many
SELECT role_id, permission FROM role_permissions ORDER BY role_id, permission;

-- name: RolePermissionsOf :many
SELECT permission FROM role_permissions WHERE role_id = $1 ORDER BY permission;

-- name: RoleByID :one
-- FOR UPDATE: sửa vai trò là đọc rồi ghi đè cả tập quyền.
SELECT id, code, name, is_system, created_at, updated_at FROM roles WHERE id = $1 FOR UPDATE;

-- name: InsertRole :exec
INSERT INTO roles (id, code, name, is_system, created_at, updated_at) VALUES ($1, $2, $3, false, $4, $5);

-- name: UpdateRoleName :exec
UPDATE roles SET name = $2, updated_at = $3 WHERE id = $1 AND NOT is_system;

-- name: DeleteRole :execrows
DELETE FROM roles WHERE id = $1 AND NOT is_system;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: InsertRolePermission :exec
INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2);

-- name: UserPermissionRows :many
-- Mọi (vai trò hệ thống?, quyền) của người dùng ĐANG HOẠT ĐỘNG. Tài khoản bị
-- khóa không ra dòng nào → tập quyền rỗng → mọi /admin/* 403 ngay, không chờ
-- access token hết hạn.
SELECT r.is_system, rp.permission
FROM user_roles ur
JOIN users u ON u.id = ur.user_id AND u.status = 'active'
JOIN roles r ON r.id = ur.role_id
LEFT JOIN role_permissions rp ON rp.role_id = r.id
WHERE ur.user_id = $1;

-- name: UsersWithRole :many
SELECT user_id FROM user_roles WHERE role_id = $1;

-- name: UserRoleIDs :many
SELECT role_id FROM user_roles WHERE user_id = $1;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = $1;

-- name: InsertUserRole :exec
INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2);

-- name: RoleIDByCode :one
SELECT id FROM roles WHERE code = $1;
