import Link from 'next/link'

// Trang tạm cho P0.4 — chỉ để chứng minh Next.js dựng và chạy được.
// Task 4 (danh mục) và Task 5 (chi tiết sản phẩm) sẽ thay bằng nội dung thật.
//
// Không bọc <main> ở đây: layout.tsx đã có sẵn một cái, xem chú thích tại đó.
export default function Page() {
  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-semibold">Base E-commerce</h1>
      <p className="mt-2 text-sm text-gray-600">
        Storefront đang được xây dựng. Trang danh mục và trang chi tiết sản phẩm sẽ có ở các task
        tiếp theo của P0.4.
      </p>
      <p className="mt-4 text-sm">
        <Link href="/danh-muc" className="text-brand hover:underline">
          Xem danh mục sản phẩm
        </Link>
      </p>
    </div>
  )
}
