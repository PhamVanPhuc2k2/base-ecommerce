import type { components } from '@/lib/api/generated/schema'

type Attribute = components['schemas']['Attribute']

/** Phần định nghĩa đủ để hiển thị một giá trị. Facet và Attribute đều có. */
type Displayable = Pick<Attribute, 'type' | 'unit'>

/**
 * Giá trị thuộc tính ra chữ cho khách: "16" + đơn vị "GB" → "16 GB",
 * boolean → "Có"/"Không".
 *
 * Giá trị lưu THUẦN (số không kèm đơn vị, boolean là "true"/"false" — đặc tả
 * P1.3 mục 2.4), nên việc ghép đơn vị làm ở đúng một chỗ này. Không có định
 * nghĩa (danh mục ở chế độ tự do) thì hiện nguyên văn.
 */
export function formatAttrValue(def: Displayable | undefined, value: string): string {
  if (def === undefined) return value
  if (def.type === 'boolean') return value === 'true' ? 'Có' : value === 'false' ? 'Không' : value
  return def.unit !== '' ? `${value} ${def.unit}` : value
}

/**
 * Sắp các cặp [mã, giá trị] theo thứ tự quản trị đặt cho danh mục (`order` là
 * danh sách mã theo `position`); mã không có trong `order` xuống cuối, theo
 * bảng chữ cái.
 *
 * Cần thiết vì JSON từ backend Go có khóa sắp theo bảng chữ cái: không sắp lại
 * thì "Màu" luôn đứng trước "RAM" dù quản trị đặt RAM lên đầu.
 */
export function byPosition(entries: [string, string][], order: string[]): [string, string][] {
  const rank = (code: string) => {
    const i = order.indexOf(code)
    return i === -1 ? Number.MAX_SAFE_INTEGER : i
  }
  return [...entries].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
}

/** Bảng tra code → định nghĩa, dựng một lần cho cả trang. */
export function indexAttributes(defs: Attribute[]): Map<string, Attribute> {
  return new Map(defs.map((d) => [d.code, d]))
}
