package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// ParentChange mô tả ý định đổi cha, với BA trạng thái.
//
//	Set=false            giữ nguyên chỗ cũ
//	Set=true, ID=nil     lên làm danh mục gốc
//	Set=true, ID=&x      chuyển vào dưới x
//
// Một *uuid.UUID trần chỉ có hai trạng thái, và "vắng mặt" lẫn "null" cùng
// thành nil — PATCH chỉ đổi tên sẽ âm thầm kéo danh mục lên gốc. Xem đặc tả
// P1.1 mục 2.2.
type ParentChange struct {
	Set bool
	ID  *uuid.UUID
}

type UpdateCategoryInput struct {
	ID       uuid.UUID
	Name     *string
	Slug     *string
	Parent   ParentChange
	Position *int
}

type UpdateCategory struct {
	tx    TxManager
	repo  CategoryRepository
	cache Cache
}

func NewUpdateCategory(tx TxManager, repo CategoryRepository, cache Cache) *UpdateCategory {
	return &UpdateCategory{tx: tx, repo: repo, cache: cache}
}

func (uc *UpdateCategory) Execute(ctx context.Context, in UpdateCategoryInput) (*domain.Category, error) {
	var updated *domain.Category

	// Mọi lần đọc nằm TRONG closure: TxManager chạy lại closure khi gặp lỗi tuần
	// tự hóa, và lần chạy lại phải thấy cây mới chứ không phải cây của lần trước.
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		// ⚠️ Khóa TRƯỚC, đọc cây SAU. Đảo thứ tự là mở lại đúng lỗ đặc tả mục
		// 2.1 mô tả: hai lệnh chuyển chéo nhau đều kiểm trên cây cũ, đều qua,
		// và cùng nhau tạo vòng lặp làm cả nhánh biến mất khỏi storefront.
		if err := uc.repo.LockForWrite(ctx); err != nil {
			return err
		}
		c, err := uc.repo.ByID(ctx, in.ID)
		if err != nil {
			return err
		}

		if in.Name != nil {
			if err := c.Rename(*in.Name); err != nil {
				return err
			}
		}
		if in.Slug != nil {
			if err := c.SetSlug(*in.Slug); err != nil {
				return err
			}
		}
		if in.Position != nil {
			c.SetPosition(*in.Position)
		}
		if in.Parent.Set {
			// Đọc cây từ DATABASE, không từ cache: cache có thể cũ tới 6 giờ,
			// và kiểm vòng lặp trên cây cũ là kiểm cho có.
			cats, err := uc.repo.All(ctx)
			if err != nil {
				return err
			}
			if err := domain.NewTree(cats).CheckMove(c.ID, in.Parent.ID); err != nil {
				return err
			}
			c.MoveTo(in.Parent.ID)
		}

		if err := uc.repo.Update(ctx, c); err != nil {
			return err
		}
		updated = c
		return nil
	}); err != nil {
		return nil, err
	}

	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyCategoryTree())
	return updated, nil
}
