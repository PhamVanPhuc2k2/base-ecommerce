package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// AttributeSchemas trả tập thuộc tính hiệu lực của một danh mục: catalog định
// nghĩa (cache một khóa) + cây danh mục (cache sẵn từ P0.2) → Schema.
//
// Mọi use case ghi sản phẩm/variant validate qua đây (gián tiếp, qua
// ProductRules) — một chỗ duy nhất quyết định "danh mục này chặt hay tự do".
type AttributeSchemas struct {
	repo  AttributeRepository
	cache Cache
	tree  *GetCategoryTree
}

func NewAttributeSchemas(repo AttributeRepository, cache Cache, tree *GetCategoryTree) *AttributeSchemas {
	return &AttributeSchemas{repo: repo, cache: cache, tree: tree}
}

func (s *AttributeSchemas) Catalog(ctx context.Context) (*domain.AttributeCatalog, error) {
	return s.cache.AttributeCatalog(ctx, TTLAttributes, s.repo.Catalog)
}

func (s *AttributeSchemas) For(ctx context.Context, categoryID uuid.UUID) (*domain.Schema, error) {
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := s.tree.Tree(ctx)
	if err != nil {
		return nil, err
	}
	return catalog.SchemaFor(tree, categoryID), nil
}

// CategoryAttributes là tập hiệu lực công khai của một danh mục theo slug.
type CategoryAttributes struct {
	schemas *AttributeSchemas
}

func NewCategoryAttributes(schemas *AttributeSchemas) *CategoryAttributes {
	return &CategoryAttributes{schemas: schemas}
}

func (uc *CategoryAttributes) Execute(ctx context.Context, slug string) ([]domain.SchemaEntry, error) {
	tree, err := uc.schemas.tree.Tree(ctx)
	if err != nil {
		return nil, err
	}
	cat, ok := tree.BySlug(slug)
	if !ok {
		return nil, domain.ErrUnknownCategory
	}
	schema, err := uc.schemas.For(ctx, cat.ID)
	if err != nil {
		return nil, err
	}
	return schema.Entries(), nil
}

// ---- Quản trị định nghĩa ----
//
// Mọi lệnh ghi xóa khóa attribute:catalog SAU commit. Không xóa thì validate
// dùng định nghĩa cũ tới 6 giờ: thêm giá trị enum mới mà sản phẩm vẫn bị từ
// chối, người quản trị không hiểu vì sao.

type ListAttributes struct{ schemas *AttributeSchemas }

func NewListAttributes(schemas *AttributeSchemas) *ListAttributes {
	return &ListAttributes{schemas: schemas}
}

func (uc *ListAttributes) Execute(ctx context.Context) ([]*domain.AttributeDefinition, error) {
	c, err := uc.schemas.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	return c.Definitions, nil
}

type CreateAttributeInput struct {
	Code       string
	Name       string
	Type       domain.AttributeType
	Unit       string
	Options    []string
	Filterable bool
	Variant    bool
}

type AttributeAdmin struct {
	tx    TxManager
	repo  AttributeRepository
	cache Cache
}

func NewAttributeAdmin(tx TxManager, repo AttributeRepository, cache Cache) *AttributeAdmin {
	return &AttributeAdmin{tx: tx, repo: repo, cache: cache}
}

func (uc *AttributeAdmin) Create(ctx context.Context, in CreateAttributeInput) (*domain.AttributeDefinition, error) {
	d, err := domain.NewAttributeDefinition(in.Code, in.Name, in.Type, in.Unit, in.Options, in.Filterable, in.Variant)
	if err != nil {
		return nil, err
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error { return uc.repo.Insert(ctx, d) }); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return d, nil
}

type UpdateAttributeInput struct {
	ID         uuid.UUID
	Name       *string
	Unit       *string
	Options    []string
	Filterable *bool
}

func (uc *AttributeAdmin) Update(ctx context.Context, in UpdateAttributeInput) (*domain.AttributeDefinition, error) {
	var updated *domain.AttributeDefinition
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		d, err := uc.repo.ByID(ctx, in.ID)
		if err != nil {
			return err
		}
		if err := d.Update(in.Name, in.Unit, in.Options, in.Filterable); err != nil {
			return err
		}
		if err := uc.repo.Update(ctx, d); err != nil {
			return err
		}
		updated = d
		return nil
	}); err != nil {
		return nil, err
	}
	uc.invalidate(ctx)
	return updated, nil
}

func (uc *AttributeAdmin) Delete(ctx context.Context, id uuid.UUID) error {
	if err := uc.tx.Run(ctx, func(ctx context.Context) error { return uc.repo.Delete(ctx, id) }); err != nil {
		return err
	}
	uc.invalidate(ctx)
	return nil
}

// SetCategoryAttributes thay TOÀN BỘ phép gán của một danh mục.
//
// Trùng thuộc tính trong cùng một lệnh bị chặn ở đây thay vì để khóa chính
// báo: lỗi của khóa chính không nói được thuộc tính nào bị gửi hai lần.
func (uc *AttributeAdmin) SetCategoryAttributes(ctx context.Context, categoryID uuid.UUID,
	assigns []domain.CategoryAttribute) error {

	seen := map[uuid.UUID]bool{}
	for i := range assigns {
		if seen[assigns[i].AttributeID] {
			return domain.ErrDuplicateAttributeAssignment
		}
		seen[assigns[i].AttributeID] = true
		assigns[i].CategoryID = categoryID
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		return uc.repo.ReplaceAssignments(ctx, categoryID, assigns)
	}); err != nil {
		return err
	}
	uc.invalidate(ctx)
	return nil
}

func (uc *AttributeAdmin) invalidate(ctx context.Context) {
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyAttributeCatalog())
}
