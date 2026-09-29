package usecase

import (
	"context"
	"errors"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

type CreateCategoryInput struct {
	Name     string
	Slug     string
	ParentID *uuid.UUID
	Position int
}

type CreateCategory struct {
	tx    TxManager
	repo  CategoryRepository
	cache Cache
}

func NewCreateCategory(tx TxManager, repo CategoryRepository, cache Cache) *CreateCategory {
	return &CreateCategory{tx: tx, repo: repo, cache: cache}
}

func (uc *CreateCategory) Execute(ctx context.Context, in CreateCategoryInput) (*domain.Category, error) {
	c, err := domain.NewCategory(in.Name, in.Slug, in.ParentID, in.Position)
	if err != nil {
		return nil, err
	}

	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		// Tạo mới không thể sinh vòng lặp (chưa ai trỏ vào danh mục này), nhưng
		// vẫn khóa: không khóa thì một lệnh XÓA cha chạy song song có thể lọt
		// vào giữa bước kiểm và bước ghi. Khóa ngoại vẫn chặn được ca đó, nên
		// khóa ở đây là để mọi lần ghi danh mục đi cùng MỘT kỷ luật, không phải
		// để vá một lỗ cụ thể.
		if err := uc.repo.LockForWrite(ctx); err != nil {
			return err
		}
		if in.ParentID != nil {
			if _, err := uc.repo.ByID(ctx, *in.ParentID); err != nil {
				// Tài nguyên trên đường dẫn thì 404, nhưng cha nằm trong BODY:
				// dữ liệu gửi lên tham chiếu thứ không tồn tại → 422.
				if errors.Is(err, domain.ErrUnknownCategory) {
					return domain.ErrCategoryNotFound
				}
				return err
			}
		}
		return uc.repo.Insert(ctx, c)
	}); err != nil {
		return nil, err
	}

	// Sau commit, và WithoutCancel: xem CreateProduct. Không xóa thì danh mục
	// mới vô hình trên storefront suốt TTL 6 giờ của cây — đúng lỗ P0.2 ghi lại.
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyCategoryTree())
	return c, nil
}
