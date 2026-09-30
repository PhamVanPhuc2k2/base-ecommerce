package pgstore

import (
	"context"
	"errors"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RoleRepository struct{ db *postgres.Manager }

func NewRoleRepository(db *postgres.Manager) *RoleRepository { return &RoleRepository{db: db} }

func (r *RoleRepository) List(ctx context.Context) ([]*domain.Role, error) {
	q := gen.New(r.db.DB(ctx))
	rows, err := q.AllRoles(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	perms, err := q.AllRolePermissions(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	byRole := map[uuid.UUID][]domain.Permission{}
	for _, p := range perms {
		byRole[p.RoleID] = append(byRole[p.RoleID], domain.Permission(p.Permission))
	}
	out := make([]*domain.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, roleToDomain(row, byRole[row.ID]))
	}
	return out, nil
}

func (r *RoleRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	q := gen.New(r.db.DB(ctx))
	row, err := q.RoleByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnknownRole
	}
	if err != nil {
		return nil, mapErr(err)
	}
	ps, err := q.RolePermissionsOf(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	perms := make([]domain.Permission, len(ps))
	for i, p := range ps {
		perms[i] = domain.Permission(p)
	}
	return roleToDomain(row, perms), nil
}

func (r *RoleRepository) Insert(ctx context.Context, role *domain.Role) error {
	q := gen.New(r.db.DB(ctx))
	err := q.InsertRole(ctx, gen.InsertRoleParams{ID: role.ID, Code: role.Code, Name: role.Name,
		CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "roles_code_key" {
		return domain.ErrDuplicateRoleCode
	}
	if err != nil {
		return mapErr(err)
	}
	return r.writePermissions(ctx, q, role)
}

// Update ghi tên và THAY TOÀN BỘ tập quyền. Phải chạy trong transaction: xóa
// xong chưa kịp chèn lại mà hỏng thì vai trò mất sạch quyền.
func (r *RoleRepository) Update(ctx context.Context, role *domain.Role) error {
	q := gen.New(r.db.DB(ctx))
	if err := q.UpdateRoleName(ctx, gen.UpdateRoleNameParams{ID: role.ID, Name: role.Name, UpdatedAt: role.UpdatedAt}); err != nil {
		return mapErr(err)
	}
	if err := q.DeleteRolePermissions(ctx, role.ID); err != nil {
		return mapErr(err)
	}
	return r.writePermissions(ctx, q, role)
}

func (r *RoleRepository) writePermissions(ctx context.Context, q *gen.Queries, role *domain.Role) error {
	for _, p := range role.Permissions {
		if err := q.InsertRolePermission(ctx, gen.InsertRolePermissionParams{RoleID: role.ID, Permission: string(p)}); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := gen.New(r.db.DB(ctx)).DeleteRole(ctx, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "user_roles_role_id_fkey" {
		return domain.ErrRoleInUse
	}
	if err != nil {
		return mapErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownRole
	}
	return nil
}

// PermissionsOf gộp quyền của mọi vai trò người dùng đang giữ. Tài khoản bị
// khóa → tập rỗng (câu SQL lọc status = 'active').
func (r *RoleRepository) PermissionsOf(ctx context.Context, userID uuid.UUID) (domain.PermissionSet, error) {
	rows, err := gen.New(r.db.DB(ctx)).UserPermissionRows(ctx, userID)
	if err != nil {
		return domain.PermissionSet{}, mapErr(err)
	}
	set := domain.PermissionSet{Perms: []domain.Permission{}}
	for _, row := range rows {
		if row.IsSystem {
			set.Super = true
		}
		if row.Permission != nil {
			set.Perms = append(set.Perms, domain.Permission(*row.Permission))
		}
	}
	return set, nil
}

func (r *RoleRepository) UsersWithRole(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := gen.New(r.db.DB(ctx)).UsersWithRole(ctx, roleID)
	return ids, mapErr(err)
}

func (r *RoleRepository) RoleIDByCode(ctx context.Context, code string) (uuid.UUID, error) {
	id, err := gen.New(r.db.DB(ctx)).RoleIDByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrRoleNotFound
	}
	return id, mapErr(err)
}

// SetUserRoles thay TOÀN BỘ vai trò của người dùng — trong transaction.
func (r *RoleRepository) SetUserRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	q := gen.New(r.db.DB(ctx))
	if err := q.DeleteUserRoles(ctx, userID); err != nil {
		return mapErr(err)
	}
	for _, id := range roleIDs {
		err := q.InsertUserRole(ctx, gen.InsertUserRoleParams{UserID: userID, RoleID: id})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23503" && pgErr.ConstraintName == "user_roles_role_id_fkey":
				return domain.ErrRoleNotFound
			case pgErr.Code == "23503" && pgErr.ConstraintName == "user_roles_user_id_fkey":
				return domain.ErrUnknownUser
			case pgErr.Code == "23505":
				continue // gửi trùng một vai trò hai lần — bỏ qua bản thứ hai
			}
		}
		if err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func roleToDomain(r gen.Role, perms []domain.Permission) *domain.Role {
	if perms == nil {
		perms = []domain.Permission{}
	}
	return &domain.Role{ID: r.ID, Code: r.Code, Name: r.Name, IsSystem: r.IsSystem, Permissions: perms,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
}

// UserRoleIDs trả các vai trò người dùng đang giữ — cho admintool cộng thêm vai
// trò mà không xóa những vai trò đã có.
func (r *RoleRepository) UserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := gen.New(r.db.DB(ctx)).UserRoleIDs(ctx, userID)
	return ids, mapErr(err)
}
