/**
 * Khung xương (skeleton) hiện trong lúc Server Component còn chờ dữ liệu.
 *
 * Vì sao là khối xám chứ không phải chữ "Đang tải...": khung xương giữ đúng
 * chỗ và đúng kích thước của nội dung sắp tới, nên khi dữ liệu về trang không
 * bị nhảy — thứ Google đo bằng chỉ số CLS và tính vào điểm xếp hạng. Dòng chữ
 * "Đang tải..." thì ngược lại: chiếm một dòng, rồi biến mất và đẩy toàn bộ
 * trang dịch chuyển.
 *
 * Dựng theo hình dáng trang danh mục (tiêu đề + lưới sản phẩm) vì đó là trang
 * vào nhiều nhất. Khung xương không cần giống y hệt, chỉ cần chiếm gần đúng
 * chiều cao.
 *
 * ---------------------------------------------------------------------------
 * VÌ SAO FILE NÀY NẰM Ở `app/danh-muc/` CHỨ KHÔNG PHẢI `app/` — đừng dời lên
 * ---------------------------------------------------------------------------
 * Ban đầu nó ở `app/loading.tsx`, tức là bọc MỌI trang con trong một Suspense
 * boundary. Hệ quả đo được ở Task 5, trên bản production standalone:
 *
 *     có app/loading.tsx   → /san-pham/<slug-không-tồn-tại> trả HTTP **200**
 *     không có             → trả đúng HTTP **404**
 *
 * Vì có boundary thì Next.js xả ngay phần vỏ (header + chính khung xương này)
 * kèm dòng trạng thái 200, rồi mới stream nội dung thật. Tới lúc `notFound()`
 * chạy thì mã trạng thái đã đi mất. Với một site sống bằng SEO đó là hỏng
 * nặng và hỏng câm: khách vẫn thấy trang 404 tiếng Việt đúng đắn, còn Google
 * đọc 200 và giữ lại mọi URL sản phẩm đã chết trong chỉ mục ("soft 404").
 *
 * Đặt ở `app/danh-muc/` thì khung xương chỉ áp cho đúng trang nó được vẽ ra để
 * phục vụ — trang danh mục, cũng là trang `force-dynamic` chờ API lâu nhất —
 * còn trang chi tiết sản phẩm giữ được mã trạng thái thật. Trang chi tiết chưa
 * cần khung xương riêng: nó chạy ISR nên hầu hết lượt xem đã có sẵn HTML.
 * Muốn thêm sau này thì phải chấp nhận đánh đổi ở trên — xem chú thích trong
 * app/san-pham/[slug]/page.tsx.
 *
 * aria-hidden + role="status": khung xương là hiệu ứng thị giác thuần túy,
 * đọc lên thành "hộp rỗng, hộp rỗng, hộp rỗng" thì vô nghĩa. Trình đọc màn
 * hình chỉ cần nghe đúng một câu "Đang tải nội dung".
 */
export default function Loading() {
  return (
    <div role="status" aria-live="polite">
      <span className="sr-only">Đang tải nội dung</span>
      <div aria-hidden="true" className="animate-pulse">
        <div className="h-4 w-64 rounded bg-gray-200" />
        <div className="mt-4 h-8 w-96 max-w-full rounded bg-gray-200" />
        <div className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {/* Mảng hằng chỉ để lặp — không có dữ liệu thật nên khóa dùng chuỗi cố định. */}
          {['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'].map((key) => (
            <div key={key} className="rounded-lg border border-gray-200 p-3">
              <div className="aspect-square w-full rounded bg-gray-200" />
              <div className="mt-3 h-4 w-full rounded bg-gray-200" />
              <div className="mt-2 h-4 w-2/3 rounded bg-gray-200" />
              <div className="mt-4 h-5 w-1/2 rounded bg-gray-200" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
