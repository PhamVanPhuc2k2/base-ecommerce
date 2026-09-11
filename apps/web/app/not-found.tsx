import Link from 'next/link'
import { messageFor } from '@/lib/errors'

/**
 * Trang 404 — hiện khi URL không khớp route nào, hoặc khi một trang gọi
 * `notFound()`.
 *
 * Khác `error.tsx`: đây KHÔNG phải sự cố. Không có gì để "thử lại", nên thay
 * vì nút thử lại thì đưa khách quay lại luồng mua hàng. Một trang 404 cụt
 * đường là chỗ khách rời site.
 *
 * Câu chữ lấy từ `messageFor('ROUTE_NOT_FOUND')` thay vì viết thẳng, để trang
 * 404 do Next.js dựng và lỗi 404 do backend trả về nói cùng một câu.
 */
export default function NotFound() {
  return (
    <div className="mx-auto max-w-xl py-16 text-center">
      <p className="text-5xl font-bold text-brand">404</p>
      <h1 className="mt-4 text-xl font-semibold text-gray-900">{messageFor('ROUTE_NOT_FOUND')}</h1>
      <p className="mt-2 text-base text-gray-600">
        Đường dẫn có thể đã thay đổi, hoặc sản phẩm đã ngừng kinh doanh.
      </p>
      <div className="mt-8 flex flex-wrap justify-center gap-3">
        <Link
          href="/danh-muc"
          className="rounded-md bg-brand px-5 py-2.5 text-sm font-medium text-white hover:bg-brand-dark"
        >
          Xem danh mục sản phẩm
        </Link>
        <Link
          href="/"
          className="rounded-md border border-gray-300 px-5 py-2.5 text-sm font-medium text-gray-700 hover:border-gray-400"
        >
          Về trang chủ
        </Link>
      </div>
    </div>
  )
}
