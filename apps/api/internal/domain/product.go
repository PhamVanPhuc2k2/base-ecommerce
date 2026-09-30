package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
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

// Product là aggregate root của catalog. Variant chỉ sửa được QUA Product.
type Product struct {
	ID               uuid.UUID
	Slug             string
	Name             string
	ShortDescription string
	CategoryID       uuid.UUID
	BrandID          uuid.UUID
	// Price là giá THẤP NHẤT của các variant active — "giá từ". Không bao giờ
	// gán trực tiếp: recomputePrice tính lại sau mọi thay đổi variant.
	//
	// Vì sao lưu mà không tính khi đọc: trang danh sách lọc và sắp theo giá trên
	// hàng trăm nghìn dòng nhờ index trên products(price). Tính min() qua JOIN
	// mỗi lượt xem là bỏ hết index đó. Đặc tả P1.2 mục 2.1.
	Price      Money
	Status     Status
	Attributes map[string]string
	Images     []string
	Variants   []*Variant
	CreatedAt  time.Time
	UpdatedAt  time.Time

	events []Event
}

// NewProduct dựng sản phẩm mới ở trạng thái draft, kèm ít nhất một variant.
//
// ID sinh NGAY tại đây chứ không để Postgres DEFAULT: entity phải có ID trước
// khi insert, vì domain event tham chiếu tới ID đó và được ghi vào outbox
// trong cùng transaction.
func NewProduct(name, shortDesc string, categoryID, brandID uuid.UUID,
	variants []VariantInput, attributes map[string]string, images []string) (*Product, error) {

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
	// Không có variant thì không có gì để bán, không có SKU, không có giá —
	// một sản phẩm như vậy chỉ là trang thông tin rỗng.
	if len(variants) == 0 {
		return nil, ErrVariantRequired
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
		ID: id, Slug: slug, Name: name, ShortDescription: shortDesc,
		CategoryID: categoryID, BrandID: brandID,
		Status: StatusDraft, Attributes: attributes, Images: images,
		CreatedAt: now, UpdatedAt: now,
	}
	for _, in := range variants {
		v, err := newVariant(in, now)
		if err != nil {
			return nil, err
		}
		if err := p.checkOptionsUnique(v.Options, uuid.Nil); err != nil {
			return nil, err
		}
		p.Variants = append(p.Variants, v)
	}
	p.recomputePrice()
	p.raise(ProductCreated{baseEvent: newBase(p.ID), slug: p.Slug})
	return p, nil
}

// Update sửa các trường của phần chung. Giá KHÔNG còn nằm ở đây — giá thuộc
// variant (UpdateVariant), còn Product.Price chỉ là giá "từ" tính ra.
//
// Quy ước: nil nghĩa là GIỮ NGUYÊN, cho mọi tham số.
//
// Bản P0.2 nhận giá trị thay vì con trỏ, và điều đó gây mất dữ liệu im lặng:
// PATCH chỉ đổi giá làm `short_description` nhận zero value và mô tả biến mất
// khỏi trang bán hàng, không một lỗi nào.
func (p *Product) Update(name, shortDesc *string,
	attributes map[string]string, images []string,
	categoryID, brandID *uuid.UUID) error {

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
	if attributes != nil {
		p.Attributes = attributes
	}
	if images != nil {
		p.Images = images
	}
	// Danh mục/thương hiệu có tồn tại hay không do khóa ngoại quyết định lúc
	// Save (map sang CATEGORY_NOT_FOUND / BRAND_NOT_FOUND). Ở đây chỉ chặn
	// uuid rỗng — giá trị mà khóa ngoại sẽ báo cùng lỗi nhưng khó đọc hơn.
	if categoryID != nil {
		if *categoryID == uuid.Nil {
			return ErrCategoryNotFound
		}
		p.CategoryID = *categoryID
	}
	if brandID != nil {
		if *brandID == uuid.Nil {
			return ErrBrandNotFound
		}
		p.BrandID = *brandID
	}

	if err := p.checkSellable(); err != nil {
		return err
	}
	p.touch()
	return nil
}

// AddVariant thêm một phiên bản mới. Trả về variant vừa tạo để use case biết ID.
func (p *Product) AddVariant(in VariantInput) (*Variant, error) {
	v, err := newVariant(in, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := p.checkOptionsUnique(v.Options, uuid.Nil); err != nil {
		return nil, err
	}
	p.Variants = append(p.Variants, v)
	p.recomputePrice()
	if err := p.checkSellable(); err != nil {
		return nil, err
	}
	p.touch()
	return v, nil
}

// UpdateVariant sửa từng phần một variant — nil là giữ nguyên. SKU không đổi
// được: SKU là định danh trong kho và hóa đơn, đổi nó là tạo phiên bản khác.
func (p *Product) UpdateVariant(id uuid.UUID, price *Money, options map[string]string,
	status *VariantStatus, position *int) error {

	v := p.variant(id)
	if v == nil {
		return ErrUnknownVariant
	}
	if price != nil {
		v.Price = *price
	}
	if options != nil {
		opts, err := normalizeOptions(options)
		if err != nil {
			return err
		}
		if err := p.checkOptionsUnique(opts, v.ID); err != nil {
			return err
		}
		v.Options = opts
	}
	if status != nil {
		if !status.Valid() {
			return ErrInvalidVariantStatus
		}
		v.Status = *status
	}
	if position != nil {
		v.Position = *position
	}
	v.UpdatedAt = time.Now().UTC()

	p.recomputePrice()
	// Kiểm trên trạng thái SAU khi sửa: tắt variant active cuối cùng của sản
	// phẩm đang bán mà không kiểm thì sản phẩm vẫn "live" nhưng không có gì để
	// mua — trang hiện ra, nút mua dẫn vào hư không, không ai được báo.
	if err := p.checkSellable(); err != nil {
		return err
	}
	p.touch()
	return nil
}

// Publish đưa sản phẩm lên bán. Đây là chỗ quy tắc nghiệp vụ thật sự nằm.
func (p *Product) Publish() error {
	if p.Status == StatusLive {
		return ErrAlreadyPublished
	}
	if err := p.sellableRules(); err != nil {
		return err
	}
	p.Status = StatusLive
	p.UpdatedAt = time.Now().UTC()
	p.raise(ProductPublished{baseEvent: newBase(p.ID), slug: p.Slug})
	return nil
}

// ActiveVariants là những variant khách được thấy và mua.
func (p *Product) ActiveVariants() []*Variant {
	out := make([]*Variant, 0, len(p.Variants))
	for _, v := range p.Variants {
		if v.Status == VariantActive {
			out = append(out, v)
		}
	}
	return out
}

// checkSellable áp quy tắc bán hàng CHỈ khi sản phẩm đang live. Sản phẩm nháp
// được phép dở dang: chưa ảnh, chưa giá, mọi variant đều tắt.
func (p *Product) checkSellable() error {
	if p.Status != StatusLive {
		return nil
	}
	return p.sellableRules()
}

// sellableRules là điều kiện để một sản phẩm được bán. Publish kiểm TRƯỚC khi
// lên live; Update/AddVariant/UpdateVariant kiểm SAU khi sửa sản phẩm đang live
// — cùng một bộ quy tắc, để không có đường nào đi vòng qua nó.
func (p *Product) sellableRules() error {
	if len(p.Images) == 0 {
		return ErrNoImage
	}
	active := p.ActiveVariants()
	if len(active) == 0 {
		return ErrNoActiveVariant
	}
	// MỌI variant active phải có giá, không chỉ variant rẻ nhất: một phiên bản
	// giá 0 đang bán là khách đặt được hàng miễn phí.
	for _, v := range active {
		if v.Price.IsZero() {
			return ErrPriceRequired
		}
	}
	return nil
}

// recomputePrice đặt Price = giá thấp nhất của variant active. Không còn
// variant active nào (chỉ có thể ở sản phẩm nháp) thì giá là 0.
func (p *Product) recomputePrice() {
	var lowest *Money
	for _, v := range p.Variants {
		if v.Status != VariantActive {
			continue
		}
		if lowest == nil || v.Price.Decimal().LessThan(lowest.Decimal()) {
			price := v.Price
			lowest = &price
		}
	}
	if lowest == nil {
		p.Price = MoneyFromDecimal(decimal.Zero, SupportedCurrency)
		return
	}
	p.Price = *lowest
}

// checkOptionsUnique chặn hai variant cùng options — khách không phân biệt được
// chúng, và "chọn RAM 16GB" dẫn tới hai SKU khác nhau. except là ID của chính
// variant đang sửa, để so với mọi variant KHÁC.
func (p *Product) checkOptionsUnique(opts map[string]string, except uuid.UUID) error {
	key := optionsKey(opts)
	for _, v := range p.Variants {
		if v.ID != except && optionsKey(v.Options) == key {
			return ErrDuplicateVariantOptions
		}
	}
	return nil
}

func (p *Product) variant(id uuid.UUID) *Variant {
	for _, v := range p.Variants {
		if v.ID == id {
			return v
		}
	}
	return nil
}

// touch cập nhật thời điểm sửa và phát product.updated. Phát SAU khi p.Slug đã
// nhận giá trị mới: đổi tên là đổi slug, mà slug là thứ consumer dùng để dựng
// URL; phát sớm hơn thì payload mang slug cũ.
func (p *Product) touch() {
	p.UpdatedAt = time.Now().UTC()
	p.raise(ProductUpdated{baseEvent: newBase(p.ID), slug: p.Slug})
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
// toàn giá trị zero. Không có phép kiểm này thì API trả 200 kèm một sản phẩm
// bịa, giá 0, suốt cả TTL.
//
// Đây KHÔNG phải nơi kiểm quy tắc nghiệp vụ lúc ghi. Ở đây chỉ hỏi một câu:
// giá trị này có thể do code của chúng ta tạo ra không?
func (p *Product) Validate() error {
	if p.ID == uuid.Nil {
		return ErrProductNotFound
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
	// Mọi sản phẩm code của ta tạo ra đều có ít nhất một variant. Cache chứa
	// `"Variants": null` hay `[null]` thì từ chối — phần tử nil làm handler
	// panic khi dựng DTO.
	if len(p.Variants) == 0 {
		return ErrVariantRequired
	}
	for _, v := range p.Variants {
		if err := v.validate(); err != nil {
			return err
		}
	}
	return nil
}
