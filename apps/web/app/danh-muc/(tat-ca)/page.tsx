import type { Metadata } from 'next'
import { permanentRedirect } from 'next/navigation'
import { ProductListing } from '@/components/product-listing'
import { categoryHref } from '@/lib/categories'
import { clearAttrs, hrefCurrent, hrefWith, toSearchParams } from '@/lib/search-params'

const BASE_PATH = '/danh-muc'

/**
 * Trang danh mục render theo từng yêu cầu, KHÔNG dựng sẵn.
 *
 * Toàn bộ bộ lọc nằm trên query string, nên mỗi tổ hợp thương hiệu × khoảng giá
 * × thuộc tính × sắp xếp × số trang là một biến thể riêng — dựng sẵn hết là bất
 * khả thi. Cache đúng chỗ nằm ở tầng khác: Redis trong backend và CDN phía
 * trước theo URL đầy đủ.
 *
 * Lưu ý: `force-dynamic` ép MỌI `fetch` trong trang thành `no-store` — đừng
 * truyền `revalidate` cho `apiGet` ở đây rồi tưởng nó có tác dụng.
 */
export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'Danh mục sản phẩm',
  description:
    'Danh sách laptop, PC và linh kiện máy tính chính hãng. Lọc theo danh mục, khoảng giá và sắp xếp theo giá hoặc hàng mới về.',
  alternates: { canonical: BASE_PATH },
}

/**
 * Link một mục trong cột danh mục: sang trang `/danh-muc/<slug>`, giữ sắp xếp
 * và khoảng giá, bỏ `attr.*` (thuộc tính thuộc về danh mục cũ).
 */
function categoryLink(params: URLSearchParams) {
  return (slug: string | undefined) =>
    hrefWith(slug === undefined ? BASE_PATH : categoryHref(slug), params, {
      ...clearAttrs(params),
      category: undefined,
    })
}

export default async function Page({ searchParams }: PageProps<'/danh-muc'>) {
  const params = toSearchParams(await searchParams)

  /*
    `?category=<slug>` là URL của P0.4–P1.4. Từ P1.5 danh mục nằm trong ĐƯỜNG
    DẪN. Chuyển 308 (vĩnh viễn) kèm mọi tham số còn lại: link cũ đã được index
    dồn tín hiệu xếp hạng sang URL mới, thay vì hai URL cùng một nội dung chia
    đôi nó. Đặc tả P1.5 mục 2.2.
  */
  const legacy = params.get('category')
  if (legacy !== null && legacy !== '') {
    const rest = new URLSearchParams(params)
    rest.delete('category')
    permanentRedirect(hrefCurrent(categoryHref(legacy), rest))
  }

  return (
    <ProductListing
      params={params}
      basePath={BASE_PATH}
      fixed={{}}
      heading="Tất cả sản phẩm"
      crumbs={[{ label: 'Trang chủ', href: '/' }, { label: 'Danh mục sản phẩm' }]}
      categoryHref={categoryLink(params)}
    />
  )
}
