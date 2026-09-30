package domain

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Permission là một quyền. Danh sách CỐ ĐỊNH trong code (chủ dự án chốt P2):
// thêm quyền là thêm tính năng, tức là sửa code — không có API tạo quyền.
type Permission string

const (
	PermCatalogProductsWrite Permission = "catalog.products.write"
	PermCatalogTaxonomyWrite Permission = "catalog.taxonomy.write"
	PermMediaUpload          Permission = "media.upload"
	PermIAMRolesManage       Permission = "iam.roles.manage"
)

// PermissionInfo mô tả một quyền cho màn hình quản trị.
type PermissionInfo struct {
	Code        Permission
	Description string
}

// AllPermissions là NGUỒN DUY NHẤT của danh sách quyền. Quyền mới phải thêm ở
// đây — vai trò chỉ được chứa quyền có trong danh sách.
var AllPermissions = []PermissionInfo{
	{PermCatalogProductsWrite, "Tạo, sửa, đăng bán sản phẩm; thêm và sửa phiên bản"},
	{PermCatalogTaxonomyWrite, "Quản lý danh mục, thương hiệu, định nghĩa thuộc tính"},
	{PermMediaUpload, "Tải ảnh lên kho"},
	{PermIAMRolesManage, "Quản lý vai trò và phân vai trò cho người dùng"},
}

func (p Permission) Valid() bool {
	return slices.ContainsFunc(AllPermissions, func(i PermissionInfo) bool { return i.Code == p })
}

// PermissionSet là tập quyền HIỆU LỰC của một người — hợp của mọi vai trò họ
// giữ. Super = có vai trò hệ thống: có mọi quyền, kể cả quyền thêm sau này.
type PermissionSet struct {
	Super bool
	Perms []Permission
}

func (s PermissionSet) Has(p Permission) bool {
	return s.Super || slices.Contains(s.Perms, p)
}

// List trả danh sách quyền đầy đủ (super → mọi quyền trong code), đã sắp.
func (s PermissionSet) List() []Permission {
	if s.Super {
		out := make([]Permission, len(AllPermissions))
		for i, p := range AllPermissions {
			out[i] = p.Code
		}
		return out
	}
	out := slices.Clone(s.Perms)
	slices.Sort(out)
	return slices.Compact(out)
}

// Validate cho cache: quyền lạ trong cache (dữ liệu cũ sau khi đổi tên quyền)
// thì bỏ cả bản cache và đọc lại.
func (s PermissionSet) Validate() error {
	for _, p := range s.Perms {
		if !p.Valid() {
			return ErrUnknownPermission
		}
	}
	return nil
}

var roleCodePattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// Role là một vai trò: tên + tập quyền. Vai trò hệ thống (super_admin) có mọi
// quyền và không sửa/xóa được.
type Role struct {
	ID          uuid.UUID
	Code        string
	Name        string
	IsSystem    bool
	Permissions []Permission
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewRole(code, name string, perms []Permission) (*Role, error) {
	code = strings.TrimSpace(code)
	if !roleCodePattern.MatchString(code) || len(code) > 50 {
		return nil, ErrInvalidRoleCode
	}
	r := &Role{Code: code}
	if err := r.Update(&name, perms); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	r.ID = id
	r.CreatedAt = r.UpdatedAt
	return r, nil
}

// Update sửa tên và/hoặc tập quyền (nil = giữ nguyên).
func (r *Role) Update(name *string, perms []Permission) error {
	if r.IsSystem {
		return ErrSystemRoleImmutable
	}
	if name != nil {
		n, err := normalizeTaxonomyName(*name, ErrRoleNameInvalid)
		if err != nil {
			return err
		}
		r.Name = n
	}
	if perms != nil {
		out := make([]Permission, 0, len(perms))
		for _, p := range perms {
			if !p.Valid() {
				return ErrUnknownPermission
			}
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		r.Permissions = out
	}
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *Role) CheckDeletable() error {
	if r.IsSystem {
		return ErrSystemRoleImmutable
	}
	return nil
}
