import type { Metadata } from 'next'
import Link from 'next/link'
import { Breadcrumb } from '@/components/breadcrumb'
import { CategoryFilter, findCategory } from '@/components/category-filter'
import { ErrorState } from '@/components/error-state'
import { Pagination } from '@/components/pagination'
import { ProductCard } from '@/components/product-card'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { hrefCurrent, hrefWith, toSearchParams } from '@/lib/search-params'

type ProductList = components['schemas']['ProductList']
type Category = components['schemas']['Category']

const BASE_PATH = '/danh-muc'

/**
 * Trang danh mục render theo từng yêu cầu, KHÔNG dựng sẵn.
 *
 * Vì sao không ISR như README mục 7.2 gợi ý cho trang danh mục: toàn bộ bộ lọc
 * nằm trên query string, nên mỗi tổ hợp danh mục × thương hiệu × khoảng giá ×
 * thuộc tính × sắp xếp × số trang là một biến thể riêng. Số biến thể là TÍCH
 * của các chiều đó — dựng sẵn hết là bất khả thi, mà dựng sẵn vài cái rồi để
 * phần còn lại rơi vào nhánh khác thì hành vi của trang lại phụ thuộc vào việc
 * khách bấm trúng tổ hợp nào, một thứ không ai kiểm chứng nổi.
 *
 * Cache đúng chỗ của nó nằm ở tầng khác: Redis cache-aside trong backend (P0.2)
 * đã đỡ phần truy vấn nặng, và CDN phía trước có thể cache theo URL đầy đủ.
 * Frontend ở đây chỉ lo dựng HTML.
 *
 * Lưu ý đi kèm: `force-dynamic` ép MỌI `fetch` trong trang này thành
 * `no-store`, kể cả lời gọi lấy cây danh mục bên dưới. Nên đừng truyền
 * `revalidate` cho `apiGet` ở đây rồi tưởng nó có tác dụng — nó không có.
 */
export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'Danh mục sản phẩm',
  description:
    'Danh sách laptop, PC và linh kiện máy tính chính hãng. Lọc theo danh mục, khoảng giá và sắp xếp theo giá hoặc hàng mới về.',
}

/** Các tham số được phép chuyển tiếp xuống backend, theo đúng api/openapi.yaml. */
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
 * Dựng query string gửi xuống backend từ tham số trên URL.
 *
 * Danh sách trắng chứ không chuyển tiếp tất cả: URL là thứ người lạ tự gõ được,
 * và chuyển tiếp mù mọi tham số nghĩa là bất kỳ ai cũng gắn thêm được tham số
 * nội bộ vào lời gọi API của chúng ta.
 */
function buildApiQuery(params: URLSearchParams): string {
  const query = new URLSearchParams()

  for (const key of FORWARDED_PARAMS) {
    const value = params.get(key)
    if (value !== null && value !== '') query.set(key, value)
  }

  /*
    `attr.<tên>` phải TỰ NỐI bằng tay, không đi qua type đã sinh được.

    Lý do: thuộc tính sản phẩm do người bán tự đặt (cpu, ram, kích thước màn
    hình...) nên api/openapi.yaml không liệt kê hết được. openapi-typescript
    sinh ra object `query` ĐÓNG, không có index signature, vì vậy
    `query['attr.ram']` là lỗi biên dịch dù backend hiểu rất rõ tham số đó.
    Chính file schema.ts được sinh ra cũng ghi đúng chú thích này ở đầu.

    Vẫn phải lọc: chỉ nhận khóa còn phần tên sau dấu chấm, để không gửi xuống
    một `attr.=` trống nghĩa.
  */
  for (const [key, value] of params.entries()) {
    if (!key.startsWith(ATTR_PREFIX)) continue
    if (key.length <= ATTR_PREFIX.length || value === '') continue
    query.set(key, value)
  }

  return query.toString()
}

export default async function Page({ searchParams }: PageProps<'/danh-muc'>) {
  // `searchParams` là Promise từ Next.js 15 trở đi và BẮT BUỘC phải await.
  // Quên await thì không có lỗi biên dịch rõ ràng nào cả, chỉ là mọi thuộc tính
  // đọc ra đều undefined — tức là trang lặng lẽ bỏ qua toàn bộ bộ lọc. Mọi ví
  // dụ cũ trên mạng đều viết theo kiểu đồng bộ, đừng chép theo.
  const sp = await searchParams
  const params = toSearchParams(sp)

  const apiQuery = buildApiQuery(params)
  const activeCategory = params.get('category') ?? undefined
  const activeSort = params.get('sort') ?? DEFAULT_SORT

  /*
    Gọi SONG SONG bằng allSettled, không phải hai lệnh await nối tiếp: hai lời
    gọi không phụ thuộc nhau, xếp hàng chỉ làm trang chậm gấp đôi một cách vô
    ích.

    Và quan trọng hơn — `allSettled` chứ không phải `all`: với `all`, cây danh
    mục hỏng sẽ kéo sập luôn danh sách sản phẩm dù danh sách đó lấy về hoàn toàn
    bình thường. Bộ lọc là thứ phụ; mất nó thì trang vẫn bán được hàng.
  */
  const [listResult, treeResult] = await Promise.allSettled([
    apiGet<ProductList>(apiQuery === '' ? '/products' : `/products?${apiQuery}`),
    apiGet<{ data: Category[] }>('/categories'),
  ])

  /*
    ======================================================================
    LỖI PHẢI ĐƯỢC BẮT NGAY TẠI ĐÂY, không được để bay lên app/error.tsx.
    ======================================================================
    Đã ĐO thực tế ở Task 3: React tước sạch thuộc tính tùy biến của lỗi ném ra
    từ Server Component trước khi giao cho error boundary, ở CẢ dev lẫn
    production:

        error.name                          → 'ApiError' (dev), 'Error' (prod)
        error.code / .status / .requestId    → undefined ở CẢ HAI

    Nghĩa là nếu để `ApiError` bay lên `error.tsx`, khách sẽ nhận đúng một câu
    chung chung "Đã có lỗi xảy ra" cho mọi thứ — xem quá sâu, danh mục không tồn
    tại, mất kết nối tới backend — và mất luôn `request_id` để tổng đài tra log.

    Ở đây thì khác: đoạn code này vẫn đang chạy trên server, `ApiError` còn
    nguyên vẹn, nên tra được đúng thông điệp tiếng Việt cho từng mã. Đây là chỗ
    DUY NHẤT còn giữ được `code` và `requestId`.
  */
  if (listResult.status === 'rejected') {
    const reason: unknown = listResult.reason
    // Không phải ApiError thì đó là lỗi lập trình thật sự (backend trả rác, lỗi
    // render) — ném tiếp cho error.tsx, vì ta không có gì tử tế để nói với khách.
    if (!(reason instanceof ApiError)) throw reason

    return (
      <div>
        <Breadcrumb items={[{ label: 'Trang chủ', href: '/' }, { label: 'Danh mục sản phẩm' }]} />
        <h1 className="mt-3 mb-8 text-2xl font-semibold text-gray-900">Danh mục sản phẩm</h1>
        <ErrorState code={reason.code} requestId={reason.requestId}>
          <EscapeActions params={params} />
        </ErrorState>
      </div>
    )
  }

  const list = listResult.value
  // Cây danh mục hỏng thì bỏ hẳn cột bộ lọc chứ không dựng thêm một khối lỗi
  // thứ hai: hai thông báo lỗi trên cùng một trang chỉ làm khách hoang mang,
  // trong khi danh sách sản phẩm bên phải vẫn đang hiển thị bình thường.
  const tree = treeResult.status === 'fulfilled' ? treeResult.value.data : []
  const currentCategory = activeCategory ? findCategory(tree, activeCategory) : undefined

  return (
    <div>
      <Breadcrumb
        items={[
          { label: 'Trang chủ', href: '/' },
          ...(currentCategory
            ? [{ label: 'Danh mục sản phẩm', href: BASE_PATH }, { label: currentCategory.name }]
            : [{ label: 'Danh mục sản phẩm' }]),
        ]}
      />

      {/* Đúng một <h1> mỗi trang (README mục 7.4). Không bọc <main>: layout đã có. */}
      <h1 className="mt-3 text-2xl font-semibold text-gray-900">
        {currentCategory?.name ?? 'Tất cả sản phẩm'}
      </h1>
      <p className="mt-1 text-sm text-gray-500">
        {list.meta.total > 0 ? `${list.meta.total} sản phẩm` : 'Không có sản phẩm nào khớp bộ lọc'}
      </p>

      <div className="mt-6 gap-8 lg:flex">
        <aside className="mb-6 shrink-0 lg:mb-0 lg:w-56">
          {tree.length > 0 ? (
            <CategoryFilter
              tree={tree}
              activeSlug={activeCategory}
              basePath={BASE_PATH}
              params={params}
            />
          ) : null}
        </aside>

        {/* min-w-0: không có nó, một tên sản phẩm dài sẽ kéo giãn cột flex và
            đẩy cả lưới tràn ngang ra khỏi màn hình điện thoại. */}
        <div className="min-w-0 flex-1">
          <SortBar activeSort={activeSort} params={params} />

          {list.data.length === 0 ? (
            <EmptyState params={params} hasPrev={list.meta.has_prev} />
          ) : (
            <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {list.data.map((product, index) => (
                <li key={product.id}>
                  {/* Bốn ô đầu là hàng trên cùng ở màn hình rộng — ảnh tính LCP. */}
                  <ProductCard product={product} priority={index < 4} />
                </li>
              ))}
            </ul>
          )}

          <Pagination meta={list.meta} basePath={BASE_PATH} params={params} />
        </div>
      </div>
    </div>
  )
}

/** Dải nút sắp xếp. Cũng là link, cùng lý do với bộ lọc danh mục. */
function SortBar({ activeSort, params }: { activeSort: string; params: URLSearchParams }) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-gray-200 pb-3">
      <span className="text-sm text-gray-500">Sắp xếp:</span>
      {SORT_OPTIONS.map((option) => {
        const active = option.value === activeSort
        return (
          <Link
            key={option.value}
            // Đổi cách sắp xếp thì reset về trang 1 — hrefWith tự làm việc đó.
            // Giữ lại page=5 khi đổi thứ tự là vô nghĩa: trang 5 của một danh
            // sách sắp xếp kiểu khác là một tập sản phẩm hoàn toàn khác.
            href={hrefWith(BASE_PATH, params, { sort: option.value })}
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
 * Không có sản phẩm nào khớp.
 *
 * Lưới trống không phải một trạng thái — nó trông y hệt một trang đang hỏng.
 * Khách cần biết đây là do bộ lọc quá hẹp chứ không phải cửa hàng hết hàng, và
 * cần một nút để thoát ra mà không phải tự sửa URL trên thanh địa chỉ.
 */
function EmptyState({ params, hasPrev }: { params: URLSearchParams; hasPrev: boolean }) {
  return (
    <div className="mt-4 rounded-lg border border-dashed border-gray-300 bg-gray-50 p-10 text-center">
      {/*
        Phân biệt hai tình huống khác hẳn nhau: "bộ lọc không ra kết quả nào" và
        "đi quá số trang thật sự có" (ví dụ sửa tay ?page=200 khi chỉ có 2
        trang). Nói chung một câu cho cả hai sẽ khiến người ở trường hợp sau
        tưởng mình lọc sai, trong khi họ chỉ cần lùi lại một trang.
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
        <EscapeActions params={params} />
      </div>
    </div>
  )
}

/**
 * Các nút đưa khách thoát khỏi một trang không có gì để xem.
 *
 * Chỉ render nút THẬT SỰ dẫn đi đâu đó. Nếu khách đang ở `/danh-muc` trần mà
 * backend chết, thì "Về trang đầu" và "Xóa toàn bộ bộ lọc" đều trỏ về đúng URL
 * họ đang đứng — bấm vào không có gì xảy ra, và khách sẽ kết luận là trang bị
 * treo hẳn. Trường hợp đó cái duy nhất còn ý nghĩa là tải lại trang.
 */
function EscapeActions({ params }: { params: URLSearchParams }) {
  const current = hrefCurrent(BASE_PATH, params)
  const firstPage = hrefWith(BASE_PATH, params, { page: undefined })

  const actions: { href: string; label: string; primary: boolean }[] = []
  if (firstPage !== current) actions.push({ href: firstPage, label: 'Về trang đầu', primary: true })
  if (BASE_PATH !== current && BASE_PATH !== firstPage) {
    actions.push({
      href: BASE_PATH,
      label: 'Xóa toàn bộ bộ lọc',
      primary: actions.length === 0,
    })
  }

  if (actions.length === 0) {
    return (
      // Thẻ <a> trần, KHÔNG phải <Link>: <Link> trỏ về chính route đang hiển
      // thị thì router coi như không có gì thay đổi và có thể không gọi lại
      // server. Ở đây thứ khách cần chính xác là một lượt tải mới — backend vừa
      // chết có thể đã sống lại.
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
