import Link from 'next/link'
import type { components } from '@/lib/api/generated/schema'

type Category = components['schemas']['Category']

/**
 * Cột lọc theo danh mục, dạng cây.
 *
 * Mỗi mục là một LINK chứ không phải nút giữ state: mở được tab mới, Google bò
 * được vào từng danh mục, và nút Back trả về đúng bộ lọc trước đó mà không cần
 * một dòng JavaScript nào. Nếu làm bằng `useState` thì cả ba thứ trên đều mất,
 * và trang phải trở thành Client Component.
 *
 * Link dựng thế nào do TRANG quyết định qua `hrefFor` (P1.5): trên `/danh-muc`
 * bấm danh mục là sang trang `/danh-muc/<slug>`; trên trang thương hiệu thì ở
 * lại trang đó và lọc bằng `?category=`. `hrefFor(undefined)` là link "Tất cả".
 */
export function CategoryFilter({
  tree,
  activeSlug,
  hrefFor,
}: {
  tree: Category[]
  /** Slug đang được chọn. */
  activeSlug?: string
  hrefFor: (slug: string | undefined) => string
}) {
  return (
    <nav aria-label="Lọc theo danh mục" className="text-sm">
      <h2 className="mb-3 font-semibold text-gray-900">Danh mục</h2>
      <ul className="space-y-1">
        <li>
          <FilterLink href={hrefFor(undefined)} active={activeSlug === undefined}>
            Tất cả sản phẩm
          </FilterLink>
        </li>
        {tree.map((node) => (
          <CategoryNode key={node.id} node={node} activeSlug={activeSlug} hrefFor={hrefFor} />
        ))}
      </ul>
    </nav>
  )
}

/** Một nhánh của cây. Có `children` thì render <ul> lồng bên trong <li> cha. */
function CategoryNode({
  node,
  activeSlug,
  hrefFor,
}: {
  node: Category
  activeSlug?: string
  hrefFor: (slug: string | undefined) => string
}) {
  return (
    <li>
      <FilterLink href={hrefFor(node.slug)} active={node.slug === activeSlug}>
        {node.name}
      </FilterLink>
      {node.children.length > 0 ? (
        // Lồng <ul> trong <li> cha (không phải bên cạnh nó) là cách duy nhất
        // đúng chuẩn HTML để diễn tả danh sách phân cấp; trình đọc màn hình dựa
        // vào đó để báo "mục 2 trong 5, cấp 2".
        <ul className="mt-1 ml-4 space-y-1 border-l border-gray-200 pl-3">
          {node.children.map((child) => (
            <CategoryNode key={child.id} node={child} activeSlug={activeSlug} hrefFor={hrefFor} />
          ))}
        </ul>
      ) : null}
    </li>
  )
}

function FilterLink({
  href,
  active,
  children,
}: {
  href: string
  active: boolean
  children: React.ReactNode
}) {
  return (
    <Link
      href={href}
      // aria-current="true" chứ không chỉ tô màu: người dùng trình đọc màn hình
      // không "thấy" chữ đậm màu đỏ, họ cần được nghe mục nào đang áp dụng.
      aria-current={active ? 'true' : undefined}
      className={
        active
          ? 'block rounded px-2 py-1 font-semibold text-brand'
          : 'block rounded px-2 py-1 text-gray-700 hover:bg-gray-50 hover:text-brand'
      }
    >
      {children}
    </Link>
  )
}
