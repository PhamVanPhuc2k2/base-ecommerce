import type { Metadata } from 'next'
import { notFound } from 'next/navigation'
import { cache } from 'react'
import { ProductListing } from '@/components/product-listing'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { brandHref } from '@/lib/brands'
import { clearAttrs, hrefWith, toSearchParams } from '@/lib/search-params'
import { absoluteUrl } from '@/lib/site'

type Brand = components['schemas']['Brand']

/** Cùng lý do với trang danh mục: bộ lọc trên query string. */
export const dynamic = 'force-dynamic'

/*
  ⚠️ KHÔNG thêm loading.tsx cho route này: boundary Suspense làm `notFound()`
  trả HTTP 200 (soft 404, đo được ở P0.4).
*/

/** Hỏng thì `null` — không có danh sách thì không được kết luận hãng không tồn tại. */
const loadBrands = cache(async (): Promise<Brand[] | null> => {
  try {
    return (await apiGet<{ data: Brand[] }>('/brands', { tags: ['brands'], revalidate: 3600 })).data
  } catch {
    return null
  }
})

export async function generateMetadata({
  params,
}: PageProps<'/thuong-hieu/[slug]'>): Promise<Metadata> {
  const { slug } = await params
  const brands = await loadBrands()
  const brand = brands?.find((b) => b.slug === slug)
  if (brands !== null && brand === undefined) {
    return { title: 'Không tìm thấy thương hiệu', robots: { index: false, follow: true } }
  }
  const name = brand?.name ?? slug
  return {
    title: `Sản phẩm ${name}`,
    description: `Laptop, PC và linh kiện ${name} chính hãng, giá tốt, bảo hành đầy đủ.`,
    alternates: { canonical: absoluteUrl(brandHref(slug)) },
  }
}

export default async function Page({ params, searchParams }: PageProps<'/thuong-hieu/[slug]'>) {
  const { slug } = await params
  const query = toSearchParams(await searchParams)
  const brands = await loadBrands()
  const brand = brands?.find((b) => b.slug === slug)
  if (brands !== null && brand === undefined) notFound()

  const basePath = brandHref(slug)
  const name = brand?.name ?? slug
  return (
    <ProductListing
      params={query}
      basePath={basePath}
      fixed={{ brand: slug }}
      heading={`Sản phẩm ${name}`}
      crumbs={[
        { label: 'Trang chủ', href: '/' },
        { label: 'Thương hiệu', href: '/thuong-hieu' },
        { label: name },
      ]}
      // Trên trang thương hiệu, bấm danh mục là LỌC TRONG hãng này (?category=)
      // chứ không rời sang trang danh mục — khách đang xem "Asus", bấm "Laptop"
      // là muốn laptop Asus.
      categoryHref={(s) => hrefWith(basePath, query, { ...clearAttrs(query), category: s })}
    />
  )
}
