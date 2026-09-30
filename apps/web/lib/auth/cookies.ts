import type { components } from '@/lib/api/generated/schema'
import { siteUrl } from '@/lib/site'

/**
 * Cookie phiên của storefront (đặc tả P2.4 mục 2.1). File này THUẦN — không
 * import `next/headers` — để cả `proxy.ts` lẫn Server Action dùng chung được.
 *
 * Token KHÔNG BAO GIỜ tới tay JavaScript phía trình duyệt: cả hai cookie đều
 * httpOnly, và API (P2.1) chỉ nhận Bearer — Next đọc cookie rồi gắn header.
 * Một lỗ XSS trên storefront vì thế không lấy được token để mang đi nơi khác.
 */

export type Session = components['schemas']['Session']

export const ACCESS_COOKIE = 'bec_at'
export const REFRESH_COOKIE = 'bec_rt'

/**
 * Access cookie chết SỚM hơn token 60 giây: proxy thấy cookie mất là làm mới
 * ngay, thay vì gửi lên API một token còn vài giây rồi bị 401 giữa chừng.
 */
const ACCESS_MARGIN_S = 60

export type CookieSpec = {
  name: string
  value: string
  options: {
    httpOnly: true
    secure: boolean
    sameSite: 'lax'
    path: '/'
    maxAge: number
  }
}

/**
 * Secure theo SITE_URL chứ không theo NODE_ENV: image chạy `next start` (NODE_ENV
 * = production) cả trên máy dev qua http://localhost — cookie Secure trên http
 * thì mọi client không phải trình duyệt đều vứt đi.
 */
function secure(): boolean {
  return siteUrl().startsWith('https://')
}

/**
 * Lax chứ không Strict: khách bấm link trong email (mở từ ứng dụng khác) vẫn
 * phải còn đăng nhập. Server Action vốn đã kiểm Origin khớp Host — lớp chống
 * CSRF không trông vào SameSite.
 */
export function sessionCookies(s: Session): CookieSpec[] {
  const base = { httpOnly: true, secure: secure(), sameSite: 'lax', path: '/' } as const
  const refreshAge = Math.floor((Date.parse(s.refresh_expires_at) - Date.now()) / 1000)
  return [
    {
      name: ACCESS_COOKIE,
      value: s.access_token,
      options: { ...base, maxAge: Math.max(s.expires_in - ACCESS_MARGIN_S, 1) },
    },
    {
      name: REFRESH_COOKIE,
      value: s.refresh_token,
      options: { ...base, maxAge: Math.max(refreshAge, 1) },
    },
  ]
}

/**
 * `next` sau đăng nhập chỉ được là đường dẫn NỘI BỘ. `//evil.com` và
 * `/\\evil.com` trình duyệt hiểu là URL tuyệt đối sang host khác — nhận chúng
 * là biến trang đăng nhập thành bàn đạp lừa đảo (open redirect).
 */
export function safeNext(raw: unknown, fallback = '/tai-khoan'): string {
  if (
    typeof raw !== 'string' ||
    !raw.startsWith('/') ||
    raw.startsWith('//') ||
    raw.startsWith('/\\')
  ) {
    return fallback
  }
  return raw
}
