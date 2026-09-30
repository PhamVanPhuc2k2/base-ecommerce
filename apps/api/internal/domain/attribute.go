package domain

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"base-ecommerce/api/pkg/errs"

	"github.com/google/uuid"
)

type AttributeType string

const (
	AttrText    AttributeType = "text"
	AttrNumber  AttributeType = "number"
	AttrBoolean AttributeType = "boolean"
	AttrEnum    AttributeType = "enum"
)

func (t AttributeType) Valid() bool {
	switch t {
	case AttrText, AttrNumber, AttrBoolean, AttrEnum:
		return true
	}
	return false
}

const (
	maxAttrValueLen   = 200
	maxAttrUnitLen    = 20
	maxAttrEnumValues = 100
)

var (
	attrCodePattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
	// Số thập phân THUẦN, không đơn vị: "16", "15.6", "-3". Đơn vị ("GB",
	// "inch") nằm ở định nghĩa — lưu "16GB" thì không so sánh, không sắp xếp
	// được, và mỗi người bán viết đơn vị một kiểu.
	attrNumberPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// AttributeDefinition nói một khóa trong products.attributes /
// product_variants.options có nghĩa gì: tên hiển thị, kiểu, đơn vị.
//
// Code, Type và Variant BẤT BIẾN sau khi tạo: đổi chúng làm dữ liệu đã lưu sai
// nghĩa mà không ai được báo. Đặc tả P1.3 mục 2.5.
type AttributeDefinition struct {
	ID         uuid.UUID
	Code       string
	Name       string
	Type       AttributeType
	Unit       string
	Options    []string
	Filterable bool
	Variant    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewAttributeDefinition(code, name string, typ AttributeType, unit string,
	options []string, filterable, variant bool) (*AttributeDefinition, error) {

	code = strings.TrimSpace(code)
	if !attrCodePattern.MatchString(code) || len(code) > maxOptionKeyLen {
		return nil, ErrInvalidAttributeCode
	}
	if !typ.Valid() {
		return nil, ErrInvalidAttributeType
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	d := &AttributeDefinition{ID: id, Code: code, Type: typ, Filterable: filterable,
		Variant: variant, CreatedAt: now, UpdatedAt: now}
	if err := d.Update(&name, &unit, options, nil); err != nil {
		return nil, err
	}
	d.UpdatedAt = now
	return d, nil
}

// Update sửa những gì an toàn để sửa. nil = giữ nguyên.
func (d *AttributeDefinition) Update(name, unit *string, options []string, filterable *bool) error {
	if name != nil {
		n, err := normalizeTaxonomyName(*name, ErrAttributeNameInvalid)
		if err != nil {
			return err
		}
		d.Name = n
	}
	if unit != nil {
		u := strings.TrimSpace(*unit)
		if utf8.RuneCountInString(u) > maxAttrUnitLen {
			return ErrInvalidAttributeOptions
		}
		d.Unit = u
	}
	if options != nil || d.Options == nil {
		opts, err := normalizeEnumOptions(d.Type, options)
		if err != nil {
			return err
		}
		d.Options = opts
	}
	if filterable != nil {
		d.Filterable = *filterable
	}
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// normalizeEnumOptions: enum PHẢI có danh sách giá trị không trùng, kiểu khác
// KHÔNG được có — cùng quy tắc với ràng buộc attribute_definitions_enum_options.
func normalizeEnumOptions(typ AttributeType, in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || utf8.RuneCountInString(v) > maxOptionValueLen || slices.Contains(out, v) {
			return nil, ErrInvalidAttributeOptions
		}
		out = append(out, v)
	}
	if (typ == AttrEnum) != (len(out) > 0) || len(out) > maxAttrEnumValues {
		return nil, ErrInvalidAttributeOptions
	}
	return out, nil
}

// checkValue trả về mã lỗi cấp trường, rỗng nghĩa là hợp lệ.
func (d *AttributeDefinition) checkValue(v string) string {
	switch d.Type {
	case AttrText:
		if strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > maxAttrValueLen {
			return FieldInvalidAttributeValue
		}
	case AttrNumber:
		if !attrNumberPattern.MatchString(v) {
			return FieldInvalidAttributeValue
		}
	case AttrBoolean:
		if v != "true" && v != "false" {
			return FieldInvalidAttributeValue
		}
	case AttrEnum:
		if !slices.Contains(d.Options, v) {
			return FieldInvalidAttributeValue
		}
	}
	return ""
}

func (d *AttributeDefinition) validate() error {
	if d == nil || d.ID == uuid.Nil || !attrCodePattern.MatchString(d.Code) || !d.Type.Valid() || d.Name == "" {
		return ErrUnknownAttribute
	}
	return nil
}

// CategoryAttribute gán một định nghĩa cho một danh mục.
type CategoryAttribute struct {
	CategoryID  uuid.UUID
	AttributeID uuid.UUID
	Required    bool
	Position    int
}

// AttributeCatalog là TOÀN BỘ định nghĩa và phép gán — vài trăm dòng, đọc một
// lần và cache một khóa. Đặc tả P1.3 mục 2.6.
type AttributeCatalog struct {
	Definitions []*AttributeDefinition
	Assignments []CategoryAttribute
}

// Validate cho cache: phần tử nil hay rỗng thì từ chối cả catalog, và phép gán
// trỏ tới định nghĩa không có trong catalog cũng vậy — SchemaFor sẽ bỏ qua nó
// một cách im lặng và validate lỏng hơn thực tế.
func (c *AttributeCatalog) Validate() error {
	ids := make(map[uuid.UUID]bool, len(c.Definitions))
	for _, d := range c.Definitions {
		if err := d.validate(); err != nil {
			return err
		}
		ids[d.ID] = true
	}
	for _, a := range c.Assignments {
		if a.CategoryID == uuid.Nil || !ids[a.AttributeID] {
			return ErrUnknownAttribute
		}
	}
	return nil
}

// SchemaEntry là một thuộc tính trong tập hiệu lực của một danh mục.
type SchemaEntry struct {
	Def      *AttributeDefinition
	Required bool
	Position int
	// Inherited: gán ở tổ tiên chứ không ở chính danh mục này.
	Inherited bool
}

// Schema là tập thuộc tính hiệu lực của một danh mục: gán cho nó CỘNG mọi tổ
// tiên. Rỗng nghĩa là danh mục ở chế độ tự do như P0 (đặc tả mục 2.2).
type Schema struct {
	entries []SchemaEntry
	byCode  map[string]SchemaEntry
}

// SchemaFor dựng tập hiệu lực. Danh mục con gán lại cùng thuộc tính thì cấu
// hình của CON thắng (required/position) — gần danh mục của sản phẩm hơn là cụ
// thể hơn.
func (c *AttributeCatalog) SchemaFor(t *Tree, categoryID uuid.UUID) *Schema {
	defs := make(map[uuid.UUID]*AttributeDefinition, len(c.Definitions))
	for _, d := range c.Definitions {
		defs[d.ID] = d
	}
	byCategory := map[uuid.UUID][]CategoryAttribute{}
	for _, a := range c.Assignments {
		byCategory[a.CategoryID] = append(byCategory[a.CategoryID], a)
	}

	s := &Schema{byCode: map[string]SchemaEntry{}}
	path := t.Ancestors(categoryID) // gốc → ... → chính nó
	for i, catID := range path {
		for _, a := range byCategory[catID] {
			d := defs[a.AttributeID]
			if d == nil {
				continue
			}
			s.byCode[d.Code] = SchemaEntry{Def: d, Required: a.Required, Position: a.Position,
				Inherited: i < len(path)-1}
		}
	}
	for _, e := range s.byCode {
		s.entries = append(s.entries, e)
	}
	sort.Slice(s.entries, func(i, j int) bool {
		if s.entries[i].Position != s.entries[j].Position {
			return s.entries[i].Position < s.entries[j].Position
		}
		return s.entries[i].Def.Code < s.entries[j].Def.Code
	})
	return s
}

func (s *Schema) Empty() bool            { return len(s.entries) == 0 }
func (s *Schema) Entries() []SchemaEntry { return s.entries }

func (s *Schema) Lookup(code string) (SchemaEntry, bool) {
	e, ok := s.byCode[code]
	return e, ok
}

// Mã lỗi CẤP TRƯỜNG — nằm trong errors[].code của VALIDATION_FAILED, không
// phải Problem.code. Message cố định, không nội suy dữ liệu người dùng: tên
// trường đã có ở errors[].field.
const (
	FieldUnknownAttribute          = "UNKNOWN_ATTRIBUTE_KEY"
	FieldInvalidAttributeValue     = "INVALID_ATTRIBUTE_VALUE"
	FieldAttributeRequired         = "ATTRIBUTE_REQUIRED"
	FieldVariantAttributeOnProduct = "VARIANT_ATTRIBUTE_ON_PRODUCT"
	FieldProductAttributeOnVariant = "PRODUCT_ATTRIBUTE_ON_VARIANT"
)

var fieldMessages = map[string]string{
	FieldUnknownAttribute:          "Danh mục của sản phẩm không có thuộc tính này",
	FieldInvalidAttributeValue:     "Giá trị không đúng kiểu hoặc không nằm trong danh sách cho phép",
	FieldAttributeRequired:         "Thuộc tính bắt buộc khi sản phẩm đang bán",
	FieldVariantAttributeOnProduct: "Đây là thuộc tính của phiên bản, phải đặt trong options của phiên bản",
	FieldProductAttributeOnVariant: "Đây là thuộc tính của sản phẩm, không đặt trong options của phiên bản",
}

// CheckProduct validate attributes và options của mọi variant theo schema.
//
// Gom ĐỦ mọi lỗi rồi trả một lần (errs.Validation với errors[] theo trường) —
// người quản trị sửa một lần thay vì gửi đi gửi lại, mỗi lần lộ thêm một lỗi.
// Schema rỗng thì không kiểm gì: danh mục ở chế độ tự do.
func (s *Schema) CheckProduct(p *Product) error {
	if s.Empty() {
		return nil
	}
	var fields []errs.FieldError
	add := func(field, code string) {
		fields = append(fields, errs.FieldError{Field: field, Code: code, Message: fieldMessages[code]})
	}

	for _, k := range sortedKeys(p.Attributes) {
		e, ok := s.byCode[k]
		switch {
		case !ok:
			add("attributes."+k, FieldUnknownAttribute)
		case e.Def.Variant:
			add("attributes."+k, FieldVariantAttributeOnProduct)
		default:
			if c := e.Def.checkValue(p.Attributes[k]); c != "" {
				add("attributes."+k, c)
			}
		}
	}
	for i, v := range p.Variants {
		for _, k := range sortedKeys(v.Options) {
			field := fmt.Sprintf("variants[%d].options.%s", i, k)
			e, ok := s.byCode[k]
			switch {
			case !ok:
				add(field, FieldUnknownAttribute)
			case !e.Def.Variant:
				add(field, FieldProductAttributeOnVariant)
			default:
				if c := e.Def.checkValue(v.Options[k]); c != "" {
					add(field, c)
				}
			}
		}
	}

	// required chỉ áp khi live — nháp được phép dở dang, cùng tinh thần quy
	// tắc ảnh/giá của P0.2 và P1.2.
	if p.Status == StatusLive {
		for _, e := range s.entries {
			if !e.Required {
				continue
			}
			if !e.Def.Variant {
				if _, ok := p.Attributes[e.Def.Code]; !ok {
					add("attributes."+e.Def.Code, FieldAttributeRequired)
				}
				continue
			}
			for i, v := range p.Variants {
				if _, ok := v.Options[e.Def.Code]; !ok && v.Status == VariantActive {
					add(fmt.Sprintf("variants[%d].options.%s", i, e.Def.Code), FieldAttributeRequired)
				}
			}
		}
	}

	if len(fields) > 0 {
		return errs.Validation(fields...)
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
