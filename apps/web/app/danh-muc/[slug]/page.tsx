import type { Metadata } from 'next'
import { notFound } from 'next/navigation'
import { cache } from 'react'
import { ProductListing } from '@/components/product-listing'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { categoryHref, categoryPathBySlug } from '@/lib/categories'
import { clearAttrs, hrefWith, toSearchParams } from '@/lib/search-params'
import { absoluteUrl } from '@/lib/site'

type Category = components['schemas']['Category']

/** Cùng lý do với `/danh-muc`: bộ lọc trên query string, không dựng sẵn được. */
export const dynamic = 'force-dynamic'

/*
  ⚠️ KHÔNG thêm loading.tsx cho route này (và không đặt loading.tsx ở
  app/danh-muc/): boundary Suspense làm `notFound()` trả HTTP 200 thay vì 404 —
  soft 404, đo được ở P0.4. Xem app/danh-muc/(tat-ca)/loading.tsx.
*/

/**
 * Cây danh mục — hỏng thì `null` (khác mảng rỗng): không có cây thì KHÔNG được
 * kết luận slug không tồn tại. Backend chết mà trả 404 là Google gỡ cả danh mục
 * khỏi chỉ mục vì mười phút sự cố.
 */
const loadTree = cache(async (): Promise<Category[] | null> => {
  try {
    return (await apiGet<{ data: Category[] }>('/categories')).data
  } catch {
    return null
  }
})

export async function generateMetadata({
  params,
}: PageProps<'/danh-muc/[slug]'>): Promise<Metadata> {
  const { slug } = await params
  const tree = await loadTree()
  const path = tree ? categoryPathBySlug(tree, slug) : []
  const current = path.at(-1)
  if (tree !== null && current === undefined) {
    return { title: 'Không tìm thấy danh mục', robots: { index: false, follow: true } }
  }
  const name = current?.name ?? slug
  return {
    title: name,
    description: `${name} chính hãng, giá tốt. Lọc theo thương hiệu, khoảng giá và thông số.`,
    // Canonical KHÔNG kèm query: `?sort=price_asc` hay `?page=3` là cùng một
    // danh mục — Google gộp tín hiệu về một URL thay vì xé ra hàng chục bản.
    alternates: { canonical: absoluteUrl(categoryHref(slug)) },
  }
}

export default async function Page({ params, searchParams }: PageProps<'/danh-muc/[slug]'>) {
  const { slug } = await params
  const query = toSearchParams(await searchParams)
  const tree = await loadTree()
  const path = tree ? categoryPathBySlug(tree, slug) : []
  const current = path.at(-1)

  // 404 THẬT chỉ khi chắc chắn: có cây và slug không nằm trong đó.
  if (tree !== null && current === undefined) notFound()

  const basePath = categoryHref(slug)
  return (
    <ProductListing
      params={query}
      basePath={basePath}
      fixed={{ category: slug }}
      heading={current?.name ?? 'Danh mục sản phẩm'}
      crumbs={[
        { label: 'Trang chủ', href: '/' },
        { label: 'Danh mục sản phẩm', href: '/danh-muc' },
        // Mọi cấp cha đều là link về trang danh mục của nó; cấp cuối (chính
        // trang này) thì không.
        ...path.slice(0, -1).map((c) => ({ label: c.name, href: categoryHref(c.slug) })),
        { label: current?.name ?? slug },
      ]}
      categoryHref={(s) =>
        hrefWith(s === undefined ? '/danh-muc' : categoryHref(s), query, {
          ...clearAttrs(query),
          category: undefined,
        })
      }
    />
  )
}
