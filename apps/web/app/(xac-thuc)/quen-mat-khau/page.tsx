import type { Metadata } from 'next'
import { ForgotForm } from './forgot-form'

export const metadata: Metadata = { title: 'Quên mật khẩu' }

export default function Page() {
  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Quên mật khẩu</h1>
      <p className="mt-2 text-sm text-gray-600">
        Nhập email đã đăng ký. Chúng tôi sẽ gửi một mã 6 số để bạn đặt mật khẩu mới.
      </p>
      <ForgotForm />
    </div>
  )
}
