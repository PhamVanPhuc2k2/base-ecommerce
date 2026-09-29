import Link from 'next/link'

/** Một chặng trên đường dẫn. Không có `href` nghĩa là chặng hiện tại. */
export type Crumb = {
  label: string
  href?: string
}

/**
 * Đường dẫn phân cấp (breadcrumb) — Trang chủ › Danh mục › Sản phẩm.
 *
 * Mục CUỐI cố ý không bao giờ là link, kể cả khi người gọi truyền `href`: đó là
 * trang khách đang đứng, một link tự trỏ về chính nó chỉ làm người dùng bàn
 * phím tốn thêm một điểm dừng mà chẳng đi tới đâu. `aria-current="page"` là
 * thứ nói cho trình đọc màn hình biết đang ở đâu, chứ không phải cái link.
 *
 * Dấu phân cách để trong <li> riêng và `aria-hidden`: nếu không, trình đọc màn
 * hình sẽ đọc "gạch chéo" giữa mỗi chặng.
 *
 * CHÚ Ý: đây chỉ là phần NHÌN THẤY. JSON-LD `BreadcrumbList` cho Google
 * (README mục 7.4) là việc riêng của từng trang, không sinh ở đây — component
 * này không biết URL tuyệt đối của site nên không tự dựng được dữ liệu đó.
 */
export function Breadcrumb({ items }: { items: Crumb[] }) {
  if (items.length === 0) return null

  return (
    <nav aria-label="Đường dẫn" className="text-sm text-gray-500">
      <ol className="flex flex-wrap items-center gap-x-2 gap-y-1">
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          return (
            // Khóa ghép href với label: hai chặng khác cấp vẫn có thể trùng tên
            // (ví dụ "Laptop" trong "Laptop › Laptop gaming"), còn href thì
            // luôn khác nhau. Không dùng chỉ số mảng làm khóa.
            <li key={`${item.href ?? ''}|${item.label}`} className="flex items-center gap-x-2">
              {index > 0 ? <span aria-hidden="true">›</span> : null}
              {isLast || item.href === undefined ? (
                <span aria-current={isLast ? 'page' : undefined} className="text-gray-900">
                  {item.label}
                </span>
              ) : (
                <Link href={item.href} className="hover:text-brand hover:underline">
                  {item.label}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
