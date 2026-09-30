import type { Metadata } from 'next'
import Link from 'next/link'
import { logout } from '@/lib/auth/actions'

/*
 * Khu tài khoản. `proxy.ts` đã chặn người chưa đăng nhập và làm mới token
 * TRƯỚC khi layout này chạy — ở đây không cần kiểm lại.
 */
export const metadata: Metadata = {
  title: { default: 'Tài khoản', template: '%s | Tài khoản' },
  robots: { index: false, follow: false },
}

export default function AccountLayout({ children }: LayoutProps<'/tai-khoan'>) {
  return (
    <div className="grid gap-8 md:grid-cols-[200px_1fr]">
      <nav aria-label="Tài khoản" className="text-sm">
        <ul className="space-y-1">
          <li>
            <Link
              href="/tai-khoan"
              className="block rounded px-3 py-2 text-gray-800 hover:bg-gray-100"
            >
              Thông tin tài khoản
            </Link>
          </li>
          <li>
            <Link
              href="/tai-khoan/dia-chi"
              className="block rounded px-3 py-2 text-gray-800 hover:bg-gray-100"
            >
              Sổ địa chỉ
            </Link>
          </li>
          <li>
            {/* Form + Server Action, không phải link: đăng xuất bằng GET thì
                một thẻ <img src="/dang-xuat"> trên trang lạ cũng đăng xuất được khách. */}
            <form action={logout}>
              <button
                type="submit"
                className="block w-full rounded px-3 py-2 text-left text-gray-800 hover:bg-gray-100"
              >
                Đăng xuất
              </button>
            </form>
          </li>
        </ul>
      </nav>
      <div>{children}</div>
    </div>
  )
}
