package domain

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type VariantStatus string

const (
	VariantActive   VariantStatus = "active"
	VariantInactive VariantStatus = "inactive"
)

func (s VariantStatus) Valid() bool { return s == VariantActive || s == VariantInactive }

const (
	maxVariantOptions = 10
	maxOptionKeyLen   = 50
	maxOptionValueLen = 100
)

// Variant là thứ thật sự đem bán: có SKU, có giá. Giỏ hàng và tồn kho (P3–P4)
// trỏ vào variant, không trỏ vào Product.
//
// Không có hàm dựng xuất khẩu: variant chỉ sinh ra QUA Product (NewProduct,
// AddVariant), vì các bất biến quan trọng — options không trùng, giá "từ" của
// sản phẩm, sản phẩm live phải còn variant bán được — đều cần nhìn cả aggregate.
type Variant struct {
	ID        uuid.UUID
	SKU       string
	Price     Money
	Options   map[string]string
	Status    VariantStatus
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// VariantInput là dữ liệu để tạo một variant mới.
type VariantInput struct {
	SKU      string
	Price    Money
	Options  map[string]string
	Position int
}

func newVariant(in VariantInput, now time.Time) (*Variant, error) {
	sku := strings.TrimSpace(in.SKU)
	if sku == "" || utf8.RuneCountInString(sku) > maxSKULen {
		return nil, ErrInvalidSKU
	}
	opts, err := normalizeOptions(in.Options)
	if err != nil {
		return nil, err
	}
	if in.Price.Currency() != SupportedCurrency {
		// Money{} rỗng (không qua NewMoney) có currency "" — chặn ở đây thay vì
		// để ràng buộc CHECK của Postgres báo thành 500.
		return nil, ErrInvalidPrice
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return &Variant{
		ID: id, SKU: sku, Price: in.Price, Options: opts, Status: VariantActive,
		Position: in.Position, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// normalizeOptions cắt khoảng trắng và kiểm giới hạn. nil thành map rỗng — cột
// là NOT NULL DEFAULT '{}' và JSON trả ra phải là {} chứ không phải null.
func normalizeOptions(in map[string]string) (map[string]string, error) {
	if len(in) > maxVariantOptions {
		return nil, ErrInvalidVariantOptions
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "" || v == "" ||
			utf8.RuneCountInString(k) > maxOptionKeyLen || utf8.RuneCountInString(v) > maxOptionValueLen {
			return nil, ErrInvalidVariantOptions
		}
		if _, dup := out[k]; dup {
			// " ram" và "ram" cùng thành "ram" sau khi cắt: hai khóa JSON khác
			// nhau gộp làm một, và một giá trị lặng lẽ biến mất.
			return nil, ErrInvalidVariantOptions
		}
		out[k] = v
	}
	return out, nil
}

// optionsKey là dạng chuẩn của options để so trùng. Sắp khóa vì map trong Go
// duyệt ngẫu nhiên — không sắp thì hai options giống hệt nhau có thể ra hai
// chuỗi khác nhau và lọt qua phép kiểm trùng.
func optionsKey(opts map[string]string) string {
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		// \x00 và \x01 không thể xuất hiện trong khóa/giá trị đã qua kiểm, nên
		// {"a":"b=c"} và {"a=b":"c"} không bao giờ ra cùng một chuỗi.
		b.WriteString(k)
		b.WriteByte(0)
		b.WriteString(opts[k])
		b.WriteByte(1)
	}
	return b.String()
}

func (v *Variant) validate() error {
	if v == nil || v.ID == uuid.Nil || strings.TrimSpace(v.SKU) == "" {
		return ErrInvalidSKU
	}
	if !v.Status.Valid() {
		return ErrInvalidVariantStatus
	}
	if v.Price.Currency() != SupportedCurrency {
		return ErrUnsupportedCurrency
	}
	return nil
}
