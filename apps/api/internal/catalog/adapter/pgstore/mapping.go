package pgstore

import (
	"encoding/json"
	"errors"

	"base-ecommerce/api/internal/catalog/adapter/pgstore/gen"
	"base-ecommerce/api/internal/catalog/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// toDomain chuyển dòng sqlc thành entity.
func toDomain(r gen.Product) (*domain.Product, error) {
	attrs := map[string]string{}
	if len(r.Attributes) > 0 {
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			return nil, err
		}
	}
	return &domain.Product{
		ID:               r.ID,
		SKU:              r.Sku,
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
		case "products_sku_uq":
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
		}
	}
	return err
}
