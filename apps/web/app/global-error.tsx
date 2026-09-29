'use client'

/**
 * Lưới an toàn cho phần mà `app/error.tsx` KHÔNG với tới được.
 *
 * Khác biệt duy nhất nhưng quyết định: `error.tsx` nằm BÊN TRONG `layout.tsx`
 * — React error boundary chỉ bắt được lỗi của cây con bên dưới nó, mà layout
 * lại là cha của boundary đó. Nên nếu chính `layout.tsx` ném lỗi (font tải
 * hỏng, một lời gọi API đặt nhầm vào layout, biến môi trường thiếu), boundary
 * kia không bao giờ được render và khách nhận trang lỗi trần của Next.js bằng
 * tiếng Anh. `global-error.tsx` đứng ngoài layout nên phủ được trường hợp đó.
 *
 * Cái giá: nó THAY THẾ toàn bộ tài liệu, nên phải tự khai <html> và <body>.
 *
 * Và vì nó thay cả tài liệu, KHÔNG thể tin là `globals.css` đã được nạp — Next
 * dựng riêng tài liệu này, không kèm style toàn cục của ứng dụng. Do đó ở đây
 * dùng thuộc tính `style` trực tiếp chứ không dùng lớp Tailwind: một trang lỗi
 * mất sạch CSS còn tệ hơn một trang lỗi xấu nhưng đọc được. Cũng vì lý do đó
 * mà file này cố tình đơn giản, không import thêm component nào — thứ nó phải
 * hiển thị được ngay cả khi phần còn lại của ứng dụng đang hỏng.
 *
 * Lưu ý: không export được `metadata` từ Client Component, nên tiêu đề tab
 * phải đặt bằng thẻ <title> viết thẳng trong JSX.
 */
export default function GlobalError({
  error,
  retry,
}: {
  error: Error & { digest?: string }
  retry: () => void
}) {
  return (
    <html lang="vi">
      <body
        style={{
          margin: 0,
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: '#ffffff',
          color: '#171717',
          fontFamily: 'system-ui, -apple-system, Segoe UI, Roboto, sans-serif',
        }}
      >
        <title>Lỗi hệ thống | Base E-commerce</title>
        <div style={{ maxWidth: '32rem', padding: '2rem', textAlign: 'center' }}>
          <h1 style={{ fontSize: '1.25rem', fontWeight: 600, margin: 0 }}>
            Hệ thống đang gặp sự cố
          </h1>
          <p style={{ marginTop: '0.5rem', color: '#4b5563' }}>
            Rất tiếc, trang không tải được. Vui lòng thử lại sau ít phút.
          </p>
          <button
            type="button"
            onClick={() => retry()}
            style={{
              marginTop: '1.5rem',
              padding: '0.625rem 1.25rem',
              borderRadius: '0.375rem',
              border: 'none',
              backgroundColor: '#d70018',
              color: '#ffffff',
              fontSize: '0.875rem',
              cursor: 'pointer',
            }}
          >
            Thử lại
          </button>
          {error.digest ? (
            <p style={{ marginTop: '2rem', fontSize: '0.75rem', color: '#9ca3af' }}>
              Mã tra cứu: <span style={{ fontFamily: 'monospace' }}>{error.digest}</span>
            </p>
          ) : null}
        </div>
      </body>
    </html>
  )
}
