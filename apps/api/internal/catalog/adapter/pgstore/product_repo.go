package pgstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	sq "github.com/Masterminds/squirrel"

	"base-ecommerce/api/internal/catalog/adapter/pgstore/gen"
	"base-ecommerce/api/internal/catalog/app"
	"base-ecommerce/api/internal/catalog/domain"
	"base-ecommerce/api/internal/platform/postgres"

	"github.com/google/uuid"
)

type ProductRepository struct{ db *postgres.Manager }

func NewProductRepository(db *postgres.Manager) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Save(ctx context.Context, p *domain.Product) error {
	attrs, err := json.Marshal(p.Attributes)
	if err != nil {
		return err
	}
	return mapErr(gen.New(r.db.DB(ctx)).UpsertProduct(ctx, gen.UpsertProductParams{
		ID:               p.ID,
		Sku:              p.SKU,
		Slug:             p.Slug,
		Name:             p.Name,
		ShortDescription: p.ShortDescription,
		CategoryID:       p.CategoryID,
		BrandID:          p.BrandID,
		Price:            p.Price.Decimal(),
		Currency:         p.Price.Currency(),
		Status:           string(p.Status),
		Attributes:       attrs,
		Images:           p.Images,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}))
}

// ByID đọc sản phẩm và KHÓA dòng đó (SELECT ... FOR UPDATE).
//
// Chỉ dùng trong use case ghi. Gọi ngoài transaction thì khóa được lấy rồi nhả
// ngay, vô hại — nhưng một endpoint đọc dùng hàm này sẽ xếp hàng sau mọi writer
// đang chạy. Cần đọc thuần thì thêm một hàm riêng, đừng dùng lại hàm này.
func (r *ProductRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	row, err := gen.New(r.db.DB(ctx)).ProductByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	return toDomain(row.Product)
}

func (r *ProductRepository) BySlug(ctx context.Context, slug string) (*domain.Product, error) {
	row, err := gen.New(r.db.DB(ctx)).ProductBySlug(ctx, slug)
	if err != nil {
		return nil, mapErr(err)
	}
	return toDomain(row.Product)
}

// List dùng squirrel chứ không dùng sqlc: bộ lọc có nhiều điều kiện tùy chọn,
// đúng giới hạn của sqlc. Squirrel tự tham số hóa nên không có nguy cơ injection
// — TUYỆT ĐỐI không nối chuỗi SQL bằng fmt.Sprintf.
func (r *ProductRepository) List(ctx context.Context, f app.ListFilter) ([]*domain.Product, int, error) {
	// ListFilter không hứa Page >= 1. Không kẹp ở đây thì Page = 0 làm
	// (Page-1)*Limit tràn uint64 và Postgres trả bigint out of range.
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 1
	}

	// .Select() rỗng trước rồi thêm cột sau: StatementBuilderType không có
	// phương thức From, chỉ SelectBuilder mới có.
	base := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Select().
		From("products").
		Where("deleted_at IS NULL").
		Where(sq.Eq{"status": string(domain.StatusLive)})

	if len(f.CategoryIDs) > 0 {
		// = ANY(?) chứ không phải sq.Eq: sq.Eq với slice sinh ra IN ($2,...,$N),
		// nên câu SQL đổi theo số danh mục con và mỗi kích thước cây con thành
		// một plan riêng trong cache. ANY dùng một placeholder duy nhất.
		base = base.Where("category_id = ANY(?)", f.CategoryIDs)
	}
	if f.BrandSlug != "" {
		base = base.Where("brand_id IN (SELECT id FROM brands WHERE slug = ?)", f.BrandSlug)
	}
	if f.PriceMin != nil {
		base = base.Where(sq.GtOrEq{"price": *f.PriceMin})
	}
	if f.PriceMax != nil {
		base = base.Where(sq.LtOrEq{"price": *f.PriceMax})
	}

	// Sắp xếp khóa trước khi duyệt: map trong Go duyệt theo thứ tự ngẫu nhiên,
	// nên cùng một bộ lọc sẽ sinh ra câu SQL khác nhau mỗi lần — làm hỏng
	// prepared statement cache và khiến EXPLAIN không lặp lại được.
	keys := make([]string, 0, len(f.Attributes))
	for k := range f.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b, err := json.Marshal(map[string]string{k: f.Attributes[k]})
		if err != nil {
			return nil, 0, err
		}
		base = base.Where("attributes @> ?::jsonb", string(b))
	}

	q := base.Columns(
		"id", "sku", "slug", "name", "short_description", "category_id", "brand_id",
		"price", "currency", "status", "attributes", "images", "created_at", "updated_at",
	)

	// sort là enum đóng ở tầng app; default bọc lót thêm một lần nữa để không
	// có đường nào đưa tên cột tự do vào ORDER BY.
	switch f.Sort {
	case app.SortPriceAsc:
		q = q.OrderBy("price ASC, id ASC")
	case app.SortPriceDesc:
		q = q.OrderBy("price DESC, id ASC")
	default:
		q = q.OrderBy("created_at DESC, id DESC")
	}

	q = q.Limit(uint64(f.Limit)).Offset(uint64((f.Page - 1) * f.Limit))

	listSQL, listArgs, err := q.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.DB(ctx).Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()

	out := make([]*domain.Product, 0, f.Limit)
	for rows.Next() {
		var g gen.Product
		if err := rows.Scan(
			&g.ID, &g.Sku, &g.Slug, &g.Name, &g.ShortDescription, &g.CategoryID,
			&g.BrandID, &g.Price, &g.Currency, &g.Status, &g.Attributes, &g.Images,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, 0, mapErr(err)
		}
		p, err := toDomain(g)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("duyệt kết quả: %w", mapErr(err))
	}

	// Đếm sau, và bỏ hẳn bước đếm khi trang đầu đã chứa hết kết quả: count(*)
	// không lọc là 16,8 ms / 5057 buffer ở 200k dòng, trong khi câu lấy trang
	// chỉ 0,049 ms / 4 buffer — tức là 99,7% chi phí nằm ở phép đếm.
	//
	// KHÔNG dùng count(*) OVER (): window aggregate phải dựng toàn bộ kết quả
	// trước LIMIT, phá mất Index Only Scan.
	total := len(out)
	if f.Page > 1 || len(out) == f.Limit {
		// còn trang nữa, hoặc đang ở trang sau — phải đếm thật
		countSQL, countArgs, err := base.Column("count(*)").ToSql()
		if err != nil {
			return nil, 0, err
		}
		if err := r.db.DB(ctx).QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, 0, mapErr(err)
		}
	}

	return out, total, nil
}
