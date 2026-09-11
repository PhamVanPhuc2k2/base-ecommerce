package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusLive     Status = "live"
	StatusArchived Status = "archived"
)

const maxSKULen = 64

// Product là aggregate root của catalog.
type Product struct {
	ID               uuid.UUID
	SKU              string
	Slug             string
	Name             string
	ShortDescription string
	CategoryID       uuid.UUID
	BrandID          uuid.UUID
	Price            Money
	Status           Status
	Attributes       map[string]string
	Images           []string
	CreatedAt        time.Time
	UpdatedAt        time.Time

	events []Event
}

// NewProduct dựng sản phẩm mới ở trạng thái draft.
//
// ID sinh NGAY tại đây chứ không để Postgres DEFAULT: entity phải có ID trước
// khi insert, vì domain event tham chiếu tới ID đó và (từ P0.3) được ghi vào
// outbox trong cùng transaction.
func NewProduct(sku, name, shortDesc string, categoryID, brandID uuid.UUID,
	price Money, attributes map[string]string, images []string) (*Product, error) {

	sku = strings.TrimSpace(sku)
	if sku == "" || len(sku) > maxSKULen {
		return nil, ErrInvalidSKU
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}
	slug, err := NewSlug(name)
	if err != nil {
		return nil, err
	}
	if categoryID == uuid.Nil {
		return nil, ErrCategoryNotFound
	}
	if brandID == uuid.Nil {
		return nil, ErrBrandNotFound
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	if attributes == nil {
		attributes = map[string]string{}
	}
	if images == nil {
		images = []string{}
	}

	p := &Product{
		ID: id, SKU: sku, Slug: slug, Name: name, ShortDescription: shortDesc,
		CategoryID: categoryID, BrandID: brandID, Price: price,
		Status: StatusDraft, Attributes: attributes, Images: images,
		CreatedAt: now, UpdatedAt: now,
	}
	p.raise(ProductCreated{newBase(id)})
	return p, nil
}

// Update sửa các trường cho phép. Không đổi được SKU: SKU là định danh dùng
// trong kho và hóa đơn, đổi nó là tạo sản phẩm khác.
func (p *Product) Update(name, shortDesc string, price Money,
	attributes map[string]string, images []string) error {

	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}
	slug, err := NewSlug(name)
	if err != nil {
		return err
	}

	p.Name = name
	p.Slug = slug
	p.ShortDescription = shortDesc
	p.Price = price
	if attributes != nil {
		p.Attributes = attributes
	}
	if images != nil {
		p.Images = images
	}
	p.UpdatedAt = time.Now().UTC()

	p.raise(ProductUpdated{newBase(p.ID)})
	return nil
}

// Publish đưa sản phẩm lên bán. Đây là chỗ quy tắc nghiệp vụ thật sự nằm.
func (p *Product) Publish() error {
	if p.Status == StatusLive {
		return ErrAlreadyPublished
	}
	if len(p.Images) == 0 {
		return ErrNoImage
	}
	if p.Price.IsZero() {
		return ErrPriceRequired
	}

	p.Status = StatusLive
	p.UpdatedAt = time.Now().UTC()
	p.raise(ProductPublished{newBase(p.ID)})
	return nil
}

func (p *Product) raise(e Event) { p.events = append(p.events, e) }

// PullEvents trả các sự kiện đã tích và xóa khỏi entity, để không phát hai lần.
func (p *Product) PullEvents() []Event {
	out := p.events
	p.events = nil
	return out
}
