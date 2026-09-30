import type { Metadata } from 'next'
import Link from 'next/link'
import { ResetForm } from './reset-form'

export const metadata: Metadata = { title: 'Đặt lại mật khẩu' }

export default async function Page({ searchParams }: PageProps<'/dat-lai-mat-khau'>) {
  const { email } = await searchParams
  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Đặt lại mật khẩu</h1>
      {/*
        Câu này đúng cho MỌI email — API không cho biết email có tài khoản hay
        không (P2.3 mục 2.4), và trang này cũng không được nói khác đi.
      */}
      <p className="mt-2 text-sm text-gray-600">
        Nếu email có tài khoản, mã 6 số đang trên đường tới hộp thư của bạn (có hiệu lực 10 phút).
        Không thấy thư? Kiểm tra thư mục spam, hoặc{' '}
        <Link href="/quen-mat-khau" className="text-brand hover:text-brand-dark">
          gửi lại sau 1 phút
        </Link>
        .
      </p>
      <ResetForm email={typeof email === 'string' ? email : ''} />
    </div>
  )
}
