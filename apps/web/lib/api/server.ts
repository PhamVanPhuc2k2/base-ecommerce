import { ApiError } from './error'

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
    })
  } catch (cause) {
    // ĐƯỜNG LỖI 3 — `fetch` NÉM, chưa từng có response nào.
    // Xảy ra khi API không chạy, DNS hỏng, kết nối bị từ chối, đứt mạng. Nếu để
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

  if (!res.ok) {
    // Hai đường lỗi còn lại đều là "có response nhưng không OK". Phải lấy được
    // `code` và `request_id` nếu có, mà tuyệt đối không đánh mất `res.status`.
    let code = 'UNKNOWN'
    let requestId: string | undefined
    try {
      // ĐƯỜNG LỖI 1 — body đúng chuẩn problem+json do backend Go sinh ra.
      const body: unknown = await res.json()
      if (body !== null && typeof body === 'object') {
        const problem = body as { code?: unknown; request_id?: unknown }
        if (typeof problem.code === 'string' && problem.code !== '') code = problem.code
        if (typeof problem.request_id === 'string' && problem.request_id !== '') {
          requestId = problem.request_id
        }
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
    throw new ApiError(code, res.status, requestId)
  }

  // Response OK. Ở đây `res.json()` vẫn có thể ném nếu backend trả rác, nhưng đó
  // là lỗi lập trình phía server chứ không phải trạng thái bình thường cần bọc —
  // để nó nổ lên error boundary kèm nguyên văn thông tin gỡ lỗi.
  return (await res.json()) as T
}
