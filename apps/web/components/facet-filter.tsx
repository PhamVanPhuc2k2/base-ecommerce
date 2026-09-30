import Link from 'next/link'
import type { components } from '@/lib/api/generated/schema'
import { formatAttrValue } from '@/lib/attributes'
import { hrefWith } from '@/lib/search-params'

type Facet = components['schemas']['Facet']

/**
 * Bộ lọc theo thuộc tính của danh mục đang xem, kèm số sản phẩm của từng giá
 * trị — lấy từ GET /products/facets.
 *
 * Cùng nguyên tắc với CategoryFilter: mỗi giá trị là một LINK đổi URL
 * (`?attr.<code>=<giá trị>`), không phải checkbox giữ state. Nhờ vậy Back trả
 * về đúng bộ lọc trước, link chia sẻ được, và Google bò được vào trang đã lọc
 * mà trang vẫn là Server Component thuần.
 *
 * Mỗi thuộc tính chọn MỘT giá trị (bấm lại giá trị đang chọn là bỏ chọn): API
 * lọc `attr.<code>` theo đúng một giá trị. Chọn nhiều giá trị cùng nhóm ("8GB
 * hoặc 16GB") cần API hỗ trợ OR — chưa có.
 */
export function FacetFilter({
  facets,
  basePath,
  params,
}: {
  facets: Facet[]
  basePath: string
  params: URLSearchParams
}) {
  const shown = facets.filter((f) => f.values.length > 0)
  if (shown.length === 0) return null

  return (
    <div className="mt-8 space-y-6 text-sm">
      {shown.map((facet) => {
        const key = `attr.${facet.code}`
        const active = params.get(key)
        return (
          <section key={facet.code} aria-labelledby={`facet-${facet.code}`}>
            <h2 id={`facet-${facet.code}`} className="mb-2 font-semibold text-gray-900">
              {facet.name}
            </h2>
            <ul className="space-y-1">
              {facet.values.map((v) => {
                const isActive = active === v.value
                return (
                  <li key={v.value}>
                    <Link
                      // Bấm giá trị đang chọn → bỏ tham số (undefined xóa hẳn
                      // khóa, không để lại `attr.ram=` rỗng). hrefWith tự bỏ
                      // `page`: đổi bộ lọc thì trang 5 của bộ lọc cũ vô nghĩa.
                      href={hrefWith(basePath, params, { [key]: isActive ? undefined : v.value })}
                      aria-current={isActive ? 'true' : undefined}
                      className={
                        isActive
                          ? 'flex justify-between rounded px-2 py-1 font-medium text-brand bg-brand/10'
                          : 'flex justify-between rounded px-2 py-1 text-gray-700 hover:text-brand'
                      }
                    >
                      <span>{formatAttrValue(facet, v.value)}</span>
                      <span className="text-gray-400">{v.count}</span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          </section>
        )
      })}
    </div>
  )
}
