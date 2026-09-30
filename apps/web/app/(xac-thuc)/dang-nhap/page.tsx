import type { Metadata } from 'next'
import Link from 'next/link'
import { safeNext } from '@/lib/auth/cookies'
import { LoginForm } from './login-form'

export const metadata: Metadata = { title: 'Đăng nhập' }

export default async function Page({ searchParams }: PageProps<'/dang-nhap'>) {
  const sp = await searchParams
  const next = safeNext(sp.next)
  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Đăng nhập</h1>
      {sp['da-doi-mat-khau'] ? (
        <p
          role="status"
          className="mt-4 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-800"
        >
          Đã đổi mật khẩu. Mọi thiết bị đã được đăng xuất — hãy đăng nhập bằng mật khẩu mới.
        </p>
      ) : null}
      <LoginForm next={next} />
      <p className="mt-6 text-sm text-gray-600">
        Chưa có tài khoản?{' '}
        <Link href="/dang-ky" className="font-medium text-brand hover:text-brand-dark">
          Đăng ký
        </Link>
      </p>
    </div>
  )
}
