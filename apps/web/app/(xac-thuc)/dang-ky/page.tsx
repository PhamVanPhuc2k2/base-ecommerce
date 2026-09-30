import type { Metadata } from 'next'
import Link from 'next/link'
import { RegisterForm } from './register-form'

export const metadata: Metadata = { title: 'Đăng ký' }

export default function Page() {
  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Tạo tài khoản</h1>
      <RegisterForm />
      <p className="mt-6 text-sm text-gray-600">
        Đã có tài khoản?{' '}
        <Link href="/dang-nhap" className="font-medium text-brand hover:text-brand-dark">
          Đăng nhập
        </Link>
      </p>
    </div>
  )
}
