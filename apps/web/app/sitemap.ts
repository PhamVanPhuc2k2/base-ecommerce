import type { MetadataRoute } from 'next'
import { connection } from 'next/server'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { absoluteUrl } from '@/lib/site'

type ProductList = components['schemas']['ProductList']

/**
 * Số giây giữ lại kết quả gọi API. Một tiếng: sitemap không cần tươi tới từng
 * phút, và mỗi lần dựng lại là tối đa 200 lượt gọi backend (xem `MAX_PAGES`).
 */
const SITEMAP_REVALIDATE = 3600

/**
 * `limit` lớn nhất API chấp nhận. Gửi lớn hơn thì bị KẸP về 100 chứ không bị từ
 * chối (xem api/openapi.yaml), nên cứ để đúng 100 cho khỏi hiểu nhầm.
 */
const API_MAX_LIMIT = 100

/**
 * Trần số trang, khớp `max_page` của backend.
 *
 * --------------------------------------------------------------------------
 * GIỚI HẠN ĐÃ BIẾT: sitemap này chỉ liệt kê được tối đa 100 × 200 = 20.000 sản
 * phẩm.
 * --------------------------------------------------------------------------
 * Backend chặn cứng ở trang 200 vì phân trang kiểu offset phải quét rồi vứt bỏ
 * `(page-1) * limit` dòng — trang 5.000 đủ sức làm nghẽn database. Vượt trần
 * thì API trả 400 PAGE_TOO_DEEP.
 *
 * Hệ quả với SEO: quá 20.000 sản phẩm thì phần dư KHÔNG có mặt trong sitemap.
 * Chúng vẫn được index nếu có link nội bộ trỏ tới, nhưng chậm hơn nhiều.
 *
 * TODO P1: làm sitemap phân mảnh (`/sitemap/[id].xml` + sitemap index) và cho
 * backend một endpoint duyệt theo con trỏ (keyset trên `created_at, id`) thay
 * vì offset. Chuẩn sitemap cho phép 50.000 URL mỗi file, nên phân mảnh xong là
 * gỡ hẳn được trần này.
 */
const MAX_PAGES = 200

/**
 * Sitemap sinh động từ API.
 *
 * ==========================================================================
 * PHẢI DỰNG LÚC CHẠY, KHÔNG ĐƯỢC DỰNG LÚC BUILD — đó là việc của `connection()`
 * ==========================================================================
 * Mặc định Next.js dựng sẵn `sitemap.xml` ngay trong `next build`. Hai thứ
 * hỏng theo:
 *
 *   1. `SITE_URL` bị đóng băng vào lúc build. Cùng một image đem chạy ở môi
 *      trường khác sẽ phát ra sitemap trỏ về tên miền cũ — xem lib/site.ts.
 *   2. Build BẮT BUỘC gọi được API. Docker build trên máy CI không có backend
 *      nào chạy cả, nên `next build` hoặc đứng hình chờ mạng, hoặc nướng luôn
 *      một sitemap rỗng vào image rồi phục vụ nó suốt vòng đời container.
 *
 * `await connection()` nói với Next.js "hàm này cần một request thật", nên nó
 * bỏ hẳn việc dựng sẵn. Khác `export const dynamic = 'force-dynamic'` ở chỗ
 * quan trọng: `force-dynamic` ép MỌI `fetch` bên trong thành `no-store`, tức
 * là mỗi lượt tải `/sitemap.xml` sẽ bắn thẳng tới 200 lượt gọi API — một con
 * bot cào đủ sức hạ backend. Với `connection()`, `revalidate` của từng lời gọi
 * vẫn có tác dụng, nên các lượt sau đọc từ Data Cache.
 */
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  await connection()

  /*
    Trang tĩnh luôn có mặt, kể cả khi API chết.

    Không gắn `priority` hay `changeFrequency`: Google đã công khai nói họ bỏ
    qua hai trường này từ lâu. Khai chúng chỉ làm file to thêm và tạo cảm giác
    sai rằng ta đang điều khiển được thứ tự cào.
  */
  const staticEntries: MetadataRoute.Sitemap = [
    { url: absoluteUrl('/') },
    { url: absoluteUrl('/danh-muc') },
  ]

  try {
    const products = await fetchAllLiveProducts()
    return [
      ...staticEntries,
      ...products.map((p) => ({
        url: absoluteUrl(`/san-pham/${p.slug}`),
        // `lastModified` là trường DUY NHẤT trong sitemap mà Google thật sự
        // dùng: nó quyết định có quay lại cào một URL cũ hay không. Lấy từ
        // `updated_at` thật của sản phẩm, tuyệt đối không lấy `new Date()` —
        // báo "mọi thứ vừa đổi" ở mỗi lần sinh file thì Google nhanh chóng
        // hiểu ra là ta nói dối và ngừng tin trường này.
        lastModified: p.updated_at,
      })),
    ]
  } catch (e) {
    /*
      ======================================================================
      API CHẾT KHÔNG ĐƯỢC PHÉP LÀM SẬP `/sitemap.xml`.
      ======================================================================
      Để lỗi bay lên thì route này trả HTTP 500. Với Google, một sitemap trả
      500 là tín hiệu xấu kéo dài: nó thử lại thưa dần, và trong lúc đó trang
      mới đăng không được phát hiện. Trả về sitemap tối thiểu thì vẫn đúng
      chuẩn XML, vẫn dẫn bot tới `/danh-muc` — nơi có link tới từng sản phẩm.

      Ghi log là BẮT BUỘC, không được nuốt lặng: một sitemap thiếu sản phẩm
      trông y hệt một sitemap của cửa hàng chưa có hàng, nên không có dòng log
      này thì chẳng ai biết nó đang hỏng cho tới khi traffic tụt.
    */
    console.error('[sitemap] không lấy được danh sách sản phẩm, trả về sitemap tối thiểu', {
      code: e instanceof ApiError ? e.code : 'UNKNOWN',
      status: e instanceof ApiError ? e.status : undefined,
      requestId: e instanceof ApiError ? e.requestId : undefined,
      cause: e instanceof Error ? e.message : String(e),
    })
    return staticEntries
  }
}

/** Chỉ hai trường sitemap cần — giữ nhẹ vì có thể tới hàng chục nghìn phần tử. */
type SitemapProduct = { slug: string; updated_at: string }

/**
 * Duyệt hết danh sách sản phẩm công khai, trang này nối trang kia.
 *
 * Điều kiện dừng là `meta.has_next` của backend, KHÔNG phải `page <
 * total_pages` tự tính: `total_pages` không bị kẹp theo `max_page`, nên tự
 * tính sẽ xin trang 201 và nhận 400 PAGE_TOO_DEEP — xem chú thích dài trong
 * components/pagination.tsx.
 *
 * `MAX_PAGES` là chốt chặn thứ hai, cố tình thừa: nếu một ngày backend có lỗi
 * khiến `has_next` luôn `true`, vòng lặp này sẽ gọi API mãi mãi và treo cả
 * tiến trình Next.js. Một vòng lặp mạng không có trần là một sự cố đang chờ
 * ngày xảy ra.
 */
async function fetchAllLiveProducts(): Promise<SitemapProduct[]> {
  const out: SitemapProduct[] = []

  for (let page = 1; page <= MAX_PAGES; page++) {
    /*
      `sort=newest` cố định: phân trang kiểu offset chỉ đúng khi thứ tự ổn
      định. Để backend tự chọn mặc định thì mai kia mặc định đổi, và sitemap
      lặng lẽ bỏ sót hoặc lặp lại sản phẩm giữa các trang.

      Không truyền bộ lọc trạng thái: endpoint công khai `/products` chỉ trả
      sản phẩm `live` (điều kiện `status = 'live'` nằm trong câu SQL). Sản phẩm
      nháp lọt vào sitemap sẽ thành URL trả 404 ngay trong file ta tự khai với
      Google.
    */
    const list = await apiGet<ProductList>(
      `/products?sort=newest&limit=${API_MAX_LIMIT}&page=${page}`,
      { tags: ['products'], revalidate: SITEMAP_REVALIDATE },
    )

    for (const p of list.data) out.push({ slug: p.slug, updated_at: p.updated_at })

    if (!list.meta.has_next) return out
  }

  // Tới đây nghĩa là đã chạm trần 20.000 sản phẩm nói ở `MAX_PAGES`. Không
  // phải lỗi, nhưng phải nhìn thấy được trong log thì mới biết lúc nào cần làm
  // sitemap phân mảnh.
  console.warn(
    `[sitemap] đã chạm trần ${MAX_PAGES} trang (${out.length} sản phẩm); phần dư không có trong sitemap`,
  )
  return out
}
