import type { Metadata } from 'next'

/**
 * Nhóm trang xác thực: đăng nhập, đăng ký, quên / đặt lại mật khẩu.
 *
 * noindex chứ không Disallow trong robots.txt: Disallow thì bot không được đọc
 * trang nên cũng không bao giờ thấy thẻ noindex — URL vẫn có thể lên kết quả
 * tìm kiếm nếu có link trỏ tới. noindex mới thật sự giữ chúng ngoài Google.
 */
export const metadata: Metadata = {
  robots: { index: false, follow: false },
}

export default function AuthLayout({ children }: LayoutProps<'/'>) {
  return <div className="mx-auto w-full max-w-md py-6">{children}</div>
}
