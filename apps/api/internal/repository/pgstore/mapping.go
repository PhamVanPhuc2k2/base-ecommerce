package pgstore

import (
	"encoding/json"
	"errors"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// toDomain chuyển dòng sqlc thành entity, kèm các variant của nó.
//
// Price đọc từ cột products.price (giá "từ" đã lưu) chứ không tính lại từ
// variants: đó chính là con số mà trang danh sách dùng để lọc và sắp xếp, nên
// trang chi tiết phải hiện đúng con số đó. Domain tính lại nó sau mọi lần ghi.
func toDomain(r gen.Product, rows []gen.ProductVariant) (*domain.Product, error) {
	variants := make([]*domain.Variant, 0, len(rows))
	for _, v := range rows {
		opts := map[string]string{}
		if len(v.Options) > 0 {
			if err := json.Unmarshal(v.Options, &opts); err != nil {
				return nil, err
			}
		}
		variants = append(variants, &domain.Variant{
			ID:        v.ID,
			SKU:       v.Sku,
			Price:     domain.MoneyFromDecimal(v.Price, v.Currency),
			Options:   opts,
			Status:    domain.VariantStatus(v.Status),
			Position:  int(v.Position),
			CreatedAt: v.CreatedAt.UTC(),
			UpdatedAt: v.UpdatedAt.UTC(),
		})
	}

	attrs := map[string]string{}
	if len(r.Attributes) > 0 {
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			return nil, err
		}
	}
	return &domain.Product{
		ID:               r.ID,
		Variants:         variants,
		Slug:             r.Slug,
		Name:             r.Name,
		ShortDescription: r.ShortDescription,
		CategoryID:       r.CategoryID,
		BrandID:          r.BrandID,
		Price:            domain.MoneyFromDecimal(r.Price, r.Currency),
		Status:           domain.Status(r.Status),
		Attributes:       attrs,
		Images:           r.Images,
		// pgx giải mã timestamptz về múi giờ của tiến trình. Không .UTC() thì
		// POST trả ...Z còn GET trả ...+07:00 cho CÙNG một sản phẩm, và lỗi này
		// vô hình trên CI vì runner chạy giờ UTC.
		CreatedAt: r.CreatedAt.UTC(),
		UpdatedAt: r.UpdatedAt.UTC(),
	}, nil
}

// mapErr đổi lỗi Postgres thành sentinel của domain.
//
// Đây là nơi DUY NHẤT biết mã lỗi Postgres. Nhờ vậy lỗi thô — kèm tên bảng,
// tên cột, tên ràng buộc — không bao giờ lọt ra response.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProductNotFound
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case "23505": // unique_violation
		switch pgErr.ConstraintName {
		case "products_sku_uq", "product_variants_sku_uq":
			return domain.ErrDuplicateSKU
		case "products_slug_uq":
			return domain.ErrDuplicateSlug
		}
	case "23503": // foreign_key_violation
		switch pgErr.ConstraintName {
		case "products_brand_id_fkey":
			return domain.ErrBrandNotFound
		case "products_category_id_fkey":
			return domain.ErrCategoryNotFound
		}
	case "23514": // check_violation
		// Những ràng buộc này lẽ ra đã bị domain chặn trước. Map lại ở đây vì
		// guard của domain nằm ở constructor (NewMoney), không nằm ở kiểu —
		// domain.Money{} rỗng vẫn dựng được từ bất kỳ package nào và lọt xuống
		// tới Postgres. Không map thì nó thành 500 thay vì 422.
		switch pgErr.ConstraintName {
		case "products_currency_vnd":
			return domain.ErrUnsupportedCurrency
		case "products_price_check":
			return domain.ErrInvalidPrice
		case "products_status_check":
			return domain.ErrInvalidStatus
		case "product_variants_currency_vnd":
			return domain.ErrUnsupportedCurrency
		case "product_variants_price_check":
			return domain.ErrInvalidPrice
		case "product_variants_status_check":
			return domain.ErrInvalidVariantStatus
		case "product_variants_options_object":
			return domain.ErrInvalidVariantOptions
		}
	}
	return err
}
