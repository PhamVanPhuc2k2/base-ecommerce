import { ApiError, type FieldError } from './error'

/**
 * URL gốc của backend Go, đọc LÚC CHẠY.
 *
 * Vì sao không dùng `NEXT_PUBLIC_API_URL`: ở P0.4 mọi lời gọi API đều xuất phát
 * từ Server Component, không có dòng nào chạy trong trình duyệt. Nên biến này
 * không cần tiền tố `NEXT_PUBLIC_` — và nhờ vậy tránh hẳn cạm bẫy lớn nhất của
 * `NEXT_PUBLIC_*`: Next.js **thay thẳng giá trị vào bundle lúc `next build`**.
 * Khi đó image build cho staging mang sẵn URL staging trong JS đã đóng gói, nên
 * cũng chính image đó đem lên production sẽ gọi nhầm staging. Muốn đổi URL phải
 * build lại — tức là artifact chạy production KHÔNG còn là artifact đã kiểm thử
 * ở staging nữa. Đọc `process.env` phía server thì một image dùng được mọi môi
 * trường, chỉ đổi biến môi trường lúc khởi động.
 *
 * CHÚ Ý khi chạy trong container: `API_URL` phải là `http://api:8080/api/v1`
 * (tên service trong compose), KHÔNG phải `localhost`. Bên trong container,
 * `localhost` trỏ về chính container đó — tức là chính process Next.js — nên sẽ
 * bị từ chối kết nối chứ không chạm được tới container API.
 * Giá trị mặc định dưới đây chỉ dành cho `task web-dev` chạy trên máy thật.
 */
const API_URL = process.env.API_URL ?? 'http://localhost:8080/api/v1'

/**
 * Trần thời gian cho MỘT lời gọi API.
 *
 * Không có nó thì `fetch` của Node chờ header tới 300 giây (mặc định của
 * undici). Backend CHẾT thì không sao — kết nối bị từ chối ngay, ra lỗi ngay.
 * Backend TREO mới là ca nguy hiểm: mỗi request storefront giữ một kết nối năm
 * phút, request dồn ứ, và khách nhìn trang trắng quay vòng thay vì thông điệp
 * lỗi. Hết giờ thì `fetch` ném, rơi đúng vào ĐƯỜNG LỖI 3 bên dưới → NETWORK_ERROR
 * → <ErrorState> tiếng Việt như mọi hỏng hóc kết nối khác.
 *
 * 10 giây: trang danh mục và chi tiết gọi API bình thường mất vài chục mili-giây,
 * nên chạm trần này nghĩa là backend đã hỏng, không phải "hơi chậm".
 */
const API_TIMEOUT_MS = 10_000

/** Tùy chọn cache của Next.js cho một lời gọi. */
type GetOptions = {
  /** Tag để `revalidateTag()` xóa cache đúng chỗ khi có event từ backend. */
  tags?: string[]
  /** Số giây ISR. 0 nghĩa là luôn gọi mới. */
  revalidate?: number
}

/**
 * Gọi `GET` tới backend và trả về JSON đã ép kiểu `T`.
 *
 * Chỉ dùng được trong Server Component / Route Handler / Server Action.
 *
 * @param path đường dẫn tính từ `API_URL`, ví dụ `/products?page=1`
 * @throws {ApiError} với mọi loại hỏng hóc — xem ba đường lỗi bên dưới
 */
export async function apiGet<T>(path: string, opts?: GetOptions): Promise<T> {
  const url = `${API_URL}${path}`

  let res: Response
  try {
    res = await fetch(url, {
      headers: { Accept: 'application/json' },
      next: { tags: opts?.tags, revalidate: opts?.revalidate },
      signal: AbortSignal.timeout(API_TIMEOUT_MS),
    })
  } catch (cause) {
    // ĐƯỜNG LỖI 3 — `fetch` NÉM, chưa từng có response nào.
    // Xảy ra khi API không chạy, DNS hỏng, kết nối bị từ chối, đứt mạng, hoặc
    // API treo quá API_TIMEOUT_MS (AbortSignal ném TimeoutError). Nếu để
    // lỗi thô của undici bay lên thì trang chỉ thấy `TypeError: fetch failed`,
    // không có `code` để tra thông điệp, và khách nhận trang trắng. Bọc lại
    // thành ApiError để đường xử lý lỗi giống hệt hai trường hợp kia.
    // status = 0 vì thật sự không có mã trạng thái HTTP nào cả — đừng bịa 500.
    const err = new ApiError('NETWORK_ERROR', 0)
    // Giữ nguyên lỗi gốc ở `cause` để log của server còn đọc được "ECONNREFUSED
    // 127.0.0.1:8080". Không đưa vào constructor để chữ ký lớp vẫn đúng hợp
    // đồng ba tham số dùng chung ở mọi nơi.
    err.cause = cause
    throw err
  }

  if (!res.ok) throw await errorFrom(res)

  // Response OK. Ở đây `res.json()` vẫn có thể ném nếu backend trả rác, nhưng đó
  // là lỗi lập trình phía server chứ không phải trạng thái bình thường cần bọc —
  // để nó nổ lên error boundary kèm nguyên văn thông tin gỡ lỗi.
  return (await res.json()) as T
}

/** Tùy chọn của một lời gọi KHÔNG cache (ghi, hoặc đọc dữ liệu riêng của khách). */
type SendOptions = {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
  /** Access token — storefront là BFF, gắn Bearer thay trình duyệt (P2.4). */
  token?: string
  /**
   * Chuỗi X-Forwarded-For của request gốc. Không chuyển tiếp thì API thấy mọi
   * khách cùng một IP (của server Next) và rate limit theo IP gộp tất cả làm
   * một — đặc tả P2.4 mục 2.3.
   */
  forwardedFor?: string | null
}

/**
 * Gọi API KHÔNG qua cache của Next — cho mọi thứ gắn với một khách cụ thể.
 * Trả `undefined` khi response không có body (202, 204).
 *
 * @throws {ApiError} cùng ba đường lỗi như apiGet, kèm `fields` khi 422 theo trường
 */
export async function apiSend<T = undefined>(path: string, opts: SendOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`
  if (opts.forwardedFor) headers['X-Forwarded-For'] = opts.forwardedFor

  let res: Response
  try {
    res = await fetch(`${API_URL}${path}`, {
      method: opts.method ?? 'POST',
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      // no-store: dữ liệu của MỘT khách lọt vào data cache dùng chung là lộ
      // thông tin khách này cho khách khác.
      cache: 'no-store',
      signal: AbortSignal.timeout(API_TIMEOUT_MS),
    })
  } catch (cause) {
    const err = new ApiError('NETWORK_ERROR', 0)
    err.cause = cause
    throw err
  }
  if (!res.ok) throw await errorFrom(res)
  const text = await res.text()
  return (text === '' ? undefined : JSON.parse(text)) as T
}

/**
 * Dựng ApiError từ response không OK. Phải lấy được `code` và `request_id`
 * nếu có, mà tuyệt đối không đánh mất `res.status`.
 */
async function errorFrom(res: Response): Promise<ApiError> {
  let code = 'UNKNOWN'
  let requestId: string | undefined
  let fields: FieldError[] = []
  try {
    // ĐƯỜNG LỖI 1 — body đúng chuẩn problem+json do backend Go sinh ra.
    const body: unknown = await res.json()
    if (body !== null && typeof body === 'object') {
      const problem = body as { code?: unknown; request_id?: unknown; errors?: unknown }
      if (typeof problem.code === 'string' && problem.code !== '') code = problem.code
      if (typeof problem.request_id === 'string' && problem.request_id !== '') {
        requestId = problem.request_id
      }
      if (Array.isArray(problem.errors)) fields = problem.errors as FieldError[]
    }
  } catch {
    // ĐƯỜNG LỖI 2 — body KHÔNG phải JSON.
    // Ví dụ thật: API chết nhưng còn proxy/ingress đứng trước, proxy trả trang
    // HTML "502 Bad Gateway" của chính nó. `res.json()` sẽ ném lỗi parse.
    // Nuốt lỗi parse ở đây là CỐ Ý: cái đáng giữ là `res.status` thật (502),
    // nếu để lỗi parse bay lên thì mã trạng thái duy nhất nói lên chuyện gì
    // đang xảy ra sẽ biến mất, và trang lại nhận một lỗi không có `code`.
    // Không có request_id vì response này không do backend của mình sinh ra.
  }
  return new ApiError(code, res.status, requestId, fields)
}
