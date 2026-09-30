package usecase

import (
	"context"
	"time"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// TTLUserPerms: quyền của một người được cache 5 phút — nhưng MỌI thay đổi (gán
// vai trò, sửa quyền vai trò) xóa cache ngay, nên 5 phút chỉ là lưới an toàn
// khi lệnh xóa cache hỏng, không phải độ trễ bình thường. Đặc tả P2.2 mục 2.4.
const TTLUserPerms = 5 * time.Minute

func KeyUserPerms(userID uuid.UUID) string { return "iam:perms:" + userID.String() }

type RoleRepository interface {
	List(ctx context.Context) ([]*domain.Role, error)
	// ByID khóa dòng (FOR UPDATE).
	ByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	Insert(ctx context.Context, r *domain.Role) error
	Update(ctx context.Context, r *domain.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	PermissionsOf(ctx context.Context, userID uuid.UUID) (domain.PermissionSet, error)
	UsersWithRole(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error)
	SetUserRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error
}

// Authorizer trả lời "người này có quyền X không" — mỗi request, qua cache.
type Authorizer struct {
	roles RoleRepository
	cache Cache
}

func NewAuthorizer(roles RoleRepository, cache Cache) *Authorizer {
	return &Authorizer{roles: roles, cache: cache}
}

func (a *Authorizer) Permissions(ctx context.Context, userID uuid.UUID) (domain.PermissionSet, error) {
	return a.cache.UserPermissions(ctx, KeyUserPerms(userID), TTLUserPerms,
		func(ctx context.Context) (domain.PermissionSet, error) { return a.roles.PermissionsOf(ctx, userID) })
}

func (a *Authorizer) Require(ctx context.Context, userID uuid.UUID, p domain.Permission) error {
	set, err := a.Permissions(ctx, userID)
	if err != nil {
		return err
	}
	if !set.Has(p) {
		return domain.ErrForbidden
	}
	return nil
}

// RoleAdmin: quản trị vai trò và gán vai trò.
type RoleAdmin struct {
	tx    TxManager
	roles RoleRepository
	cache Cache
}

func NewRoleAdmin(tx TxManager, roles RoleRepository, cache Cache) *RoleAdmin {
	return &RoleAdmin{tx: tx, roles: roles, cache: cache}
}

func (uc *RoleAdmin) List(ctx context.Context) ([]*domain.Role, error) { return uc.roles.List(ctx) }

func (uc *RoleAdmin) Create(ctx context.Context, code, name string, perms []domain.Permission) (*domain.Role, error) {
	r, err := domain.NewRole(code, name, perms)
	if err != nil {
		return nil, err
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error { return uc.roles.Insert(ctx, r) }); err != nil {
		return nil, err
	}
	// Vai trò mới chưa ai giữ — không có cache nào phải xóa.
	return r, nil
}

func (uc *RoleAdmin) Update(ctx context.Context, id uuid.UUID, name *string, perms []domain.Permission) (*domain.Role, error) {
	var (
		updated *domain.Role
		holders []uuid.UUID
	)
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		r, err := uc.roles.ByID(ctx, id)
		if err != nil {
			return err
		}
		if err := r.Update(name, perms); err != nil {
			return err
		}
		if err := uc.roles.Update(ctx, r); err != nil {
			return err
		}
		updated = r
		holders, err = uc.roles.UsersWithRole(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	// Quyền của vai trò đổi → xóa cache của MỌI người đang giữ nó, để thêm hay
	// gỡ quyền có hiệu lực ở request kế tiếp chứ không sau 5 phút.
	uc.invalidate(ctx, holders...)
	return updated, nil
}

func (uc *RoleAdmin) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.tx.Run(ctx, func(ctx context.Context) error {
		r, err := uc.roles.ByID(ctx, id)
		if err != nil {
			return err
		}
		if err := r.CheckDeletable(); err != nil {
			return err
		}
		// Còn người giữ thì khóa ngoại RESTRICT chặn → ROLE_IN_USE. Không ai
		// giữ thì không có cache nào phải xóa.
		return uc.roles.Delete(ctx, id)
	})
}

// SetUserRoles thay toàn bộ vai trò của userID. actor là người đang thao tác.
func (uc *RoleAdmin) SetUserRoles(ctx context.Context, actor, userID uuid.UUID, roleIDs []uuid.UUID) error {
	// Không tự đổi vai trò của mình: super admin duy nhất tự gỡ vai trò là
	// không còn ai quản trị được. Đặc tả P2.2 mục 2.5.
	if actor == userID {
		return domain.ErrCannotChangeOwnRoles
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		return uc.roles.SetUserRoles(ctx, userID, roleIDs)
	}); err != nil {
		return err
	}
	uc.invalidate(ctx, userID)
	return nil
}

func (uc *RoleAdmin) invalidate(ctx context.Context, users ...uuid.UUID) {
	if len(users) == 0 {
		return
	}
	keys := make([]string, len(users))
	for i, u := range users {
		keys[i] = KeyUserPerms(u)
	}
	uc.cache.Invalidate(context.WithoutCancel(ctx), keys...)
}
