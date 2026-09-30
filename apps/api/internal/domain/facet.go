package domain

// FacetCount là số sản phẩm có Code = Value trong tập kết quả hiện tại.
type FacetCount struct {
	Code  string
	Value string
	Count int
}

// FacetCounts đi qua cache (TTL 60 giây). Validate từ chối phần tử rỗng —
// `[{}]` giải mã thành phần tử zero value, và storefront sẽ hiện một nút lọc
// không tên dẫn tới "?attr.=".
type FacetCounts []FacetCount

func (fs FacetCounts) Validate() error {
	for _, f := range fs {
		if f.Code == "" || f.Value == "" || f.Count <= 0 {
			return ErrUnknownAttribute
		}
	}
	return nil
}
