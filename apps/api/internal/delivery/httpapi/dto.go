package httpapi

import (
	"encoding/json"
	"time"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// productDTO là hình dạng JSON công khai. Nó tách khỏi domain entity để hợp
// đồng OpenAPI ổn định độc lập với thay đổi bên trong.
type productDTO struct {
	ID               uuid.UUID         `json:"id"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	ShortDescription string            `json:"short_description"`
	CategoryID       uuid.UUID         `json:"category_id"`
	BrandID          uuid.UUID         `json:"brand_id"`
	Price            string            `json:"price"` // giá "từ"; chuỗi: number của JS mất chính xác với số lớn
	Currency         string            `json:"currency"`
	Status           string            `json:"status"`
	Attributes       map[string]string `json:"attributes"`
	Images           []string          `json:"images"`
	Variants         []variantDTO      `json:"variants"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type variantDTO struct {
	ID       uuid.UUID         `json:"id"`
	SKU      string            `json:"sku"`
	Price    string            `json:"price"`
	Currency string            `json:"currency"`
	Options  map[string]string `json:"options"`
	Status   string            `json:"status"`
	Position int               `json:"position"`
}

// variantScope quyết định variant nào lọt vào response.
type variantScope int

const (
	// publicVariants: chỉ variant active — khách không được thấy phiên bản đã
	// ngừng bán, càng không được đặt mua nó.
	publicVariants variantScope = iota
	// adminVariants: đủ cả variant inactive, để quản trị thấy và bật lại được.
	adminVariants
)

func toProductDTO(p *domain.Product, scope variantScope) productDTO {
	vs := p.Variants
	if scope == publicVariants {
		vs = p.ActiveVariants()
	}
	variants := make([]variantDTO, 0, len(vs))
	for _, v := range vs {
		variants = append(variants, variantDTO{
			ID: v.ID, SKU: v.SKU, Price: v.Price.String(), Currency: v.Price.Currency(),
			Options: v.Options, Status: string(v.Status), Position: v.Position,
		})
	}
	return productDTO{
		ID: p.ID, Slug: p.Slug, Name: p.Name,
		Variants:         variants,
		ShortDescription: p.ShortDescription,
		CategoryID:       p.CategoryID, BrandID: p.BrandID,
		Price:      p.Price.String(),
		Currency:   p.Price.Currency(),
		Status:     string(p.Status),
		Attributes: p.Attributes,
		Images:     p.Images,
		CreatedAt:  p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

type categoryDTO struct {
	ID       uuid.UUID     `json:"id"`
	Slug     string        `json:"slug"`
	Name     string        `json:"name"`
	Position int           `json:"position"`
	Children []categoryDTO `json:"children"`
}

func toCategoryDTO(t *domain.Tree, c *domain.Category) categoryDTO {
	kids := t.Children(c.ID)
	out := categoryDTO{
		ID: c.ID, Slug: c.Slug, Name: c.Name, Position: c.Position,
		Children: make([]categoryDTO, 0, len(kids)),
	}
	for _, k := range kids {
		out.Children = append(out.Children, toCategoryDTO(t, k))
	}
	return out
}

type listMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	MaxPage    int  `json:"max_page"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type listResponse struct {
	Data []productDTO `json:"data"`
	Meta listMeta     `json:"meta"`
}

type createProductRequest struct {
	Name             string                 `json:"name"`
	ShortDescription string                 `json:"short_description"`
	CategoryID       uuid.UUID              `json:"category_id"`
	BrandID          uuid.UUID              `json:"brand_id"`
	Variants         []createVariantRequest `json:"variants"`
	Attributes       map[string]string      `json:"attributes"`
	Images           []string               `json:"images"`
}

type createVariantRequest struct {
	SKU      string            `json:"sku"`
	Price    string            `json:"price"`
	Currency string            `json:"currency"`
	Options  map[string]string `json:"options"`
	Position int               `json:"position"`
}

func (r createVariantRequest) toInput() (domain.VariantInput, error) {
	price, err := domain.NewMoney(r.Price, r.Currency)
	if err != nil {
		return domain.VariantInput{}, err
	}
	return domain.VariantInput{SKU: r.SKU, Price: price, Options: r.Options, Position: r.Position}, nil
}

// updateVariantRequest: PATCH từng phần, con trỏ cho mọi trường vô hướng — cùng
// lý do với updateProductRequest. Không có sku: SKU bất biến.
type updateVariantRequest struct {
	Price    *string           `json:"price"`
	Currency *string           `json:"currency"`
	Options  map[string]string `json:"options"`
	Status   *string           `json:"status"`
	Position *int              `json:"position"`
}

// updateProductRequest dùng con trỏ cho mọi trường vô hướng vì đây là PATCH:
// trường vắng mặt phải GIỮ NGUYÊN, không phải đặt về zero value. Dùng kiểu giá
// trị thì không phân biệt được "client bỏ qua mô tả" với "client muốn xóa mô
// tả", và mặc định im lặng rơi vào vế thứ hai.
//
// Attributes và Images để nguyên kiểu map/slice vì chúng đã có nil sẵn.
//
// Không có price/currency: giá thuộc variant từ P1.2. Gửi `price` ở đây trả
// 400 MALFORMED_REQUEST (DisallowUnknownFields) — cố ý báo to, vì client cũ
// tưởng mình đang đổi giá mà thật ra không có gì đổi cả.
type updateProductRequest struct {
	Name             *string           `json:"name"`
	ShortDescription *string           `json:"short_description"`
	Attributes       map[string]string `json:"attributes"`
	Images           []string          `json:"images"`
	// Con trỏ: vắng mặt = giữ nguyên. Không có "null = xóa" — sản phẩm luôn
	// phải thuộc một danh mục và một thương hiệu (cột NOT NULL).
	CategoryID *uuid.UUID `json:"category_id"`
	BrandID    *uuid.UUID `json:"brand_id"`
}

type brandDTO struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

func toBrandDTO(b *domain.Brand) brandDTO {
	return brandDTO{ID: b.ID, Slug: b.Slug, Name: b.Name}
}

// categoryNodeDTO là MỘT danh mục đứng riêng, không kèm cây con — trả về từ API
// quản trị. Khác categoryDTO (cây) của GET /categories: trả cả cây sau mỗi lần
// sửa một nút là tốn và che mất nút vừa sửa nằm ở đâu.
type categoryNodeDTO struct {
	ID       uuid.UUID  `json:"id"`
	ParentID *uuid.UUID `json:"parent_id"` // null = gốc; không omitempty — hợp đồng hứa luôn có khóa
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Position int        `json:"position"`
}

func toCategoryNodeDTO(c *domain.Category) categoryNodeDTO {
	return categoryNodeDTO{ID: c.ID, ParentID: c.ParentID, Slug: c.Slug, Name: c.Name, Position: c.Position}
}

type createCategoryRequest struct {
	Name     string     `json:"name"`
	Slug     string     `json:"slug"`
	ParentID *uuid.UUID `json:"parent_id"` // tạo mới: vắng mặt và null cùng nghĩa "gốc"
	Position int        `json:"position"`
}

type updateCategoryRequest struct {
	Name     *string      `json:"name"`
	Slug     *string      `json:"slug"`
	ParentID optionalUUID `json:"parent_id"`
	Position *int         `json:"position"`
}

type createBrandRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type updateBrandRequest struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

// optionalUUID phân biệt BA trạng thái của một trường JSON có thể null:
//
//	khóa vắng mặt       → Set=false
//	"parent_id": null   → Set=true, Value=nil
//	"parent_id": "…"    → Set=true, Value=&id
//
// Mấu chốt: encoding/json CHỈ gọi UnmarshalJSON khi khóa có mặt trong JSON.
// Nên chỉ cần đặt Set=true bên trong nó — vắng mặt thì hàm không bao giờ chạy
// và Set giữ zero value false.
//
// Vì sao không dùng *uuid.UUID: nó chỉ có hai trạng thái, "vắng mặt" và "null"
// cùng thành nil, và PATCH chỉ đổi tên sẽ âm thầm kéo danh mục lên làm gốc.
type optionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (o *optionalUUID) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(b, &id); err != nil {
		return err
	}
	o.Value = &id
	return nil
}
