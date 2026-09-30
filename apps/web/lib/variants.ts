import type { components } from '@/lib/api/generated/schema'
import { formatAttrValue } from '@/lib/attributes'

type Product = components['schemas']['Product']
type Variant = components['schemas']['Variant']
type Attribute = components['schemas']['Attribute']

/**
 * So hai giá dạng chuỗi thập phân, CHỈ để so lớn nhỏ — không bao giờ để hiển
 * thị (hiển thị luôn đi qua `formatVND` với chuỗi gốc, xem lib/format.ts).
 *
 * Vì sao dùng Number ở đây lại an toàn, trái với cảnh báo chung về giá tiền:
 * cột là NUMERIC(15,2), nên tính theo xu giá lớn nhất là ~10^15 — vẫn nằm
 * trong vùng số nguyên JS biểu diễn chính xác (2^53 ≈ 9·10^15). Cảnh báo ở
 * lib/format.ts là về làm tròn khi HIỂN THỊ và cộng dồn, không phải về so sánh.
 */
function cmp(a: string, b: string): number {
  return Number(a) - Number(b)
}

/** Giá thấp nhất và cao nhất trong các phiên bản API đã trả (công khai: chỉ `active`). */
export function priceRange(p: Product): { low: string; high: string } {
  let low = p.price
  let high = p.price
  for (const v of p.variants) {
    if (cmp(v.price, low) < 0) low = v.price
    if (cmp(v.price, high) > 0) high = v.price
  }
  return { low, high }
}

/**
 * Có nên hiện "Từ …" không: chỉ khi các phiên bản CÓ giá khác nhau. Ba phiên
 * bản cùng giá mà hiện "Từ 20.000.000 ₫" là gợi ý sai rằng còn lựa chọn đắt
 * hơn.
 */
export function hasPriceRange(p: Product): boolean {
  const { low, high } = priceRange(p)
  return cmp(low, high) !== 0
}

/**
 * Nhãn tùy chọn dạng "RAM: 16 GB · Màu: Đen"; phiên bản không tùy chọn trả
 * chuỗi rỗng. Có định nghĩa thì dùng tên + đơn vị, không có (danh mục ở chế độ
 * tự do) thì hiện nguyên mã và giá trị.
 */
export function optionsLabel(v: Variant, defs?: Map<string, Attribute>): string {
  return Object.entries(v.options)
    .map(([k, val]) => {
      const def = defs?.get(k)
      return `${def?.name ?? k}: ${formatAttrValue(def, val)}`
    })
    .join(' · ')
}
