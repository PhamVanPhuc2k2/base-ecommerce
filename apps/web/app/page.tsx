import Link from 'next/link'

/**
 * Trang chủ tối giản.
 *
 * Vì sao KHÔNG `redirect('/danh-muc')` cho nhanh: `/` là địa chỉ khách gõ tay,
 * là thứ Google gắn với thương hiệu, và là đích của mọi link ngoài trỏ về cửa
 * hàng. Chuyển hướng nó đi chỗ khác nghĩa là trang có thẩm quyền cao nhất của
 * site không còn nội dung riêng, và toàn bộ tín hiệu SEO của tên miền dồn hết
 * vào một trang danh sách có bộ lọc — đúng cái trang KHÔNG nên là bộ mặt.
 *
 * Nội dung thật (khối sản phẩm nổi bật, danh mục nổi bật, khuyến mãi) là việc
 * của P1. Ở P0.4 trang này chỉ cần là một điểm vào tử tế, có đúng một <h1> và
 * link rõ ràng sang trang danh mục.
 *
 * Không bọc <main>: layout.tsx đã có sẵn một cái, xem chú thích tại đó.
 */
export default function Page() {
  return (
    <div className="py-10">
      <h1 className="text-3xl font-bold tracking-tight text-gray-900">
        Máy tính, laptop và linh kiện chính hãng
      </h1>
      <p className="mt-3 max-w-2xl text-base text-gray-600">
        Giá niêm yết rõ ràng, đã bao gồm VAT. Chọn máy theo danh mục, lọc theo khoảng giá và so sánh
        trực tiếp trên cùng một trang.
      </p>
      <Link
        href="/danh-muc"
        className="mt-8 inline-block rounded-md bg-brand px-6 py-3 text-sm font-medium text-white hover:bg-brand-dark"
      >
        Xem danh mục sản phẩm
      </Link>
    </div>
  )
}
