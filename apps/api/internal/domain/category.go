package domain

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Slug      string
	Name      string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCategory dựng danh mục mới. Cha có tồn tại hay không là việc của use case
// (nó có cây trong tay), không phải của hàm dựng.
func NewCategory(name, slug string, parentID *uuid.UUID, position int) (*Category, error) {
	name, err := normalizeTaxonomyName(name, ErrCategoryNameInvalid)
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
	return &Category{ID: id, ParentID: parentID, Slug: s, Name: name,
		Position: position, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename đổi tên nhưng KHÔNG đổi slug: slug là URL bộ lọc (?category=laptop)
// đã nằm trong chỉ mục tìm kiếm. Xem đặc tả P1.1 mục 2.3.
func (c *Category) Rename(name string) error {
	n, err := normalizeTaxonomyName(name, ErrCategoryNameInvalid)
	if err != nil {
		return err
	}
	c.Name = n
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Category) SetSlug(slug string) error {
	s, err := NewSlug(slug)
	if err != nil {
		return err
	}
	c.Slug = s
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Category) SetPosition(p int) {
	c.Position = p
	c.UpdatedAt = time.Now().UTC()
}

// MoveTo đổi cha. nil = lên làm gốc. Không tự kiểm vòng lặp — việc đó cần cả
// cây, gọi Tree.CheckMove TRƯỚC khi gọi hàm này.
func (c *Category) MoveTo(parentID *uuid.UUID) {
	c.ParentID = parentID
	c.UpdatedAt = time.Now().UTC()
}

func (c *Category) Validate() error {
	if c == nil || c.ID == uuid.Nil || c.Slug == "" || c.Name == "" {
		return ErrUnknownCategory
	}
	return nil
}

// Categories là danh sách phẳng đi qua cache. Validate kiểm từng phần tử vì
// `[null]` giải mã thành phần tử nil, và NewTree deref nó thì cả tiến trình
// panic — lỗ này có từ P0.2, xem đặc tả P1.1 mục 2.6.
type Categories []*Category

func (cs Categories) Validate() error {
	for _, c := range cs {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Tree là cây danh mục dựng trong bộ nhớ từ danh sách phẳng.
//
// Đây là mấu chốt của quyết định dùng adjacency list: đọc toàn bộ danh mục một
// lần, cache lại, rồi giải ID con cháu TRONG GO. Truy vấn sản phẩm theo nhánh
// nhờ vậy trở thành câu SQL phẳng `category_id = ANY($1)` dùng index thường,
// không cần recursive CTE mỗi lần và không cần cột path phải bảo trì.
type Tree struct {
	byID     map[uuid.UUID]*Category
	bySlug   map[string]*Category
	children map[uuid.UUID][]*Category
	roots    []*Category
}

func NewTree(cats []*Category) *Tree {
	t := &Tree{
		byID:     make(map[uuid.UUID]*Category, len(cats)),
		bySlug:   make(map[string]*Category, len(cats)),
		children: make(map[uuid.UUID][]*Category),
	}
	for _, c := range cats {
		t.byID[c.ID] = c
		t.bySlug[c.Slug] = c
	}
	for _, c := range cats {
		if c.ParentID == nil {
			t.roots = append(t.roots, c)
			continue
		}
		t.children[*c.ParentID] = append(t.children[*c.ParentID], c)
	}
	return t
}

func (t *Tree) Roots() []*Category { return t.roots }

func (t *Tree) Children(id uuid.UUID) []*Category { return t.children[id] }

func (t *Tree) BySlug(slug string) (*Category, bool) {
	c, ok := t.bySlug[slug]
	return c, ok
}

func (t *Tree) ByID(id uuid.UUID) (*Category, bool) {
	c, ok := t.byID[id]
	return c, ok
}

// CheckMove cho biết có được chuyển danh mục id vào dưới newParent không.
//
// Cấm khi newParent là chính nó hoặc nằm trong cây con của nó — cả hai đều tạo
// vòng lặp, mà vòng lặp làm cả nhánh biến mất khỏi Roots() không một lỗi nào.
//
// ⚠️ Chỉ đúng khi cây được đọc SAU khi đã khóa bảng categories trong cùng
// transaction. Kiểm trên cây cũ (từ cache, hoặc đọc trước khi khóa) thì hai
// lệnh chuyển đồng thời A→dưới B và B→dưới A đều qua, và cùng nhau tạo vòng
// lặp. Xem đặc tả P1.1 mục 2.1.
func (t *Tree) CheckMove(id uuid.UUID, newParent *uuid.UUID) error {
	if newParent == nil {
		return nil
	}
	if _, ok := t.byID[*newParent]; !ok {
		return ErrCategoryNotFound
	}
	for _, d := range t.DescendantIDs(id) {
		if d == *newParent {
			return ErrCategoryCycle
		}
	}
	return nil
}

// DescendantIDs trả về id của root KÈM toàn bộ con cháu.
//
// Có tập visited để dữ liệu lỗi tạo thành vòng lặp cũng không treo tiến trình.
// Ràng buộc khóa ngoại không ngăn được vòng lặp trong cây tự tham chiếu.
func (t *Tree) DescendantIDs(root uuid.UUID) []uuid.UUID {
	if _, ok := t.byID[root]; !ok {
		return nil
	}
	var (
		out     []uuid.UUID
		visited = make(map[uuid.UUID]bool)
		stack   = []uuid.UUID{root}
	)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[id] {
			continue
		}
		visited[id] = true
		out = append(out, id)
		for _, child := range t.children[id] {
			stack = append(stack, child.ID)
		}
	}
	return out
}
