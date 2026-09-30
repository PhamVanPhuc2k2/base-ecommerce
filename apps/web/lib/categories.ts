import type { components } from '@/lib/api/generated/schema'

type Category = components['schemas']['Category']

/*
  Hiểu biết về CÂY DANH MỤC, dùng chung cho trang danh mục, trang thương hiệu,
  trang chi tiết và sitemap. Cây của một cửa hàng bán lẻ sâu 3–4 cấp, rộng vài
  trăm nút — duyệt đệ quy là đủ, không lo tràn ngăn xếp.
*/

/** Đường dẫn công khai của một danh mục (P1.5 — thay cho `/danh-muc?category=`). */
export function categoryHref(slug: string): string {
  return `/danh-muc/${slug}`
}

/** Tìm danh mục theo slug trong cây lồng nhau. */
export function findCategory(tree: Category[], slug: string): Category | undefined {
  for (const node of tree) {
    if (node.slug === slug) return node
    const found = findCategory(node.children, slug)
    if (found) return found
  }
  return undefined
}

/**
 * Đường đi từ gốc tới danh mục thỏa `match`, ví dụ [Máy tính, Laptop, Laptop
 * gaming]. Trả CẢ đường đi chứ không chỉ nút cuối: breadcrumb cần đủ các cấp
 * cha, chỉ hiện "Laptop gaming" thì khách (và Google) không thấy nó nằm ở đâu.
 */
function pathTo(tree: Category[], match: (c: Category) => boolean): Category[] {
  for (const node of tree) {
    if (match(node)) return [node]
    const deeper = pathTo(node.children, match)
    if (deeper.length > 0) return [node, ...deeper]
  }
  return []
}

/** Theo ID — `Product` chỉ mang `category_id`, không mang slug. */
export function categoryPathById(tree: Category[], id: string): Category[] {
  return pathTo(tree, (c) => c.id === id)
}

export function categoryPathBySlug(tree: Category[], slug: string): Category[] {
  return pathTo(tree, (c) => c.slug === slug)
}

/** Mọi danh mục trong cây, phẳng — cho sitemap. */
export function flattenCategories(tree: Category[]): Category[] {
  return tree.flatMap((c) => [c, ...flattenCategories(c.children)])
}
