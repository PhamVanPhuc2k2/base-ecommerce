import type { Metadata } from 'next'
import Image from 'next/image'
import Link from 'next/link'
import { notFound } from 'next/navigation'
import { cache } from 'react'
import { Breadcrumb, type Crumb } from '@/components/breadcrumb'
import { ErrorState } from '@/components/error-state'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { formatDate, formatVND } from '@/lib/format'
import { absoluteUrl } from '@/lib/site'

type Product = components['schemas']['Product']
type Category = components['schemas']['Category']

/**
 * ISR 60 giây (README mục 7.2).
 *
 * Trang chi tiết là trang được Google index nhiều nhất và cũng là trang phải
 * hiện giá đúng, nên nó không được vừa tĩnh vĩnh viễn vừa động hoàn toàn:
 * - dựng động mỗi lượt xem → mỗi con bot cào một lượt là một truy vấn database;
 * - dựng tĩnh vĩnh viễn → đổi giá xong khách vẫn thấy giá cũ tới lần build sau.
 *
 * 60 giây là trần thời gian sai lệch chấp nhận được. Đường đi nhanh hơn đã có
 * sẵn: mỗi lời gọi gắn tag `product:<slug>`, nên khi worker nhận event
 * `product.updated` từ RabbitMQ (P1) chỉ cần `revalidateTag('product:<slug>')`
 * là trang được dựng lại ngay, không phải chờ hết 60 giây.
 */
export const revalidate = 60

const PRODUCT_PATH = '/san-pham'
const CATEGORY_PATH = '/danh-muc'

/** Số giây ISR cho cây danh mục — dữ liệu này gần như không đổi. */
const CATEGORY_REVALIDATE = 300

/**
 * Kết quả tải sản phẩm, ở dạng GIÁ TRỊ chứ không phải ngoại lệ.
 *
 * Vì sao không để `loadProduct` ném lỗi như mọi chỗ khác: hàm này được gọi từ
 * `generateMetadata`, và một ngoại lệ ném ra ở đó thành lỗi 500 TRƯỚC KHI thân
 * trang kịp chạy — tức là mọi xử lý `notFound()` / `<ErrorState>` viết công
 * phu bên dưới không bao giờ tới lượt. Trả về giá trị thì cả hai phía cùng
 * quyết định được, và TypeScript bắt buộc phải xử lý đủ ba nhánh.
 */
type Loaded =
  | { kind: 'ok'; product: Product }
  | { kind: 'not-found' }
  | { kind: 'error'; code: string; requestId?: string }

/**
 * ==========================================================================
 * ĐÂY LÀ NƠI DUY NHẤT ĐƯỢC GỌI `/products/{slug}`. Đọc kỹ trước khi thêm một
 * lời gọi thứ hai ở bất kỳ đâu trong file này.
 * ==========================================================================
 * `generateMetadata` và thân trang đều cần đúng một sản phẩm, và Next.js chạy
 * chúng như hai lượt riêng biệt. Nếu mỗi bên tự gọi `apiGet`, backend sẽ nhận
 * HAI request cho MỘT lượt xem trang — nhân đôi tải database trên đúng trang
 * có lưu lượng lớn nhất của cả site.
 *
 * Next.js có khử trùng lặp `fetch` sẵn, NHƯNG chỉ khi URL và tùy chọn giống
 * hệt nhau. Lệch một chữ trong `next.tags` (ví dụ một bên `product:${slug}`,
 * một bên `product-${slug}`) là hai khóa cache khác nhau, và hệ thống lặng lẽ
 * gọi hai lần mà không báo gì cả. Gói lời gọi vào một hàm dùng chung là cách
 * duy nhất khiến "giống hệt nhau" trở thành điều không thể sai.
 *
 * Bọc thêm `cache()` của React là lớp khóa thứ hai, không thừa: nó ghi nhớ
 * theo THAM SỐ HÀM trong phạm vi một request, nên vẫn đúng một lời gọi kể cả
 * khi sau này ai đó đổi `revalidate` hay thêm header và vô tình phá mất điều
 * kiện khử trùng lặp của `fetch`.
 *
 * Đã đo: xem mục kiểm chứng số 5 — một lượt tải trang cho đúng một dòng log
 * `"path":"/api/v1/products/<slug>"` ở backend.
 */
const loadProduct = cache(async (slug: string): Promise<Loaded> => {
  try {
    const product = await apiGet<Product>(`/products/${encodeURIComponent(slug)}`, {
      tags: [`product:${slug}`],
      revalidate,
    })
    return { kind: 'ok', product }
  } catch (e) {
    // Không phải ApiError nghĩa là lỗi lập trình thật sự (backend trả rác, lỗi
    // JSON). Ném tiếp để nó nổ lên error boundary kèm nguyên văn thông tin gỡ
    // lỗi — ta không có gì tử tế để nói với khách về một lỗi mình chưa hiểu.
    if (!(e instanceof ApiError)) throw e
    /*
      PRODUCT_NOT_FOUND phải tách riêng khỏi mọi mã lỗi khác, vì hai bên dẫn
      tới hai MÃ TRẠNG THÁI HTTP khác nhau — và đó là thứ Google đọc, không
      phải chữ trên màn hình:
        - slug không tồn tại  → 404, Google gỡ URL khỏi chỉ mục. Đúng.
        - backend đang chết   → không được trả 404, nếu không Google sẽ gỡ
                                toàn bộ sản phẩm khỏi chỉ mục chỉ vì ta có
                                mười phút hỏng hóc.
    */
    if (e.code === 'PRODUCT_NOT_FOUND') return { kind: 'not-found' }
    return { kind: 'error', code: e.code, requestId: e.requestId }
  }
})

/**
 * Cây danh mục, chỉ để dựng breadcrumb.
 *
 * Hỏng thì trả mảng rỗng chứ không ném: breadcrumb là thứ phụ, mất nó thì
 * trang vẫn bán được hàng. Đây cũng là lý do nó không dùng kiểu `Loaded`.
 */
const loadCategoryTree = cache(async (): Promise<Category[]> => {
  try {
    const body = await apiGet<{ data: Category[] }>('/categories', {
      tags: ['categories'],
      revalidate: CATEGORY_REVALIDATE,
    })
    return body.data
  } catch {
    return []
  }
})

/**
 * Đường đi từ gốc tới danh mục có `id` cho trước, ví dụ
 * [Máy tính, Laptop, Laptop Gaming].
 *
 * Tìm theo ID chứ không theo slug vì `Product` chỉ mang `category_id` — API
 * không trả kèm slug danh mục. Trả về CẢ đường đi chứ không chỉ nút cuối, vì
 * breadcrumb cần đủ các cấp cha; chỉ hiện "Laptop Gaming" thì khách (và
 * Google) không thấy được sản phẩm này nằm ở đâu trong cửa hàng.
 *
 * Cây của một cửa hàng bán lẻ sâu 3–4 cấp, đệ quy là đủ — xem thêm chú thích
 * của `findCategory` trong components/category-filter.tsx.
 */
function categoryPath(tree: Category[], id: string): Category[] {
  for (const node of tree) {
    if (node.id === id) return [node]
    const deeper = categoryPath(node.children, id)
    if (deeper.length > 0) return [node, ...deeper]
  }
  return []
}

/** Giới hạn mô tả cho thẻ meta. Google cắt quanh 155–160 ký tự. */
const META_DESCRIPTION_MAX = 160

function truncate(text: string, max: number): string {
  const clean = text.trim().replace(/\s+/g, ' ')
  if (clean.length <= max) return clean
  // Cắt ở khoảng trắng gần nhất để không chặt giữa một từ, rồi thêm dấu ba
  // chấm thật (…) thay vì ba dấu chấm rời — Google đếm nó là một ký tự.
  const cut = clean.slice(0, max - 1)
  const lastSpace = cut.lastIndexOf(' ')
  return `${(lastSpace > max / 2 ? cut.slice(0, lastSpace) : cut).trimEnd()}…`
}

export async function generateMetadata({
  params,
}: PageProps<'/san-pham/[slug]'>): Promise<Metadata> {
  // `params` là Promise từ Next.js 15 trở đi và BẮT BUỘC await — quên thì
  // không có lỗi biên dịch, chỉ là `slug` thành undefined và mọi trang chi
  // tiết cùng gọi `/products/undefined`.
  const { slug } = await params
  const loaded = await loadProduct(slug)

  if (loaded.kind !== 'ok') {
    /*
      Ở đây chỉ TRẢ VỀ metadata, không ném lỗi và cũng không gọi `notFound()` —
      xem chú thích của kiểu `Loaded`. Quyết định 404 nằm trong thân trang.

      `robots: noindex` cho cả hai nhánh hỏng: một trang đang báo lỗi hoặc sắp
      trả 404 mà vẫn mời Google index thì thứ được lưu vào chỉ mục chính là
      trang lỗi đó. Cũng KHÔNG khai canonical: canonical là lời khẳng định
      "đây là bản chính thức của nội dung này", không được nói ra khi ta còn
      chưa biết nội dung là gì.

      `follow` vẫn bật: link trong header/footer vẫn đáng đi theo.
    */
    return {
      title: loaded.kind === 'not-found' ? 'Không tìm thấy sản phẩm' : 'Không tải được sản phẩm',
      robots: { index: false, follow: true },
    }
  }

  const p = loaded.product
  const description =
    p.short_description.trim() !== ''
      ? truncate(p.short_description, META_DESCRIPTION_MAX)
      : truncate(
          `${p.name} — giá ${formatVND(p.price)}, chính hãng, bảo hành đầy đủ.`,
          META_DESCRIPTION_MAX,
        )

  return {
    // Chỉ phần riêng của trang; tên cửa hàng do `template` trong app/layout.tsx
    // nối vào. Tự nối ở đây sẽ thành "... | Base E-commerce | Base E-commerce".
    title: p.name,
    description,
    alternates: {
      /*
        Canonical trỏ về chính URL này ở dạng TUYỆT ĐỐI.

        Vì sao vẫn cần dù trang chỉ có một địa chỉ: cùng một sản phẩm sẽ được
        chia sẻ kèm đủ loại tham số theo dõi (`?utm_source=...`,
        `?fbclid=...`). Với Google mỗi tham số là một URL khác nhau, và không
        có canonical thì cùng một nội dung bị xé thành hàng chục bản trùng
        lặp, chia nhỏ tín hiệu xếp hạng ra từng mảnh.
      */
      canonical: absoluteUrl(`${PRODUCT_PATH}/${p.slug}`),
    },
    openGraph: {
      title: p.name,
      description,
      // URL ảnh phải tuyệt đối — Facebook/Zalo lấy ảnh từ máy chủ của họ, ở đó
      // đường dẫn tương đối chẳng trỏ tới đâu cả.
      images: p.images,
      url: absoluteUrl(`${PRODUCT_PATH}/${p.slug}`),
      type: 'website',
      siteName: 'Base E-commerce',
      locale: 'vi_VN',
    },
  }
}

export default async function Page({ params }: PageProps<'/san-pham/[slug]'>) {
  const { slug } = await params

  /*
    Gọi song song: cây danh mục không phụ thuộc vào sản phẩm, xếp hàng chỉ làm
    trang chậm thêm một vòng mạng một cách vô ích. `loadProduct` ở đây KHÔNG
    phát sinh request mới — `generateMetadata` đã gọi nó trong cùng request nên
    kết quả lấy từ bộ nhớ đệm của `cache()`.
  */
  const [loaded, tree] = await Promise.all([loadProduct(slug), loadCategoryTree()])

  /*
    ======================================================================
    404 PHẢI LÀ HTTP 404 THẬT — VÀ ĐIỀU ĐÓ PHỤ THUỘC VÀO VIỆC KHÔNG CÓ
    `loading.tsx` NÀO NẰM TRÊN ROUTE NÀY.
    ======================================================================
    Đã đo trên bản production standalone (`node .next/standalone/server.js`):

      app/loading.tsx tồn tại  → /san-pham/<slug-la> trả HTTP **200**
      app/loading.tsx đã dời đi → /san-pham/<slug-la> trả HTTP **404**

    Lý do: `loading.tsx` bọc mọi trang con trong một Suspense boundary, nên
    Next.js gửi ngay phần vỏ (header + skeleton) kèm dòng trạng thái 200 rồi
    mới stream nội dung thật xuống sau. Tới lúc `notFound()` chạy thì mã trạng
    thái đã nằm trên đường truyền và không sửa được nữa. Đã thử gọi
    `notFound()` sớm hơn, ngay trong `generateMetadata` — vẫn 200, vì phần vỏ
    đã được xả trước cả khi đó.

    Vì vậy `loading.tsx` đã được dời từ `app/` xuống `app/danh-muc/` (xem chú
    thích tại chính file đó). ĐỪNG tạo lại `app/loading.tsx`, và cũng đừng
    thêm `app/san-pham/[slug]/loading.tsx`: nó sẽ lặng lẽ biến mọi trang 404
    thành 200.

    Vì sao 200 lại tệ tới vậy, tệ hơn cả 500: trên trình duyệt trông hoàn toàn
    đúng — khách vẫn thấy trang 404 tiếng Việt tử tế — nên không ai phát hiện
    ra. Nhưng Google chỉ đọc mã trạng thái: nó kết luận đây là trang hợp lệ,
    giữ mọi sản phẩm đã ngừng kinh doanh trong chỉ mục vĩnh viễn dưới dạng
    "soft 404", và ngân sách cào bị tiêu vào đó thay vì vào hàng đang bán.

    ĐÁNH ĐỔI đã biết và chấp nhận: khi `notFound()` chạy trong một lượt render
    động, Next.js trả về phần vỏ `<html id="__next_error__">` với <body> rỗng,
    còn nội dung `not-found.tsx` đi kèm trong gói RSC và được dựng ở phía
    client. Tức là trang 404 tiếng Việt vẫn hiện đúng trên trình duyệt, nhưng
    không nằm sẵn trong HTML thô. Chấp nhận được vì mã trạng thái mới là thứ
    quyết định: đã là 404 thì không công cụ tìm kiếm nào đọc tới phần thân nữa.
    Đổi lại mà lấy 200 kèm HTML đầy đủ là một món hời tồi.

    Đây cũng là lý do `PRODUCT_NOT_FOUND` phải tách khỏi mọi mã lỗi khác:
    backend chết thì KHÔNG được trả 404, nếu không Google gỡ sạch sản phẩm
    khỏi chỉ mục chỉ vì ta có mười phút hỏng hóc — nhánh 'error' bên dưới trả
    200 kèm <ErrorState> đúng vì vậy.
  */
  if (loaded.kind === 'not-found') notFound()

  if (loaded.kind === 'error') {
    // Vẫn ở trên server nên `code` và `requestId` còn nguyên vẹn — khách đọc
    // đúng câu tiếng Việt cho từng mã, và tổng đài có mã để tra log.
    return (
      <div>
        <Breadcrumb items={[{ label: 'Trang chủ', href: '/' }, { label: 'Sản phẩm' }]} />
        <h1 className="mt-3 mb-8 text-2xl font-semibold text-gray-900">Sản phẩm</h1>
        <ErrorState code={loaded.code} requestId={loaded.requestId}>
          <Link
            href={CATEGORY_PATH}
            className="rounded-md bg-brand px-5 py-2.5 text-sm font-medium text-white hover:bg-brand-dark"
          >
            Xem danh mục sản phẩm
          </Link>
        </ErrorState>
      </div>
    )
  }

  const product = loaded.product
  const path = categoryPath(tree, product.category_id)
  const crumbs: Crumb[] = [
    { label: 'Trang chủ', href: '/' },
    { label: 'Danh mục sản phẩm', href: CATEGORY_PATH },
    // Mỗi cấp danh mục trỏ về trang danh mục đã lọc sẵn theo slug của nó.
    ...path.map((c): Crumb => ({ label: c.name, href: `${CATEGORY_PATH}?category=${c.slug}` })),
    { label: product.name },
  ]

  const cover = product.images[0]
  const rest = product.images.slice(1)
  const specs = Object.entries(product.attributes)

  return (
    <div>
      <ProductJsonLd product={product} />
      <Breadcrumb items={crumbs} />

      {/* items-start: không có nó, cột ảnh bị kéo cao bằng cột thông tin. */}
      <div className="mt-4 items-start gap-8 lg:flex">
        <div className="lg:w-1/2">
          {/*
            Khung ảnh tỉ lệ CỐ ĐỊNH, nền xám. Đây là điều kiện sống còn với dữ
            liệu thật: ảnh 404, ảnh chưa tải xong hay thiếu hẳn ảnh đều không
            làm khối này co lại, nên phần thông tin bên dưới không nhảy chỗ khi
            ảnh về — thứ Google đo bằng chỉ số CLS. Dữ liệu mẫu của P0.4 dùng
            URL bịa (https://vi.du/anh.jpg) nên KHÔNG tải được; bố cục này phải
            đứng vững đúng trong tình huống đó.
          */}
          <div className="relative aspect-square w-full overflow-hidden rounded-lg border border-gray-200 bg-gray-100">
            {cover === undefined ? (
              <span className="absolute inset-0 flex items-center justify-center text-sm text-gray-400">
                Chưa có ảnh
              </span>
            ) : (
              <Image
                src={cover}
                // alt là tên sản phẩm: vừa là thứ trình đọc màn hình đọc lên,
                // vừa là chữ hiện ra khi ảnh tải hỏng.
                alt={product.name}
                fill
                sizes="(min-width: 1024px) 45vw, 95vw"
                // object-contain: ảnh nhà cung cấp gửi có tỉ lệ lung tung,
                // cover sẽ cắt mất góc máy.
                className="object-contain"
                // Ảnh lớn nhất trên màn hình đầu của trang chi tiết — chính là
                // phần tử Google đo LCP. Đúng MỘT ảnh được `priority`
                // (README mục 7.6); bật cho cả dải ảnh thì chúng tranh băng
                // thông và ảnh quan trọng nhất về chậm hơn.
                priority
              />
            )}
          </div>

          {/*
            Dải ảnh phụ chỉ để XEM, chưa bấm đổi được.

            Bấm để đổi ảnh chính cần state, tức là phải biến khối này thành
            Client Component. Ở P0.4 thứ đáng giá hơn là giữ nguyên trang thuần
            server cho Google và cho tốc độ tải.
            TODO P1: tách thành <ProductGallery> client component, có bấm đổi
            ảnh và phóng to.
          */}
          {rest.length > 0 ? (
            <ul className="mt-3 grid grid-cols-4 gap-3">
              {rest.map((src) => (
                <li
                  key={src}
                  className="relative aspect-square overflow-hidden rounded border border-gray-200 bg-gray-100"
                >
                  <Image
                    src={src}
                    // alt rỗng là CỐ Ý: ảnh phụ không mang thông tin mới so với
                    // ảnh chính đã có alt đầy đủ. Đặt alt lặp lại tên sản phẩm
                    // ở cả bốn ô chỉ khiến trình đọc màn hình đọc đi đọc lại
                    // đúng một câu bốn lần.
                    alt=""
                    fill
                    sizes="(min-width: 1024px) 11vw, 24vw"
                    className="object-contain"
                  />
                </li>
              ))}
            </ul>
          ) : null}
        </div>

        <div className="mt-6 lg:mt-0 lg:w-1/2">
          {/* Đúng một <h1> mỗi trang (README mục 7.4). Không bọc <main>: layout đã có. */}
          <h1 className="text-2xl font-semibold text-gray-900">{product.name}</h1>

          <p className="mt-2 text-sm text-gray-500">
            Mã sản phẩm: <span className="font-mono text-gray-700">{product.sku}</span>
          </p>

          {/*
            formatVND nhận CHUỖI. `price` của API là chuỗi thập phân và phải
            giữ nguyên như vậy cho tới đúng lúc hiển thị — xem chú thích trong
            lib/format.ts về ranh giới của kiểu number trong JS.
          */}
          <p className="mt-4 text-3xl font-bold text-brand">{formatVND(product.price)}</p>
          <p className="mt-1 text-sm text-gray-500">Đã bao gồm VAT</p>

          {product.short_description.trim() !== '' ? (
            <p className="mt-6 text-base text-gray-700">{product.short_description}</p>
          ) : null}

          {specs.length > 0 ? (
            <section className="mt-8">
              {/* h2, không phải h1 thứ hai: tiêu đề phải giảm dần đúng thứ bậc. */}
              <h2 className="text-lg font-semibold text-gray-900">Thông số kỹ thuật</h2>
              {/*
                <table> chứ không phải <div> xếp lưới: đây là dữ liệu hai cột
                có quan hệ tên–giá trị thật sự. Trình đọc màn hình sẽ đọc
                "RAM: 32GB DDR5" nhờ <th scope="row">, còn với một mớ div thì
                nó chỉ đọc được hai chuỗi rời nhau.
              */}
              <table className="mt-3 w-full border-collapse text-sm">
                <tbody>
                  {specs.map(([key, value]) => (
                    <tr key={key} className="border-b border-gray-200 last:border-0">
                      {/*
                        Tên thuộc tính do người bán tự đặt nên hiện NGUYÊN VĂN,
                        không tự viết hoa hay dịch: bảng tra cứng sẽ bỏ sót mọi
                        khóa mới, và một khóa tiếng Việt như "ổ cứng" mà bị ép
                        qua bảng tiếng Anh sẽ biến mất khỏi bảng thông số.
                      */}
                      <th
                        scope="row"
                        className="w-2/5 py-2.5 pr-4 text-left font-medium text-gray-500"
                      >
                        {key}
                      </th>
                      <td className="py-2.5 text-gray-900">{value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          ) : null}

          {/*
            Ngày cập nhật hiện ở cuối, chữ nhỏ: với hàng công nghệ, khách có
            nhìn xem thông tin còn mới không trước khi tin vào giá.
            formatDate đổi sang giờ Việt Nam — container thường chạy TZ=UTC.
          */}
          <p className="mt-8 text-xs text-gray-400">
            Cập nhật lần cuối: {formatDate(product.updated_at)}
          </p>
        </div>
      </div>
    </div>
  )
}

/**
 * Dữ liệu có cấu trúc schema.org cho Google (README mục 7.4).
 *
 * Đây là thứ quyết định sản phẩm có được hiện kèm GIÁ và tình trạng còn hàng
 * ngay trên trang kết quả tìm kiếm hay không — khác biệt lớn nhất về tỉ lệ
 * bấm vào mà một trang thương mại điện tử có thể tạo ra bằng HTML.
 *
 * TODO P1: bổ sung `BreadcrumbList` và `AggregateRating` khi có dữ liệu đánh
 * giá thật. Tuyệt đối không bịa `aggregateRating` — Google phạt nặng dữ liệu
 * có cấu trúc không khớp với nội dung nhìn thấy trên trang.
 */
function ProductJsonLd({ product }: { product: Product }) {
  const url = absoluteUrl(`${PRODUCT_PATH}/${product.slug}`)

  const data = {
    '@context': 'https://schema.org',
    '@type': 'Product',
    name: product.name,
    sku: product.sku,
    description: product.short_description,
    image: product.images,
    offers: {
      '@type': 'Offer',
      /*
        ====================================================================
        `price` PHẢI là chuỗi số thuần: "25990000". KHÔNG được formatVND.
        ====================================================================
        Đây là chỗ dễ nhầm nhất của cả trang, vì mọi chỗ khác đều đi qua
        `formatVND`. Nhưng "25.990.000 ₫" thì Google từ chối thẳng: dấu chấm
        phân cách nghìn và ký hiệu tiền tệ khiến trường này không parse được,
        và cả khối dữ liệu bị loại — mất luôn giá hiển thị trên trang kết quả
        tìm kiếm. Ký hiệu tiền tệ đã có chỗ riêng của nó là `priceCurrency`.

        `product.price` từ API vốn đã đúng dạng đó (chuỗi thập phân, không
        định dạng hiển thị — xem api/openapi.yaml), nên chỉ việc truyền thẳng.
        Đừng "dọn dẹp" dòng này cho giống các dòng khác trong file.
      */
      price: product.price,
      priceCurrency: product.currency,
      /*
        P0 chưa có tồn kho: mọi sản phẩm `live` đều coi là còn hàng. Nói dối
        Google chỗ này rất đắt — khách bấm vào từ kết quả tìm kiếm rồi thấy
        hết hàng sẽ thoát ngay, và Google ghi nhận điều đó.
        TODO P1: lấy `availability` từ tồn kho thật.
      */
      availability: 'https://schema.org/InStock',
      url,
    },
  }

  return (
    /*
      ======================================================================
      VÌ SAO PHẢI THAY `<` BẰNG DẠNG THOÁT `<` — đừng bỏ dòng replace.
      (dấu gạch chéo ngược + u003c, viết đúng như vậy trong chuỗi JSON)
      ======================================================================
      Nội dung bên trong <script> KHÔNG được trình duyệt giải mã thực thể HTML;
      trình phân tích cú pháp chỉ dò đúng chuỗi `</script`. Nghĩa là nếu tên
      hay mô tả sản phẩm chứa `</script>` — người bán dán nhầm từ một trang
      khác, hoặc cố tình — thì thẻ script kết thúc ngay tại đó, phần JSON còn
      lại đổ thẳng ra làm chữ hiển thị trên trang, và mọi thứ viết sau nó chạy
      như mã thật. Đó là lỗ hổng XSS lưu trữ kinh điển.

      `JSON.stringify` KHÔNG bảo vệ được: với nó `<` là một ký tự hợp lệ, không
      có gì phải thoát. Dạng thoát `<` thì ngược lại — JSON.parse đọc lại
      thành đúng ký tự `<`, còn trình phân tích HTML không bao giờ nhìn thấy
      chuỗi `</script`. Dữ liệu tới Google vẫn y nguyên, chỉ khác cách viết.

      Đã kiểm bằng một sản phẩm thử có `</script><script>alert(...)</script>`
      trong mô tả — xem mục kiểm chứng số 3.
    */
    <script
      type="application/ld+json"
      // biome-ignore lint/security/noDangerouslySetInnerHtml: JSON-LD bắt buộc nằm nguyên văn trong <script>; rủi ro duy nhất là chuỗi `</script` và đã xử lý ở dòng dưới.
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data).replaceAll('<', '\\u003c') }}
    />
  )
}
