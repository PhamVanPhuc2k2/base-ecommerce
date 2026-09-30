package usecase

import "context"

// SitemapPageSize là số sản phẩm mỗi file sitemap. Google cho tới 50.000;
// 5.000 giữ mỗi file nhỏ (vài trăm KB) và mỗi câu OFFSET ngắn.
const SitemapPageSize = 5000

type SitemapPage struct {
	Items      []SitemapEntry
	Page       int
	Total      int
	TotalPages int
}

type SitemapProducts struct{ repo ProductRepository }

func NewSitemapProducts(repo ProductRepository) *SitemapProducts {
	return &SitemapProducts{repo: repo}
}

// Execute trả trang page (bắt đầu từ 1). Trang vượt quá trả rỗng chứ không
// báo lỗi: storefront tính số file từ total ở lần gọi TRƯỚC, giữa hai lần sản
// phẩm có thể bị gỡ bớt — file cuối rỗng vẫn là sitemap hợp lệ, 4xx thì không.
//
// KHÔNG có trần MaxPage như danh sách: OFFSET ở đây chạy trên index hẹp
// (Index Only Scan, không đọc heap), khác hẳn câu danh sách phải sắp và đọc
// cả dòng. Đó là lý do sitemap có endpoint riêng.
func (uc *SitemapProducts) Execute(ctx context.Context, page int) (*SitemapPage, error) {
	if page < 1 {
		page = 1
	}
	total, err := uc.repo.CountLive(ctx)
	if err != nil {
		return nil, err
	}
	items, err := uc.repo.Sitemap(ctx, (page-1)*SitemapPageSize, SitemapPageSize)
	if err != nil {
		return nil, err
	}
	return &SitemapPage{Items: items, Page: page, Total: total,
		TotalPages: (total + SitemapPageSize - 1) / SitemapPageSize}, nil
}
