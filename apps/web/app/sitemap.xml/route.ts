import { apiGet } from '@/lib/api/server'
import { absoluteUrl } from '@/lib/site'
import { sitemapIndex, xmlResponse } from '@/lib/sitemap-xml'

/**
 * `/sitemap.xml` — sitemap INDEX (P1.5), trỏ tới:
 *
 *   /sitemaps/pages.xml          trang chủ, danh mục, thương hiệu
 *   /sitemaps/products-<n>.xml   5.000 sản phẩm mỗi file
 *
 * Bỏ trần 20.000 sản phẩm của P0.4 (max_page 200 × limit 100): số file sản
 * phẩm tính LÚC CHẠY từ `total_pages` của API, không lúc build.
 */
export const dynamic = 'force-dynamic'

type SitemapMeta = { meta: { total_pages: number } }

export async function GET() {
  const locs = [absoluteUrl('/sitemaps/pages.xml')]
  try {
    const { meta } = await apiGet<SitemapMeta>('/sitemap/products?page=1')
    for (let n = 1; n <= meta.total_pages; n++) {
      locs.push(absoluteUrl(`/sitemaps/products-${n}.xml`))
    }
  } catch (e) {
    /*
      API chết KHÔNG được làm sập `/sitemap.xml`: một sitemap trả 500 là tín
      hiệu xấu kéo dài với Google. Index chỉ còn `pages.xml` vẫn là XML hợp lệ
      và vẫn dẫn bot tới danh mục — nơi có link tới từng sản phẩm.

      Ghi log là BẮT BUỘC: một index thiếu file sản phẩm trông y hệt cửa hàng
      chưa có hàng, không có dòng log này thì không ai biết nó đang hỏng.
    */
    console.error('[sitemap] không lấy được số trang sản phẩm, index chỉ còn pages.xml', {
      cause: e instanceof Error ? e.message : String(e),
    })
  }
  return xmlResponse(sitemapIndex(locs))
}
