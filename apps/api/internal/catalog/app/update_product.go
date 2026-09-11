package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
)

// UpdateProductInput mô tả một lệnh sửa từng phần: mọi trường nil nghĩa là
// giữ nguyên giá trị đang có.
type UpdateProductInput struct {
	ID               uuid.UUID
	Name             *string
	ShortDescription *string
	Price            *domain.Money
	Attributes       map[string]string
	Images           []string
}

type UpdateProduct struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewUpdateProduct(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *UpdateProduct {
	return &UpdateProduct{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *UpdateProduct) Execute(ctx context.Context, in UpdateProductInput) (*domain.Product, error) {
	var (
		updated *domain.Product
		oldSlug string
	)

	// repo.ByID PHẢI nằm trong closure. TxManager chạy lại closure khi gặp lỗi
	// tuần tự hóa, và cả sự kiện lẫn oldSlug đều phụ thuộc vào việc đọc lại
	// aggregate ở mỗi lần thử. Nhấc nó ra ngoài thì lần thử thứ hai phát ra
	// không sự kiện nào và xóa nhầm khóa cache.
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		p, err := uc.repo.ByID(ctx, in.ID)
		if err != nil {
			return err
		}
		oldSlug = p.Slug

		if err := p.Update(in.Name, in.ShortDescription, in.Price, in.Attributes, in.Images); err != nil {
			return err
		}
		if err := uc.repo.Save(ctx, p); err != nil {
			return err
		}
		updated = p
		return uc.events.Publish(ctx, p.PullEvents()...)
	}); err != nil {
		return nil, err
	}

	// Xóa cả slug cũ lẫn mới: đổi tên sản phẩm là đổi slug, để sót slug cũ thì
	// đường dẫn cũ vẫn trả dữ liệu cũ cho tới khi hết TTL.
	//
	// WithoutCancel: transaction đã commit rồi, việc xóa cache KHÔNG được hủy
	// theo. Client ngắt kết nối giữa chừng mà cache không xóa thì dữ liệu cũ
	// nằm lại tới hết TTL.
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyProductSlug(oldSlug), KeyProductSlug(updated.Slug))

	return updated, nil
}
