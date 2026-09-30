import { createHash } from 'node:crypto'
import type { NextRequest } from 'next/server'
import { NextResponse } from 'next/server'
import { ApiError } from '@/lib/api/error'
import { apiSend } from '@/lib/api/server'
import { ACCESS_COOKIE, REFRESH_COOKIE, type Session, sessionCookies } from '@/lib/auth/cookies'

/**
 * Hai việc, theo đường dẫn:
 *
 * 1. `/danh-muc?category=<slug>` → 308 `/danh-muc/<slug>` (P1.5).
 * 2. `/tai-khoan/*`: bắt đăng nhập và LÀM MỚI token (P2.4).
 */
export async function proxy(req: NextRequest) {
  if (req.nextUrl.pathname === '/danh-muc') return legacyCategoryRedirect(req)
  return requireSession(req)
}

/**
 * ======================================================================
 * VÌ SAO REDIRECT DANH MỤC Ở ĐÂY MÀ KHÔNG `permanentRedirect()` TRONG PAGE
 * ======================================================================
 * Đã thử và ĐO: trang `/danh-muc` nằm dưới `loading.tsx` (skeleton), tức một
 * boundary Suspense. Next xả phần vỏ kèm HTTP **200** trước, rồi mới chạy page
 * — tới lúc `permanentRedirect()` chạy thì mã trạng thái đã đi mất, Next đành
 * chèn `<meta http-equiv="refresh">`. Trình duyệt vẫn chuyển trang, nhưng
 * Google thấy 200 + meta refresh chứ không thấy 308: link cũ KHÔNG được dồn
 * tín hiệu sang URL mới. Cùng họ với cái bẫy soft 404 của P0.4.
 *
 * Proxy chạy TRƯỚC khi render, nên đặt được mã trạng thái thật.
 */
function legacyCategoryRedirect(req: NextRequest) {
  const category = req.nextUrl.searchParams.get('category')
  if (category === null || category === '') return NextResponse.next()

  const url = req.nextUrl.clone()
  url.pathname = `/danh-muc/${encodeURIComponent(category)}`
  url.searchParams.delete('category')
  return NextResponse.redirect(url, 308)
}

/**
 * Trang tài khoản cần access token. Server Component KHÔNG ghi được cookie,
 * nên việc làm mới phải xảy ra ở đây, trước khi render (đặc tả P2.4 mục 2.2):
 *
 * - còn `bec_at` → cho qua;
 * - hết `bec_at`, còn `bec_rt` → refresh, ghi cookie mới lên CẢ response (cho
 *   trình duyệt) LẪN request đang đi tiếp (để trang render bằng token mới
 *   ngay lần này);
 * - không còn gì / refresh hỏng → xóa cookie, 307 về trang đăng nhập.
 */
async function requireSession(req: NextRequest) {
  if (req.cookies.has(ACCESS_COOKIE)) return NextResponse.next()

  const rt = req.cookies.get(REFRESH_COOKIE)?.value
  const session = rt ? await refreshOnce(rt, req.headers.get('x-forwarded-for')) : 'invalid'
  if (typeof session === 'string') {
    const url = req.nextUrl.clone()
    url.pathname = '/dang-nhap'
    url.search = ''
    url.searchParams.set('next', req.nextUrl.pathname + req.nextUrl.search)
    const res = NextResponse.redirect(url, 307)
    // Chỉ xóa khi API NÓI token hỏng. API chết thoáng qua mà xóa là đăng xuất
    // khách vì một sự cố của mình — giữ lại để lần sau refresh tiếp được.
    if (session === 'invalid') res.cookies.delete(REFRESH_COOKIE)
    return res
  }

  const specs = sessionCookies(session)
  for (const c of specs) req.cookies.set(c.name, c.value)
  const res = NextResponse.next({ request: { headers: req.headers } })
  for (const c of specs) res.cookies.set(c.name, c.value, c.options)
  return res
}

/**
 * GỘP các lần refresh cùng một refresh token.
 *
 * Trang và các prefetch của nó (hay hai tab) cùng hết `bec_at` một lúc thì
 * cùng mang MỘT `bec_rt` tới. Gửi cả hai lên API: lần thứ hai trông y hệt kẻ
 * trộm dùng lại token đã đổi, và API thu hồi CẢ phiên (P2.1 mục 2.4) — khách
 * tự dưng bị đăng xuất. Ở đây lần tới sau nhận chung kết quả của lần đầu.
 *
 * Giữ kết quả thêm 10 giây sau khi xong: request tới trễ một nhịp vẫn mang
 * token cũ trong cookie (trình duyệt chưa kịp nhận Set-Cookie).
 *
 * ⚠️ Bộ nhớ của MỘT tiến trình. Chạy nhiều bản web thì hai bản vẫn có thể cùng
 * refresh — khi đó phải gộp qua Redis. Ghi ở bảng giới hạn của TIEN-DO.
 */
type RefreshResult = Session | 'invalid' | 'unavailable'

const inflight = new Map<string, Promise<RefreshResult>>()
const KEEP_MS = 10_000

function refreshOnce(rt: string, forwardedFor: string | null): Promise<RefreshResult> {
  // Khóa là hash, không phải token: token thô không nằm lâu hơn cần thiết
  // trong bộ nhớ dưới dạng khóa của một Map sống suốt tiến trình.
  const key = createHash('sha256').update(rt).digest('base64url')
  const running = inflight.get(key)
  if (running) return running

  const p: Promise<RefreshResult> = apiSend<Session>('/auth/refresh', {
    body: { refresh_token: rt },
    forwardedFor,
  }).catch((e) => (e instanceof ApiError && e.status === 401 ? 'invalid' : 'unavailable'))
  inflight.set(key, p)
  p.finally(() => setTimeout(() => inflight.delete(key), KEEP_MS))
  return p
}

export const config = {
  // CHỈ những đường dẫn cần: proxy không có matcher chạy cho MỌI request, kể
  // cả /_next/static và /img/* — tốn một lượt xử lý cho mỗi file tĩnh.
  matcher: ['/danh-muc', '/tai-khoan', '/tai-khoan/:path*'],
}
