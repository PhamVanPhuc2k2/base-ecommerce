import type { NextRequest } from 'next/server'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { brandHref } from '@/lib/brands'
import { categoryHref, flattenCategories } from '@/lib/categories'
import { absoluteUrl } from '@/lib/site'
import { type UrlEntry, urlset, xmlResponse } from '@/lib/sitemap-xml'

type Category = components['schemas']['Category']
type Brand = components['schemas']['Brand']
type SitemapPage = { data: { slug: string; updated_at: string }[] }

/**
 * Các file con của sitemap index: `pages.xml` và `products-<n>.xml`.
 * Tên file lạ trả 404 — route này không được biến thành chỗ gọi API tùy ý.
 */
export const dynamic = 'force-dynamic'

const PRODUCTS_FILE = /^products-([1-9]\d{0,5})\.xml$/

export async function GET(_req: NextRequest, ctx: RouteContext<'/sitemaps/[file]'>) {
  const { file } = await ctx.params

  if (file === 'pages.xml') return xmlResponse(urlset(await pageEntries()))

  const m = PRODUCTS_FILE.exec(file)
  if (m === null) return new Response('Not found', { status: 404 })

  try {
    const page = await apiGet<SitemapPage>(`/sitemap/products?page=${m[1]}`, {
      revalidate: 3600,
    })
    return xmlResponse(
      urlset(
        page.data.map((p) => ({
          loc: absoluteUrl(`/san-pham/${p.slug}`),
          lastmod: p.updated_at,
        })),
      ),
    )
  } catch (e) {
    /*
      File sản phẩm KHÔNG trả rỗng khi API chết: một file sản phẩm rỗng với mã
      200 nghĩa là "5.000 sản phẩm này không còn" — Google sẽ dần gỡ chúng. 503
      kèm Retry-After nói đúng sự thật: tạm thời hỏng, quay lại sau.
    */
    console.error('[sitemap] không lấy được trang sản phẩm', {
      file,
      cause: e instanceof Error ? e.message : String(e),
    })
    return new Response('Service unavailable', { status: 503, headers: { 'Retry-After': '600' } })
  }
}

/** Trang chủ, danh sách, mọi danh mục, mọi thương hiệu. Phần nào hỏng thì bỏ phần đó. */
async function pageEntries(): Promise<UrlEntry[]> {
  const entries: UrlEntry[] = [
    { loc: absoluteUrl('/') },
    { loc: absoluteUrl('/danh-muc') },
    { loc: absoluteUrl('/thuong-hieu') },
  ]
  const [tree, brands] = await Promise.allSettled([
    apiGet<{ data: Category[] }>('/categories', { revalidate: 3600 }),
    apiGet<{ data: Brand[] }>('/brands', { tags: ['brands'], revalidate: 3600 }),
  ])
  if (tree.status === 'fulfilled') {
    for (const c of flattenCategories(tree.value.data)) {
      entries.push({ loc: absoluteUrl(categoryHref(c.slug)) })
    }
  }
  if (brands.status === 'fulfilled') {
    for (const b of brands.value.data) entries.push({ loc: absoluteUrl(brandHref(b.slug)) })
  }
  return entries
}
