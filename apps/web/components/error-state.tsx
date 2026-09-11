import type { ReactNode } from 'react'
import { messageFor } from '@/lib/errors'

/**
 * Khối báo lỗi hiển thị NGAY TRONG trang, do Server Component tự bắt lỗi rồi
 * render ra.
 *
 * Đây là đường xử lý lỗi CHÍNH của storefront, không phải `app/error.tsx`.
 * Cách dùng:
 *
 *     try {
 *       product = await apiGet<Product>(`/products/${slug}`)
 *     } catch (e) {
 *       if (e instanceof ApiError) return <ErrorState code={e.code} requestId={e.requestId} />
 *       throw e
 *     }
 *
 * Vì sao phải bắt tại chỗ thay vì để lỗi bay lên error.tsx: `error.tsx` là
 * Client Component, và ở bản production Next.js KHÔNG gửi thông điệp lỗi thật
 * của server xuống trình duyệt (chống rò rỉ thông tin nội bộ). Tức là ở đó
 * không còn `code` để tra thông điệp tiếng Việt. Còn ở đây, code vẫn đang chạy
 * trên server nên `ApiError.code` và `request_id` vẫn nguyên vẹn — khách đọc
 * được đúng câu "Không tìm thấy sản phẩm này" thay vì một câu chung chung.
 * Xem thêm phần đầu `app/error.tsx`.
 *
 * `requestId` hiện nhỏ và mờ ở góc dưới: khách gọi lên tổng đài chỉ cần đọc mã
 * này là nhân viên dò được đúng dòng log của lần hỏng đó, nhưng nó không được
 * phép tranh chỗ với thông điệp chính — chuỗi UUID to giữa trang chỉ làm người
 * mua hoang mang.
 */
export function ErrorState({
  code,
  requestId,
  children,
}: {
  code: string
  requestId?: string
  /** Chỗ để trang đặt hành động tiếp theo, ví dụ link về trang danh mục. */
  children?: ReactNode
}) {
  return (
    <div className="mx-auto max-w-xl rounded-lg border border-gray-200 bg-white p-6 text-center">
      <p className="text-base text-gray-900">{messageFor(code)}</p>
      {children ? <div className="mt-4 flex justify-center gap-3">{children}</div> : null}
      {requestId ? (
        <p className="mt-6 text-xs text-gray-400">
          Mã tra cứu: <span className="font-mono">{requestId}</span>
        </p>
      ) : null}
    </div>
  )
}
