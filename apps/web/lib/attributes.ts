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

/** Bảng tra code → định nghĩa, dựng một lần cho cả trang. */
export function indexAttributes(defs: Attribute[]): Map<string, Attribute> {
  return new Map(defs.map((d) => [d.code, d]))
}
