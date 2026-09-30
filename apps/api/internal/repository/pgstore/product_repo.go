package pgstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	sq "github.com/Masterminds/squirrel"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
)

type ProductRepository struct{ db *postgres.Manager }

func NewProductRepository(db *postgres.Manager) *ProductRepository {
	return &ProductRepository{db: db}
}

// Save ghi CẢ aggregate: dòng products rồi từng variant.
//
// Nhiều câu lệnh, nên PHẢI được gọi trong TxManager.Run — mọi use case ghi
// đều làm vậy. Ngoài transaction thì hỏng giữa chừng để lại sản phẩm với giá
// "từ" không khớp variant nào.
//
// Không xóa variant nào: domain không có thao tác xóa (đặc tả P1.2 mục 2.2),
// nên upsert từng cái là đủ.
func (r *ProductRepository) Save(ctx context.Context, p *domain.Product) error {
	attrs, err := json.Marshal(p.Attributes)
	if err != nil {
		return err
	}
	if len(p.Variants) == 0 {
		return domain.ErrVariantRequired
	}
	q := gen.New(r.db.DB(ctx))
	if err := mapErr(q.UpsertProduct(ctx, gen.UpsertProductParams{
		ID:               p.ID,
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
	})); err != nil {
		return err
	}
	for _, v := range p.Variants {
		opts, err := json.Marshal(v.Options)
		if err != nil {
			return err
		}
		if err := mapErr(q.UpsertVariant(ctx, gen.UpsertVariantParams{
			ID: v.ID, ProductID: p.ID, Sku: v.SKU,
			Price: v.Price.Decimal(), Currency: v.Price.Currency(),
			Options: opts, Status: string(v.Status), Position: int32(v.Position),
			CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		})); err != nil {
			return err
		}
	}
	return nil
}

// ByID đọc sản phẩm và KHÓA dòng đó (SELECT ... FOR UPDATE).
//
// Chỉ dùng trong use case ghi. Gọi ngoài transaction thì khóa được lấy rồi nhả
// ngay, vô hại — nhưng một endpoint đọc dùng hàm này sẽ xếp hàng sau mọi writer
// đang chạy. Cần đọc thuần thì thêm một hàm riêng, đừng dùng lại hàm này.
func (r *ProductRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	q := gen.New(r.db.DB(ctx))
	row, err := q.ProductByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	// Variant KHÔNG cần FOR UPDATE riêng: mọi lần ghi variant đều đi qua dòng
	// products vừa khóa ở trên, nên khóa của aggregate root là đủ.
	vs, err := q.VariantsByProduct(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	return toDomain(row.Product, vs)
}

func (r *ProductRepository) BySlug(ctx context.Context, slug string) (*domain.Product, error) {
	q := gen.New(r.db.DB(ctx))
	row, err := q.ProductBySlug(ctx, slug)
	if err != nil {
		return nil, mapErr(err)
	}
	vs, err := q.VariantsByProduct(ctx, row.Product.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return toDomain(row.Product, vs)
}

// filtered dựng phần FROM + WHERE dùng chung cho List và Count.
//
// Dùng squirrel chứ không dùng sqlc: bộ lọc có nhiều điều kiện tùy chọn, đúng
// giới hạn của sqlc. Squirrel tự tham số hóa nên không có nguy cơ injection —
// TUYỆT ĐỐI không nối chuỗi SQL bằng fmt.Sprintf.
//
// List và Count PHẢI đi qua cùng một hàm: hai bản WHERE viết tay sẽ lệch nhau
// vào một ngày nào đó, và total báo 40 trong khi lật hết trang chỉ thấy 38.
func filtered(f usecase.ListFilter) (sq.SelectBuilder, error) {
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
			return base, err
		}
		base = base.Where("attributes @> ?::jsonb", string(b))
	}

	// Tùy chọn biến thể gộp vào MỘT EXISTS: ram=16GB&mau=den nghĩa là có MỘT
	// phiên bản đang bán thỏa CẢ HAI — không phải một bản 16GB và một bản khác
	// màu đen. Tách thành hai EXISTS là trả về sản phẩm khách không mua được
	// đúng thứ mình lọc. Đặc tả P1.3 mục 2.8.
	//
	// json.Marshal sắp khóa map, nên cùng bộ lọc luôn ra cùng tham số.
	if len(f.VariantOptions) > 0 {
		b, err := json.Marshal(f.VariantOptions)
		if err != nil {
			return base, err
		}
		base = base.Where(`EXISTS (SELECT 1 FROM product_variants v
			WHERE v.product_id = products.id AND v.status = 'active' AND v.options @> ?::jsonb)`, string(b))
	}
	return base, nil
}

func (r *ProductRepository) List(ctx context.Context, f usecase.ListFilter) ([]*domain.Product, error) {
	// ListFilter không hứa Page >= 1. Không kẹp ở đây thì Page = 0 làm
	// (Page-1)*Limit tràn uint64 và Postgres trả bigint out of range.
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 1
	}
	base, err := filtered(f)
	if err != nil {
		return nil, err
	}

	q := base.Columns(
		"id", "slug", "name", "short_description", "category_id", "brand_id",
		"price", "currency", "status", "attributes", "images", "created_at", "updated_at",
	)

	// sort là enum đóng ở tầng usecase; default bọc lót thêm một lần nữa để không
	// có đường nào đưa tên cột tự do vào ORDER BY.
	switch f.Sort {
	case usecase.SortPriceAsc:
		q = q.OrderBy("price ASC, id ASC")
	case usecase.SortPriceDesc:
		q = q.OrderBy("price DESC, id ASC")
	default:
		q = q.OrderBy("created_at DESC, id DESC")
	}

	q = q.Limit(uint64(f.Limit)).Offset(uint64((f.Page - 1) * f.Limit))

	listSQL, listArgs, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.DB(ctx).Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	var prods []gen.Product
	for rows.Next() {
		var g gen.Product
		if err := rows.Scan(
			&g.ID, &g.Slug, &g.Name, &g.ShortDescription, &g.CategoryID,
			&g.BrandID, &g.Price, &g.Currency, &g.Status, &g.Attributes, &g.Images,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, mapErr(err)
		}
		prods = append(prods, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("duyệt kết quả: %w", mapErr(err))
	}
	// Đóng TRƯỚC câu tiếp theo: trong transaction, pgx không cho chạy câu mới
	// khi kết quả câu trước chưa đọc xong trên cùng kết nối.
	rows.Close()

	// Nạp variant cho CẢ trang bằng một câu (= ANY) rồi chia theo product_id.
	// Một câu mỗi sản phẩm là 24 lượt đi về database cho một trang danh sách.
	ids := make([]uuid.UUID, len(prods))
	for i, g := range prods {
		ids[i] = g.ID
	}
	byProduct := map[uuid.UUID][]gen.ProductVariant{}
	if len(ids) > 0 {
		vs, err := gen.New(r.db.DB(ctx)).VariantsByProducts(ctx, ids)
		if err != nil {
			return nil, mapErr(err)
		}
		for _, v := range vs {
			byProduct[v.ProductID] = append(byProduct[v.ProductID], v)
		}
	}

	out := make([]*domain.Product, 0, len(prods))
	for _, g := range prods {
		p, err := toDomain(g, byProduct[g.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Facets đếm số sản phẩm theo từng giá trị của các thuộc tính lọc được, trên
// ĐÚNG tập kết quả của List (cùng filtered()). productCodes đọc từ
// products.attributes, variantCodes từ options của variant active.
//
// Hai câu, mỗi câu một GROUP BY trên jsonb_each_text — không có câu nào mỗi
// thuộc tính. Tập kết quả lấy qua FromSelect: squirrel tự đổi placeholder của
// subquery về dạng ? để đánh số $n ở câu ngoài không lệch (squirrel #183).
func (r *ProductRepository) Facets(ctx context.Context, f usecase.ListFilter,
	productCodes, variantCodes []string) (domain.FacetCounts, error) {

	base, err := filtered(f)
	if err != nil {
		return nil, err
	}
	var queries []sq.SelectBuilder
	if len(productCodes) > 0 {
		queries = append(queries, sq.Select("e.key", "e.value", "count(*)").
			FromSelect(base.Columns("id", "attributes"), "p").
			JoinClause("CROSS JOIN LATERAL jsonb_each_text(p.attributes) AS e(key, value)").
			Where("e.key = ANY(?)", productCodes).
			GroupBy("e.key", "e.value"))
	}
	if len(variantCodes) > 0 {
		// count(DISTINCT p.id): hai phiên bản 16GB của cùng một sản phẩm vẫn
		// là MỘT sản phẩm — con số hiện cạnh nút lọc là số sản phẩm sẽ thấy.
		queries = append(queries, sq.Select("e.key", "e.value", "count(DISTINCT p.id)").
			FromSelect(base.Columns("id"), "p").
			Join("product_variants v ON v.product_id = p.id AND v.status = 'active'").
			JoinClause("CROSS JOIN LATERAL jsonb_each_text(v.options) AS e(key, value)").
			Where("e.key = ANY(?)", variantCodes).
			GroupBy("e.key", "e.value"))
	}

	out := domain.FacetCounts{}
	for _, q := range queries {
		sqlStr, args, err := q.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return nil, err
		}
		rows, err := r.db.DB(ctx).Query(ctx, sqlStr, args...)
		if err != nil {
			return nil, mapErr(err)
		}
		for rows.Next() {
			var fc domain.FacetCount
			if err := rows.Scan(&fc.Code, &fc.Value, &fc.Count); err != nil {
				rows.Close()
				return nil, mapErr(err)
			}
			out = append(out, fc)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, mapErr(err)
		}
	}
	// Thứ tự ổn định cho cache và cho giao diện: theo mã, rồi nhiều sản phẩm
	// trước, rồi theo giá trị.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out, nil
}

// Count đếm toàn bộ kết quả của bộ lọc — câu đắt nhất của trang danh sách.
// Use case gọi nó qua cache (usecase.KeyProductCount) và bỏ qua hẳn khi trang
// đầu đã chứa hết kết quả.
//
// KHÔNG dùng count(*) OVER () gộp vào List: window aggregate phải dựng toàn bộ
// kết quả trước LIMIT, phá mất Index Only Scan.
func (r *ProductRepository) Count(ctx context.Context, f usecase.ListFilter) (int, error) {
	base, err := filtered(f)
	if err != nil {
		return 0, err
	}
	countSQL, countArgs, err := base.Column("count(*)").ToSql()
	if err != nil {
		return 0, err
	}
	var total int
	if err := r.db.DB(ctx).QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return 0, mapErr(err)
	}
	return total, nil
}

// Sitemap đọc một trang cho sitemap phân mảnh — Index Only Scan trên
// products_live_id_idx (đo ở migration sitemap_index).
func (r *ProductRepository) Sitemap(ctx context.Context, offset, limit int) ([]usecase.SitemapEntry, error) {
	rows, err := gen.New(r.db.DB(ctx)).SitemapProducts(ctx, gen.SitemapProductsParams{
		PageSize: int32(limit), PageOffset: int32(offset),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]usecase.SitemapEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, usecase.SitemapEntry{Slug: row.Slug, UpdatedAt: row.UpdatedAt.UTC()})
	}
	return out, nil
}

func (r *ProductRepository) CountLive(ctx context.Context) (int, error) {
	n, err := gen.New(r.db.DB(ctx)).CountLiveProducts(ctx)
	if err != nil {
		return 0, mapErr(err)
	}
	return int(n), nil
}
