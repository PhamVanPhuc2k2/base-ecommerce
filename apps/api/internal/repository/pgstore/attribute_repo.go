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

type AttributeRepository struct{ db *postgres.Manager }

func NewAttributeRepository(db *postgres.Manager) *AttributeRepository {
	return &AttributeRepository{db: db}
}

// Catalog đọc TOÀN BỘ định nghĩa và phép gán — hai câu, vài trăm dòng. Use case
// cache kết quả dưới một khóa duy nhất (đặc tả P1.3 mục 2.6).
func (r *AttributeRepository) Catalog(ctx context.Context) (*domain.AttributeCatalog, error) {
	q := gen.New(r.db.DB(ctx))
	defs, err := q.AllAttributeDefinitions(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	assigns, err := q.AllCategoryAttributes(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	c := &domain.AttributeCatalog{
		Definitions: make([]*domain.AttributeDefinition, 0, len(defs)),
		Assignments: make([]domain.CategoryAttribute, 0, len(assigns)),
	}
	for _, d := range defs {
		c.Definitions = append(c.Definitions, attributeToDomain(d))
	}
	for _, a := range assigns {
		c.Assignments = append(c.Assignments, domain.CategoryAttribute{
			CategoryID: a.CategoryID, AttributeID: a.AttributeID,
			Required: a.Required, Position: int(a.Position),
		})
	}
	return c, nil
}

func (r *AttributeRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.AttributeDefinition, error) {
	row, err := gen.New(r.db.DB(ctx)).AttributeByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnknownAttribute
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return attributeToDomain(row), nil
}

func (r *AttributeRepository) Insert(ctx context.Context, d *domain.AttributeDefinition) error {
	return mapAttributeWriteErr(gen.New(r.db.DB(ctx)).InsertAttribute(ctx, gen.InsertAttributeParams{
		ID: d.ID, Code: d.Code, Name: d.Name, Type: string(d.Type), Unit: d.Unit,
		Options: d.Options, Filterable: d.Filterable, Variant: d.Variant,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}))
}

func (r *AttributeRepository) Update(ctx context.Context, d *domain.AttributeDefinition) error {
	n, err := gen.New(r.db.DB(ctx)).UpdateAttribute(ctx, gen.UpdateAttributeParams{
		ID: d.ID, Name: d.Name, Unit: d.Unit, Options: d.Options,
		Filterable: d.Filterable, UpdatedAt: d.UpdatedAt,
	})
	if err != nil {
		return mapAttributeWriteErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownAttribute
	}
	return nil
}

func (r *AttributeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := gen.New(r.db.DB(ctx)).DeleteAttribute(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" &&
			pgErr.ConstraintName == "category_attributes_attribute_id_fkey" {
			return domain.ErrAttributeInUse
		}
		return mapErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownAttribute
	}
	return nil
}

// ReplaceAssignments thay TOÀN BỘ phép gán của một danh mục: xóa hết rồi chèn
// lại. Phải chạy trong transaction — hỏng giữa chừng mà không rollback thì danh
// mục mất sạch thuộc tính và lặng lẽ rơi về chế độ tự do.
func (r *AttributeRepository) ReplaceAssignments(ctx context.Context, categoryID uuid.UUID,
	assigns []domain.CategoryAttribute) error {

	q := gen.New(r.db.DB(ctx))
	if err := q.DeleteCategoryAttributes(ctx, categoryID); err != nil {
		return mapErr(err)
	}
	for _, a := range assigns {
		err := q.InsertCategoryAttribute(ctx, gen.InsertCategoryAttributeParams{
			CategoryID: categoryID, AttributeID: a.AttributeID,
			Required: a.Required, Position: int32(a.Position),
		})
		if err == nil {
			continue
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23503" && pgErr.ConstraintName == "category_attributes_attribute_id_fkey":
				return domain.ErrAttributeNotFound
			case pgErr.Code == "23503" && pgErr.ConstraintName == "category_attributes_category_id_fkey":
				return domain.ErrUnknownCategory
			case pgErr.Code == "23505":
				return domain.ErrDuplicateAttributeAssignment
			}
		}
		return mapErr(err)
	}
	return nil
}

func mapAttributeWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) {
		return mapErr(err)
	}
	switch pgErr.ConstraintName {
	case "attribute_definitions_code_key":
		return domain.ErrDuplicateAttributeCode
	case "attribute_definitions_code_format":
		return domain.ErrInvalidAttributeCode
	case "attribute_definitions_type_check":
		return domain.ErrInvalidAttributeType
	case "attribute_definitions_enum_options":
		return domain.ErrInvalidAttributeOptions
	}
	return mapErr(err)
}

func attributeToDomain(r gen.AttributeDefinition) *domain.AttributeDefinition {
	opts := r.Options
	if opts == nil {
		opts = []string{}
	}
	return &domain.AttributeDefinition{
		ID: r.ID, Code: r.Code, Name: r.Name, Type: domain.AttributeType(r.Type),
		Unit: r.Unit, Options: opts, Filterable: r.Filterable, Variant: r.Variant,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}
