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
