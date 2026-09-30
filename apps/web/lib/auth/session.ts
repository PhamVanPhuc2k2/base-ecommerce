import { cookies, headers } from 'next/headers'
import { redirect } from 'next/navigation'
import { ApiError } from '@/lib/api/error'
import { apiSend } from '@/lib/api/server'
import { ACCESS_COOKIE, REFRESH_COOKIE, type Session, sessionCookies } from './cookies'

/**
 * Phần phiên chỉ chạy ở server (Server Component, Server Action). Việc LÀM
 * MỚI token không nằm ở đây mà ở `proxy.ts` — Server Component không ghi
 * được cookie (đặc tả P2.4 mục 2.2).
 */

/** Chuỗi X-Forwarded-For của request hiện tại — xem `apiSend`. */
export async function forwardedFor(): Promise<string | null> {
  return (await headers()).get('x-forwarded-for')
}

/** Ghi cookie phiên — CHỈ gọi được trong Server Action / Route Handler. */
export async function saveSession(s: Session): Promise<void> {
  const jar = await cookies()
  for (const c of sessionCookies(s)) jar.set(c.name, c.value, c.options)
}

export async function clearSession(): Promise<void> {
  const jar = await cookies()
  jar.delete(ACCESS_COOKIE)
  jar.delete(REFRESH_COOKIE)
}

export async function refreshToken(): Promise<string | undefined> {
  return (await cookies()).get(REFRESH_COOKIE)?.value
}

/**
 * Gọi API thay khách đang đăng nhập. Không có token, hoặc API trả 401 (tài
 * khoản bị khóa, phiên bị thu hồi) → về trang đăng nhập, quay lại đúng chỗ cũ.
 *
 * `redirect()` ném một lỗi đặc biệt của Next — gọi hàm này trong try/catch thì
 * phải ném lại lỗi đó (xem `unstable_rethrow`), đừng nuốt.
 */
export async function authed<T = undefined>(
  path: string,
  opts: { method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'; body?: unknown; back: string },
): Promise<T> {
  const token = (await cookies()).get(ACCESS_COOKIE)?.value
  if (!token) redirect(loginUrl(opts.back))
  try {
    return await apiSend<T>(path, {
      method: opts.method ?? 'GET',
      body: opts.body,
      token,
      forwardedFor: await forwardedFor(),
    })
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) redirect(loginUrl(opts.back))
    throw e
  }
}

export function loginUrl(back: string): string {
  return `/dang-nhap?next=${encodeURIComponent(back)}`
}
