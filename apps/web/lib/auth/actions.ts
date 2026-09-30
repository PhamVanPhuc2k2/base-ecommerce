'use server'

import { revalidatePath } from 'next/cache'
import { redirect } from 'next/navigation'
import { apiSend } from '@/lib/api/server'
import { type Session, safeNext } from './cookies'
import { type FormState, failure, str } from './form-state'
import { authed, clearSession, forwardedFor, refreshToken, saveSession } from './session'

/*
 * Server Action của tài khoản (đặc tả P2.4). Mọi lời gọi API mang theo
 * X-Forwarded-For của khách — xem apiSend.
 *
 * Next tự kiểm Origin khớp Host cho Server Action: đó là lớp chống CSRF,
 * cookie SameSite=Lax chỉ là lớp thứ hai.
 */

export async function login(_: FormState, fd: FormData): Promise<FormState> {
  const email = str(fd, 'email')
  let s: Session
  try {
    s = await apiSend<Session>('/auth/login', {
      body: { email, password: str(fd, 'password') },
      forwardedFor: await forwardedFor(),
    })
  } catch (e) {
    return failure(e, { email })
  }
  await saveSession(s)
  redirect(safeNext(fd.get('next')))
}

export async function register(_: FormState, fd: FormData): Promise<FormState> {
  const values = { email: str(fd, 'email'), full_name: str(fd, 'full_name') }
  let s: Session
  try {
    s = await apiSend<Session>('/auth/register', {
      body: { ...values, password: str(fd, 'password') },
      forwardedFor: await forwardedFor(),
    })
  } catch (e) {
    return failure(e, values)
  }
  await saveSession(s)
  // Đăng ký đã tự gửi mã xác minh (P2.3) — trang tài khoản có ô nhập mã.
  redirect('/tai-khoan')
}

export async function logout(): Promise<void> {
  const rt = await refreshToken()
  if (rt) {
    // Lỗi thu hồi (API chết) không chặn đăng xuất: cookie vẫn bị xóa, và
    // refresh token hết hạn theo lịch của nó.
    await apiSend('/auth/logout', { body: { refresh_token: rt } }).catch(() => undefined)
  }
  await clearSession()
  redirect('/')
}

export async function forgotPassword(_: FormState, fd: FormData): Promise<FormState> {
  const email = str(fd, 'email')
  try {
    await apiSend('/auth/password/forgot', { body: { email }, forwardedFor: await forwardedFor() })
  } catch (e) {
    return failure(e, { email })
  }
  redirect(`/dat-lai-mat-khau?email=${encodeURIComponent(email)}`)
}

export async function resetPassword(_: FormState, fd: FormData): Promise<FormState> {
  const values = { email: str(fd, 'email'), code: str(fd, 'code') }
  try {
    await apiSend('/auth/password/reset', {
      body: { ...values, new_password: str(fd, 'new_password') },
      forwardedFor: await forwardedFor(),
    })
  } catch (e) {
    return failure(e, values)
  }
  // API đã thu hồi MỌI phiên của tài khoản (P2.3 mục 2.5) — cookie đang có
  // (nếu trình duyệt này từng đăng nhập) là rác.
  await clearSession()
  redirect('/dang-nhap?da-doi-mat-khau=1')
}

export async function requestVerification(_: FormState): Promise<FormState> {
  try {
    await authed('/auth/email/verification', { method: 'POST', back: '/tai-khoan' })
  } catch (e) {
    return failure(e)
  }
  return { notice: 'Đã gửi mã mới. Hãy kiểm tra hộp thư của bạn.' }
}

export async function verifyEmail(_: FormState, fd: FormData): Promise<FormState> {
  const code = str(fd, 'code').trim()
  try {
    await authed('/auth/email/verify', { method: 'POST', body: { code }, back: '/tai-khoan' })
  } catch (e) {
    return failure(e, { code })
  }
  revalidatePath('/tai-khoan')
  return { notice: 'Email của bạn đã được xác minh.' }
}

export async function updateProfile(_: FormState, fd: FormData): Promise<FormState> {
  const full_name = str(fd, 'full_name')
  try {
    await authed('/me', { method: 'PATCH', body: { full_name }, back: '/tai-khoan' })
  } catch (e) {
    return failure(e, { full_name })
  }
  revalidatePath('/tai-khoan')
  return { notice: 'Đã lưu họ tên.', values: { full_name } }
}

const ADDRESS_FIELDS = ['recipient_name', 'phone', 'province', 'ward', 'street'] as const

export async function saveAddress(_: FormState, fd: FormData): Promise<FormState> {
  const values = Object.fromEntries(ADDRESS_FIELDS.map((k) => [k, str(fd, k)]))
  const id = str(fd, 'id')
  try {
    await authed(id ? `/me/addresses/${encodeURIComponent(id)}` : '/me/addresses', {
      method: id ? 'PATCH' : 'POST',
      body: values,
      back: '/tai-khoan/dia-chi',
    })
  } catch (e) {
    return failure(e, values)
  }
  revalidatePath('/tai-khoan/dia-chi')
  // Về trang danh sách (bỏ ?sua=) — form trống lại cho lần thêm tiếp theo.
  redirect('/tai-khoan/dia-chi')
}

export async function deleteAddress(fd: FormData): Promise<void> {
  const id = str(fd, 'id')
  await authed(`/me/addresses/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    back: '/tai-khoan/dia-chi',
  })
  revalidatePath('/tai-khoan/dia-chi')
}

export async function setDefaultAddress(fd: FormData): Promise<void> {
  const id = str(fd, 'id')
  await authed(`/me/addresses/${encodeURIComponent(id)}/default`, {
    method: 'POST',
    back: '/tai-khoan/dia-chi',
  })
  revalidatePath('/tai-khoan/dia-chi')
}
