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

type CategoryRepository struct{ db *postgres.Manager }

func NewCategoryRepository(db *postgres.Manager) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) All(ctx context.Context) (domain.Categories, error) {
	// db.DB(ctx) trả transaction nếu đang ở trong transaction, ngược lại trả
	// pool. KHÔNG giữ *pgxpool.Pool trực tiếp trong struct — check-arch.sh chặn
	// điều đó (xem scripts/check-arch.sh, luật 3).
	rows, err := gen.New(r.db.DB(ctx)).AllCategories(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(domain.Categories, 0, len(rows))
	for _, row := range rows {
		out = append(out, categoryToDomain(row))
	}
	return out, nil
}

// LockForWrite khóa bảng ở chế độ SHARE ROW EXCLUSIVE tới hết transaction.
//
// Chế độ này tự xung đột với chính nó (hai lần ghi danh mục xếp hàng) nhưng
// KHÔNG xung đột với SELECT (storefront đọc bình thường) và với ROW SHARE (khóa
// khóa ngoại khi ghi products — tạo sản phẩm không bị chặn). Bảng xung đột đầy
// đủ ở đặc tả P1.1 mục 2.1.
//
// Không đi qua sqlc: LOCK không trả gì và sqlc không có gì để sinh.
//
// Gọi NGOÀI transaction là lỗi lập trình: khóa nhả ngay khi câu lệnh xong và
// không bảo vệ được gì. Postgres tự báo "LOCK TABLE can only be used in
// transaction blocks", nên lỗi đó lộ ra ngay ở lần chạy đầu.
func (r *CategoryRepository) LockForWrite(ctx context.Context) error {
	_, err := r.db.DB(ctx).Exec(ctx, "LOCK TABLE categories IN SHARE ROW EXCLUSIVE MODE")
	return err
}

func (r *CategoryRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	row, err := gen.New(r.db.DB(ctx)).CategoryByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnknownCategory
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return categoryToDomain(row), nil
}

func (r *CategoryRepository) Insert(ctx context.Context, c *domain.Category) error {
	err := gen.New(r.db.DB(ctx)).InsertCategory(ctx, gen.InsertCategoryParams{
		ID: c.ID, ParentID: c.ParentID, Slug: c.Slug, Name: c.Name,
		Position: int32(c.Position), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	})
	return mapCategoryWriteErr(err)
}

func (r *CategoryRepository) Update(ctx context.Context, c *domain.Category) error {
	n, err := gen.New(r.db.DB(ctx)).UpdateCategory(ctx, gen.UpdateCategoryParams{
		ID: c.ID, ParentID: c.ParentID, Slug: c.Slug, Name: c.Name,
		Position: int32(c.Position), UpdatedAt: c.UpdatedAt,
	})
	if err != nil {
		return mapCategoryWriteErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownCategory
	}
	return nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := gen.New(r.db.DB(ctx)).DeleteCategory(ctx, id)
	if err != nil {
		// Ở chiều XÓA, categories_parent_id_fkey nghĩa là "còn con" — ngược với
		// chiều ghi, nơi cùng ràng buộc đó nghĩa là "cha không tồn tại". Vì thế
		// map theo thao tác chứ không dùng chung mapErr.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			switch pgErr.ConstraintName {
			case "categories_parent_id_fkey":
				return domain.ErrCategoryHasChildren
			case "products_category_id_fkey":
				return domain.ErrCategoryHasProducts
			}
		}
		return mapErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownCategory
	}
	return nil
}

// mapCategoryWriteErr map lỗi của INSERT/UPDATE danh mục.
func mapCategoryWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) {
		return mapErr(err)
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "categories_slug_key":
		return domain.ErrDuplicateCategorySlug
	case pgErr.Code == "23503" && pgErr.ConstraintName == "categories_parent_id_fkey":
		// Chiều GHI: cha không tồn tại. Use case đã kiểm trên cây, nên tới được
		// đây nghĩa là có ai xóa cha bằng SQL tay giữa chừng.
		return domain.ErrCategoryNotFound
	case pgErr.Code == "23514" && pgErr.ConstraintName == "categories_no_self_parent":
		return domain.ErrCategoryCycle
	}
	return mapErr(err)
}

func categoryToDomain(row gen.Category) *domain.Category {
	return &domain.Category{
		ID:        row.ID,
		ParentID:  row.ParentID,
		Slug:      row.Slug,
		Name:      row.Name,
		Position:  int(row.Position),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}
}
