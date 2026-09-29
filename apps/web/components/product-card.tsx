import Image from 'next/image'
import Link from 'next/link'
import type { components } from '@/lib/api/generated/schema'
import { formatVND } from '@/lib/format'

type Product = components['schemas']['Product']

/**
 * Một ô sản phẩm trong lưới danh mục.
 *
 * `priority` chỉ bật cho vài ô ĐẦU TIÊN của lưới (README mục 7.6): đó là ảnh
 * nằm trên màn hình đầu, và ảnh lớn nhất trên màn hình đầu chính là thứ Google
 * đo bằng chỉ số LCP. Bật `priority` cho cả lưới thì phản tác dụng — trình
 * duyệt tải song song 24 ảnh, tranh băng thông với nhau, và ảnh quan trọng
 * nhất về CHẬM hơn là khi không bật gì cả.
 */
export function ProductCard({
  product,
  priority = false,
}: {
  product: Product
  priority?: boolean
}) {
  // noUncheckedIndexedAccess biến images[0] thành `string | undefined`, nên
  // trường hợp mảng rỗng buộc phải xử lý — đúng ý: sản phẩm "draft" chưa kịp
  // gắn ảnh vẫn có thể lọt vào danh sách nội bộ sau này.
  const cover = product.images[0]

  return (
    <article className="group rounded-lg border border-gray-200 bg-white p-3 transition hover:border-brand hover:shadow-sm">
      <Link href={`/san-pham/${product.slug}`} className="block">
        {/*
          Khung ảnh có tỉ lệ CỐ ĐỊNH (aspect-square) và nền xám nhạt. Đây là
          điều kiện sống còn với dữ liệu thật: ảnh hỏng, ảnh 404, ảnh chưa tải
          xong hay ảnh thiếu hẳn đều không làm ô sản phẩm co lại, nên lưới
          không nhảy loạn khi ảnh lần lượt về. Dữ liệu mẫu của P0.4 dùng URL
          bịa (https://vi.du/anh.jpg) nên tải KHÔNG được — bố cục dưới đây phải
          đứng vững đúng trong tình huống đó.
        */}
        <div className="relative aspect-square w-full overflow-hidden rounded bg-gray-100">
          {cover === undefined ? (
            <span className="absolute inset-0 flex items-center justify-center text-xs text-gray-400">
              Chưa có ảnh
            </span>
          ) : (
            <Image
              src={cover}
              // alt lấy tên sản phẩm chứ không để rỗng: với trình đọc màn hình,
              // đây là thứ duy nhất mô tả ô này. Cũng là thứ hiện ra khi ảnh
              // tải hỏng, nên khách vẫn biết ô đó là sản phẩm gì.
              alt={product.name}
              fill
              // sizes bắt buộc khi dùng `fill`, và phải khớp với breakpoint của
              // lưới bên dưới — sai thì Next.js tải ảnh to gấp mấy lần cần
              // thiết trên điện thoại.
              sizes="(min-width: 1024px) 22vw, (min-width: 640px) 30vw, 45vw"
              // object-contain chứ không object-cover: ảnh sản phẩm máy tính do
              // nhà cung cấp gửi có tỉ lệ lung tung, cover sẽ cắt mất góc máy.
              className="object-contain transition group-hover:scale-105"
              priority={priority}
            />
          )}
        </div>

        {/*
          line-clamp-2 + min-h cố định hai dòng: tên sản phẩm dài ngắn khác nhau
          nhưng mọi ô phải cao bằng nhau, nếu không hàng giá sẽ so le.
        */}
        <h3 className="mt-3 line-clamp-2 min-h-10 text-sm font-medium text-gray-900 group-hover:text-brand">
          {product.name}
        </h3>

        {/*
          formatVND nhận CHUỖI. `price` của API là chuỗi thập phân và phải giữ
          nguyên như vậy cho tới đúng lúc hiển thị — xem chú thích trong
          lib/format.ts về giới hạn của kiểu number trong JS.
        */}
        <p className="mt-2 text-base font-semibold text-brand">{formatVND(product.price)}</p>
      </Link>
    </article>
  )
}
