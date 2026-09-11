package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusLive     Status = "live"
	StatusArchived Status = "archived"
)

const maxSKULen = 64
const maxNameLen = 200

// Valid cho biết giá trị có nằm trong tập trạng thái đã định nghĩa không.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusLive, StatusArchived:
		return true
	}
	return false
}

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
	if sku == "" || utf8.RuneCountInString(sku) > maxSKULen {
		return nil, ErrInvalidSKU
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return nil, ErrNameTooLong
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
//
// Quy ước: nil nghĩa là GIỮ NGUYÊN, cho mọi tham số.
//
// Bản trước nhận giá trị thay vì con trỏ, và điều đó gây mất dữ liệu im lặng:
// gọi PATCH chỉ để đổi giá thì `short_description` nhận chuỗi rỗng — zero value
// của string — và mô tả sản phẩm biến mất khỏi trang bán hàng, không một lỗi
// nào. Tệ hơn là nó KHÔNG nhất quán: `attributes` và `images` đã có guard nil
// nên được giữ, chỉ mình `short_description` bị xóa. Không ai đoán được điều đó
// từ hợp đồng API.
func (p *Product) Update(name, shortDesc *string, price *Money,
	attributes map[string]string, images []string) error {

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return ErrNameRequired
		}
		if utf8.RuneCountInString(trimmed) > maxNameLen {
			return ErrNameTooLong
		}
		slug, err := NewSlug(trimmed)
		if err != nil {
			return err
		}
		p.Name = trimmed
		p.Slug = slug
	}
	if shortDesc != nil {
		p.ShortDescription = *shortDesc
	}
	if price != nil {
		p.Price = *price
	}
	if attributes != nil {
		p.Attributes = attributes
	}
	if images != nil {
		p.Images = images
	}

	// Sản phẩm đang bán phải luôn thỏa điều kiện của Publish. Không kiểm ở đây
	// thì một lệnh PATCH có thể tước ảnh và giá của sản phẩm đang live mà không
	// báo lỗi gì — Publish canh lúc đăng bán, nhưng không ai canh lúc sửa.
	//
	// Kiểm trên trạng thái CUỐI CÙNG chứ không trên tham số truyền vào: sau khi
	// chuyển sang ngữ nghĩa nil-giữ-nguyên, tham số không còn mô tả đủ sản phẩm
	// sẽ ra sao sau lệnh sửa.
	if p.Status == StatusLive {
		if len(p.Images) == 0 {
			return ErrNoImage
		}
		if p.Price.IsZero() {
			return ErrPriceRequired
		}
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

// Validate kiểm những bất biến mà một Product hợp lệ luôn phải thỏa.
//
// Dùng khi dựng lại Product từ một nguồn KHÔNG đi qua hàm dựng — hiện là
// Redis cache. json.Unmarshal nhận `{}` mà không báo lỗi gì và cho ra struct
// toàn giá trị zero: ID toàn số 0, tên rỗng, giá 0, status rỗng. Không có phép
// kiểm này thì API trả 200 kèm một sản phẩm bịa, giá 0, suốt cả TTL.
//
// Đây KHÔNG phải nơi kiểm quy tắc nghiệp vụ lúc ghi — chỗ đó là NewProduct,
// Update và Publish. Ở đây chỉ hỏi một câu: giá trị này có thể do code của
// chúng ta tạo ra không?
func (p *Product) Validate() error {
	if p.ID == uuid.Nil {
		return ErrProductNotFound
	}
	if strings.TrimSpace(p.SKU) == "" {
		return ErrInvalidSKU
	}
	if strings.TrimSpace(p.Slug) == "" {
		return ErrInvalidSlug
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrNameRequired
	}
	if !p.Status.Valid() {
		return ErrInvalidStatus
	}
	if p.Price.Currency() != SupportedCurrency {
		return ErrUnsupportedCurrency
	}
	return nil
}
