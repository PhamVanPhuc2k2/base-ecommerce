import Link from 'next/link'
import { absoluteUrl } from '@/lib/site'

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
      <BreadcrumbJsonLd items={items} />
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

/**
 * JSON-LD `BreadcrumbList` đi KÈM breadcrumb nhìn thấy (P1.5) — một component,
 * hai đầu ra, nên không có trang nào có breadcrumb mà thiếu dữ liệu có cấu trúc.
 * Google dùng nó để hiện "Trang chủ › Laptop › Laptop gaming" thay cho URL trần
 * trên trang kết quả.
 *
 * `item` phải là URL TUYỆT ĐỐI. Chặng cuối (trang hiện tại, không có href) bỏ
 * `item` — schema.org cho phép, và đó đúng là trang Google đang đọc.
 */
function BreadcrumbJsonLd({ items }: { items: Crumb[] }) {
  const data = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: items.map((c, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: c.label,
      ...(c.href !== undefined ? { item: absoluteUrl(c.href) } : {}),
    })),
  }
  return (
    <script
      type="application/ld+json"
      // biome-ignore lint/security/noDangerouslySetInnerHtml: JSON-LD bắt buộc nằm nguyên văn trong <script>; `<` được thoát ở dưới — xem ProductJsonLd.
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data).replaceAll('<', '\\u003c') }}
    />
  )
}
