import Link from 'next/link'
import type { components } from '@/lib/api/generated/schema'
import { hrefWith } from '@/lib/search-params'

type Category = components['schemas']['Category']

/**
 * Tìm một danh mục theo slug trong cây lồng nhau.
 *
 * Đặt ở đây thay vì trong page vì nó là hiểu biết về CÂY DANH MỤC, không phải
 * về trang: trang chi tiết sản phẩm (Task 5) cũng sẽ cần đúng phép tìm này để
 * dựng breadcrumb.
 *
 * Cây danh mục của một cửa hàng bán lẻ sâu tối đa 3–4 cấp và rộng vài trăm nút,
 * nên duyệt đệ quy là đủ; không cần lo tràn ngăn xếp.
 */
export function findCategory(tree: Category[], slug: string): Category | undefined {
  for (const node of tree) {
    if (node.slug === slug) return node
    const found = findCategory(node.children, slug)
    if (found !== undefined) return found
  }
  return undefined
}

/**
 * Bộ lọc theo danh mục — một cây link, KHÔNG phải form có state.
 *
 * Mỗi mục là một thẻ <a> thật trỏ tới URL đã lọc. Hệ quả: bấm chuột giữa mở
 * được tab mới, Google bò được vào từng danh mục, và nút Back trả về đúng bộ
 * lọc trước đó mà không cần một dòng JavaScript nào. Nếu làm bằng `useState`
 * thì cả ba thứ trên đều mất, và trang phải trở thành Client Component.
 */
export function CategoryFilter({
  tree,
  activeSlug,
  basePath,
  params,
}: {
  tree: Category[]
  /** Slug đang được chọn, lấy từ `?category=`. */
  activeSlug?: string
  /** Đường dẫn của trang đang đứng, ví dụ `/danh-muc`. */
  basePath: string
  /** Toàn bộ tham số hiện tại — để link lọc giữ nguyên sắp xếp, khoảng giá... */
  params: URLSearchParams
}) {
  return (
    <nav aria-label="Lọc theo danh mục" className="text-sm">
      <h2 className="mb-3 font-semibold text-gray-900">Danh mục</h2>
      <ul className="space-y-1">
        <li>
          {/*
            "Tất cả" truyền category: undefined nên hrefWith XÓA hẳn tham số,
            chứ không để lại `category=` rỗng — backend sẽ coi chuỗi rỗng là một
            slug không tồn tại và trả 422 CATEGORY_NOT_FOUND.
          */}
          <FilterLink
            href={hrefWith(basePath, params, { category: undefined })}
            active={activeSlug === undefined}
          >
            Tất cả sản phẩm
          </FilterLink>
        </li>
        {tree.map((node) => (
          <CategoryNode
            key={node.id}
            node={node}
            activeSlug={activeSlug}
            basePath={basePath}
            params={params}
          />
        ))}
      </ul>
    </nav>
  )
}

/** Một nhánh của cây. Có `children` thì render <ul> lồng bên trong <li> cha. */
function CategoryNode({
  node,
  activeSlug,
  basePath,
  params,
}: {
  node: Category
  activeSlug?: string
  basePath: string
  params: URLSearchParams
}) {
  return (
    <li>
      <FilterLink
        href={hrefWith(basePath, params, { category: node.slug })}
        active={node.slug === activeSlug}
      >
        {node.name}
      </FilterLink>
      {node.children.length > 0 ? (
        // Lồng <ul> trong <li> cha (không phải bên cạnh nó) là cách duy nhất
        // đúng chuẩn HTML để diễn tả danh sách phân cấp; trình đọc màn hình dựa
        // vào đó để báo "mục 2 trong 5, cấp 2".
        <ul className="mt-1 ml-4 space-y-1 border-l border-gray-200 pl-3">
          {node.children.map((child) => (
            <CategoryNode
              key={child.id}
              node={child}
              activeSlug={activeSlug}
              basePath={basePath}
              params={params}
            />
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
