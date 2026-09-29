package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// maxTaxonomyNameLen là trần độ dài tên cho danh mục và thương hiệu. Ngắn hơn
// tên sản phẩm vì chúng hiện trên menu và breadcrumb.
const maxTaxonomyNameLen = 100

type Brand struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewBrand dựng thương hiệu mới. slug rỗng thì sinh từ tên; có thì vẫn chuẩn
// hóa qua NewSlug để client gửi "Apple Inc" cũng thành "apple-inc".
func NewBrand(name, slug string) (*Brand, error) {
	name, err := normalizeTaxonomyName(name, ErrBrandNameInvalid)
	if err != nil {
		return nil, err
	}
	if slug == "" {
		slug = name
	}
	s, err := NewSlug(slug)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Brand{ID: id, Slug: s, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename đổi tên nhưng KHÔNG đổi slug — slug nằm trong URL đã được index, xem
// đặc tả P1.1 mục 2.3. Muốn đổi slug thì gọi SetSlug tường minh.
func (b *Brand) Rename(name string) error {
	n, err := normalizeTaxonomyName(name, ErrBrandNameInvalid)
	if err != nil {
		return err
	}
	b.Name = n
	b.UpdatedAt = time.Now().UTC()
	return nil
}

func (b *Brand) SetSlug(slug string) error {
	s, err := NewSlug(slug)
	if err != nil {
		return err
	}
	b.Slug = s
	b.UpdatedAt = time.Now().UTC()
	return nil
}

func (b *Brand) Validate() error {
	if b == nil || b.ID == uuid.Nil || b.Slug == "" || b.Name == "" {
		return ErrUnknownBrand
	}
	return nil
}

// Brands là danh sách thương hiệu đi qua cache.
//
// Validate kiểm TỪNG phần tử: GetOrLoad chỉ từ chối `null` ở cấp ngoài cùng,
// còn `[null]` giải mã thành slice có phần tử nil và nổ ở chỗ dùng.
type Brands []*Brand

func (bs Brands) Validate() error {
	for _, b := range bs {
		if err := b.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTaxonomyName(name string, invalid error) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxTaxonomyNameLen {
		return "", invalid
	}
	return name, nil
}
