package httpapi

import (
	"time"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
)

// productDTO là hình dạng JSON công khai. Nó tách khỏi domain entity để hợp
// đồng OpenAPI ổn định độc lập với thay đổi bên trong.
type productDTO struct {
	ID               uuid.UUID         `json:"id"`
	SKU              string            `json:"sku"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	ShortDescription string            `json:"short_description"`
	CategoryID       uuid.UUID         `json:"category_id"`
	BrandID          uuid.UUID         `json:"brand_id"`
	Price            string            `json:"price"` // chuỗi: number của JS mất chính xác với số lớn
	Currency         string            `json:"currency"`
	Status           string            `json:"status"`
	Attributes       map[string]string `json:"attributes"`
	Images           []string          `json:"images"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func toProductDTO(p *domain.Product) productDTO {
	return productDTO{
		ID: p.ID, SKU: p.SKU, Slug: p.Slug, Name: p.Name,
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
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type listResponse struct {
	Data []productDTO `json:"data"`
	Meta listMeta     `json:"meta"`
}

type createProductRequest struct {
	SKU              string            `json:"sku"`
	Name             string            `json:"name"`
	ShortDescription string            `json:"short_description"`
	CategoryID       uuid.UUID         `json:"category_id"`
	BrandID          uuid.UUID         `json:"brand_id"`
	Price            string            `json:"price"`
	Currency         string            `json:"currency"`
	Attributes       map[string]string `json:"attributes"`
	Images           []string          `json:"images"`
}

type updateProductRequest struct {
	Name             string            `json:"name"`
	ShortDescription string            `json:"short_description"`
	Price            string            `json:"price"`
	Currency         string            `json:"currency"`
	Attributes       map[string]string `json:"attributes"`
	Images           []string          `json:"images"`
}
