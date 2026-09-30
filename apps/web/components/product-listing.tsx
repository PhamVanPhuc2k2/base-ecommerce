import Link from 'next/link'
import { Breadcrumb, type Crumb } from '@/components/breadcrumb'
import { CategoryFilter } from '@/components/category-filter'
import { ErrorState } from '@/components/error-state'
import { FacetFilter } from '@/components/facet-filter'
import { Pagination } from '@/components/pagination'
import { ProductCard } from '@/components/product-card'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { hrefCurrent, hrefWith } from '@/lib/search-params'

type ProductList = components['schemas']['ProductList']
type Category = components['schemas']['Category']
type Facet = components['schemas']['Facet']

/*
  Danh sách sản phẩm dùng chung cho ba trang (P1.5, đặc tả mục 2.1):

    /danh-muc               tất cả sản phẩm
    /danh-muc/<slug>        cố định category (trong ĐƯỜNG DẪN, không trong query)
    /thuong-hieu/<slug>     cố định brand

  Ba trang khác nhau đúng ở: tham số cố định gửi xuống API (`fixed`), đường dẫn
  gốc cho mọi link lọc/sắp xếp/phân trang (`basePath`), cách dựng link danh mục
  (`categoryHref`), và tiêu đề/breadcrumb. Mọi thứ còn lại — nhất là cách bắt
  lỗi — nằm ở MỘT chỗ; ba bản chép tay sẽ lệch nhau ở đúng những chỗ đã khó làm
  đúng nhất.
*/

/** Tham số người dùng được phép chuyển tiếp xuống backend, theo api/openapi.yaml. */
const FORWARDED_PARAMS = ['category', 'brand', 'sort', 'page', 'limit', 'price_min', 'price_max']

const SORT_OPTIONS: { value: string; label: string }[] = [
  { value: 'newest', label: 'Mới nhất' },
  { value: 'price_asc', label: 'Giá thấp đến cao' },
  { value: 'price_desc', label: 'Giá cao đến thấp' },
]

/** Backend mặc định về 'newest' khi thiếu `sort`, nên giao diện phải sáng đúng nút đó. */
const DEFAULT_SORT = 'newest'

const ATTR_PREFIX = 'attr.'

/**
 * Query string gửi xuống backend: tham số trên URL (danh sách trắng) + tham số
 * cố định của trang, cố định THẮNG — `/thuong-hieu/asus?brand=dell` vẫn là
 * trang Asus.
 *
 * Danh sách trắng chứ không chuyển tiếp tất cả: URL là thứ người lạ tự gõ được,
 * và chuyển tiếp mù nghĩa là ai cũng gắn thêm được tham số nội bộ vào lời gọi
 * API của chúng ta.
 */
function buildApiQuery(params: URLSearchParams, fixed: Fixed): string {
  const query = new URLSearchParams()
  for (const key of FORWARDED_PARAMS) {
    const value = params.get(key)
    if (value !== null && value !== '') query.set(key, value)
  }
  /*
    `attr.<tên>` phải TỰ NỐI bằng tay, không đi qua type đã sinh được: thuộc
    tính do quản trị tự đặt nên openapi.yaml không liệt kê hết được, và object
    `query` mà openapi-typescript sinh ra là object ĐÓNG. Chỉ nhận khóa còn phần
    tên sau dấu chấm, để không gửi xuống một `attr.=` trống nghĩa.
  */
  for (const [key, value] of params.entries()) {
    if (!key.startsWith(ATTR_PREFIX)) continue
    if (key.length <= ATTR_PREFIX.length || value === '') continue
    query.set(key, value)
  }
  if (fixed.category !== undefined) query.set('category', fixed.category)
  if (fixed.brand !== undefined) query.set('brand', fixed.brand)
  return query.toString()
}

type Fixed = { category?: string; brand?: string }

export async function ProductListing({
  params,
  basePath,
  fixed,
  heading,
  crumbs,
  categoryHref,
}: {
  /** Tham số trên URL của người dùng. */
  params: URLSearchParams
  /** Đường dẫn trang đang đứng — gốc của mọi link lọc, sắp xếp, phân trang. */
  basePath: string
  fixed: Fixed
  heading: string
  crumbs: Crumb[]
  /** Link cho một mục trong cột danh mục; `undefined` là "Tất cả". */
  categoryHref: (slug: string | undefined) => string
}) {
  const apiQuery = buildApiQuery(params, fixed)
  const activeCategory = fixed.category ?? params.get('category') ?? undefined
  const activeSort = params.get('sort') ?? DEFAULT_SORT

  /*
    Gọi SONG SONG bằng allSettled, không phải await nối tiếp: các lời gọi không
    phụ thuộc nhau. Và `allSettled` chứ không phải `all`: cây danh mục hay facet
    hỏng không được kéo sập danh sách sản phẩm đang lấy về bình thường — bộ lọc
    là thứ phụ, mất nó thì trang vẫn bán được hàng.

    Next.js ghi nhớ fetch GET giống hệt nhau trong một lượt render, nên trang
    gọi `/categories` để kiểm 404 rồi component này gọi lại cũng chỉ tốn MỘT
    request.
  */
  const [listResult, treeResult, facetResult] = await Promise.allSettled([
    apiGet<ProductList>(apiQuery === '' ? '/products' : `/products?${apiQuery}`),
    apiGet<{ data: Category[] }>('/categories'),
    // Facet chỉ có nghĩa trong một danh mục (thuộc tính thuộc về danh mục).
    // Cùng query string với danh sách: số cạnh mỗi giá trị phải khớp với số
    // sản phẩm khách sẽ thấy khi bấm vào.
    activeCategory !== undefined
      ? apiGet<{ data: Facet[] }>(`/products/facets?${apiQuery}`)
      : Promise.resolve({ data: [] as Facet[] }),
  ])

  /*
    ======================================================================
    LỖI PHẢI ĐƯỢC BẮT NGAY TẠI ĐÂY, không được để bay lên app/error.tsx.
    ======================================================================
    Đã ĐO ở P0.4 Task 3: React tước sạch thuộc tính tùy biến của lỗi ném ra từ
    Server Component trước khi giao cho error boundary, ở CẢ dev lẫn production
    (error.code / .status / .requestId → undefined). Để `ApiError` bay lên
    `error.tsx` thì khách nhận đúng một câu chung chung cho mọi thứ và mất luôn
    `request_id`. Đoạn này vẫn chạy trên server — chỗ DUY NHẤT còn giữ được
    `code` và `requestId`.
  */
  if (listResult.status === 'rejected') {
    const reason: unknown = listResult.reason
    // Không phải ApiError là lỗi lập trình thật — ném tiếp cho error.tsx.
    if (!(reason instanceof ApiError)) throw reason
    return (
      <div>
        <Breadcrumb items={crumbs} />
        <h1 className="mt-3 mb-8 text-2xl font-semibold text-gray-900">{heading}</h1>
        <ErrorState code={reason.code} requestId={reason.requestId}>
          <EscapeActions basePath={basePath} params={params} />
        </ErrorState>
      </div>
    )
  }

  const list = listResult.value
  // Cây/facet hỏng thì bỏ hẳn khối lọc tương ứng chứ không dựng thêm khối lỗi
  // thứ hai: hai thông báo lỗi trên một trang chỉ làm khách hoang mang.
  const tree = treeResult.status === 'fulfilled' ? treeResult.value.data : []
  const facets = facetResult.status === 'fulfilled' ? facetResult.value.data : []

  return (
    <div>
      <Breadcrumb items={crumbs} />

      {/* Đúng một <h1> mỗi trang (README mục 7.4). Không bọc <main>: layout đã có. */}
      <h1 className="mt-3 text-2xl font-semibold text-gray-900">{heading}</h1>
      <p className="mt-1 text-sm text-gray-500">
        {list.meta.total > 0 ? `${list.meta.total} sản phẩm` : 'Không có sản phẩm nào khớp bộ lọc'}
      </p>

      <div className="mt-6 gap-8 lg:flex">
        <aside className="mb-6 shrink-0 lg:mb-0 lg:w-56">
          {tree.length > 0 ? (
            <CategoryFilter tree={tree} activeSlug={activeCategory} hrefFor={categoryHref} />
          ) : null}
          <FacetFilter facets={facets} basePath={basePath} params={params} />
        </aside>

        {/* min-w-0: không có nó, một tên sản phẩm dài kéo giãn cột flex và
            đẩy cả lưới tràn ngang ra khỏi màn hình điện thoại. */}
        <div className="min-w-0 flex-1">
          <SortBar basePath={basePath} activeSort={activeSort} params={params} />

          {list.data.length === 0 ? (
            <EmptyState basePath={basePath} params={params} hasPrev={list.meta.has_prev} />
          ) : (
            <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {list.data.map((product, index) => (
                <li key={product.id}>
                  {/* Bốn ô đầu là hàng trên cùng ở màn hình rộng — ảnh tính LCP. */}
                  <ProductCard product={product} preload={index < 4} />
                </li>
              ))}
            </ul>
          )}

          <Pagination meta={list.meta} basePath={basePath} params={params} />
        </div>
      </div>
    </div>
  )
}

/** Dải nút sắp xếp. Cũng là link, cùng lý do với bộ lọc danh mục. */
function SortBar({
  basePath,
  activeSort,
  params,
}: {
  basePath: string
  activeSort: string
  params: URLSearchParams
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-gray-200 pb-3">
      <span className="text-sm text-gray-500">Sắp xếp:</span>
      {SORT_OPTIONS.map((option) => {
        const active = option.value === activeSort
        return (
          <Link
            key={option.value}
            // Đổi cách sắp xếp thì về trang 1 — hrefWith tự làm việc đó: trang 5
            // của danh sách sắp kiểu khác là một tập sản phẩm hoàn toàn khác.
            href={hrefWith(basePath, params, { sort: option.value })}
            aria-current={active ? 'true' : undefined}
            className={
              active
                ? 'rounded-md bg-brand px-3 py-1.5 text-sm font-medium text-white'
                : 'rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:border-brand hover:text-brand'
            }
          >
            {option.label}
          </Link>
        )
      })}
    </div>
  )
}

/**
 * Không có sản phẩm nào khớp. Lưới trống trông y hệt một trang đang hỏng —
 * khách cần biết đây là do bộ lọc quá hẹp, và cần một nút để thoát ra.
 */
function EmptyState({
  basePath,
  params,
  hasPrev,
}: {
  basePath: string
  params: URLSearchParams
  hasPrev: boolean
}) {
  return (
    <div className="mt-4 rounded-lg border border-dashed border-gray-300 bg-gray-50 p-10 text-center">
      {/*
        Hai tình huống khác hẳn nhau: "bộ lọc không ra kết quả" và "đi quá số
        trang thật sự có" (sửa tay ?page=200 khi chỉ có 2 trang). Một câu chung
        cho cả hai khiến người ở trường hợp sau tưởng mình lọc sai.
      */}
      <p className="text-base font-medium text-gray-900">
        {hasPrev ? 'Trang này không còn sản phẩm nào' : 'Không tìm thấy sản phẩm phù hợp'}
      </p>
      <p className="mt-2 text-sm text-gray-600">
        {hasPrev
          ? 'Bạn đã đi quá trang cuối của danh sách. Hãy quay lại trang đầu.'
          : 'Bộ lọc hiện tại đang quá hẹp. Hãy bỏ bớt điều kiện để xem thêm sản phẩm.'}
      </p>
      <div className="mt-6 flex flex-wrap justify-center gap-3">
        <EscapeActions basePath={basePath} params={params} />
      </div>
    </div>
  )
}

/**
 * Các nút đưa khách thoát khỏi một trang không có gì để xem. Chỉ render nút
 * THẬT SỰ dẫn đi đâu đó — nút trỏ về đúng URL đang đứng thì bấm không có gì xảy
 * ra, và khách kết luận trang bị treo. Khi đó thứ duy nhất có ý nghĩa là tải lại.
 */
function EscapeActions({ basePath, params }: { basePath: string; params: URLSearchParams }) {
  const current = hrefCurrent(basePath, params)
  const firstPage = hrefWith(basePath, params, { page: undefined })

  const actions: { href: string; label: string; primary: boolean }[] = []
  if (firstPage !== current) actions.push({ href: firstPage, label: 'Về trang đầu', primary: true })
  if (basePath !== current && basePath !== firstPage) {
    actions.push({ href: basePath, label: 'Xóa toàn bộ bộ lọc', primary: actions.length === 0 })
  }

  if (actions.length === 0) {
    return (
      // Thẻ <a> trần, KHÔNG phải <Link>: <Link> trỏ về chính route đang hiển
      // thị thì router có thể không gọi lại server — mà thứ khách cần chính là
      // một lượt tải mới, backend vừa chết có thể đã sống lại.
      <a
        href={current}
        className="rounded-md bg-brand px-5 py-2.5 text-sm font-medium text-white hover:bg-brand-dark"
      >
        Tải lại trang
      </a>
    )
  }

  return (
    <>
      {actions.map((action) => (
        <Link
          key={action.href}
          href={action.href}
          className={
            action.primary
              ? 'rounded-md bg-brand px-5 py-2.5 text-sm font-medium text-white hover:bg-brand-dark'
              : 'rounded-md border border-gray-300 px-5 py-2.5 text-sm font-medium text-gray-700 hover:border-gray-400'
          }
        >
          {action.label}
        </Link>
      ))}
    </>
  )
}
