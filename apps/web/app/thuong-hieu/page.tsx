import type { Metadata } from 'next'
import Link from 'next/link'
import { Breadcrumb } from '@/components/breadcrumb'
import { ErrorState } from '@/components/error-state'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { apiGet } from '@/lib/api/server'
import { brandHref } from '@/lib/brands'

type Brand = components['schemas']['Brand']

/**
 * Render theo yêu cầu, KHÔNG ISR — đã ĐO: với `revalidate = 3600` Next dựng sẵn
 * trang này lúc `next build`, khi đó (trong `docker build`) API không chạy, nên
 * thứ được cache là trang LỖI "Không kết nối được tới máy chủ" — và nó nằm đó
 * suốt một giờ sau MỖI lần deploy. Dữ liệu vẫn được cache ở tầng dưới: Redis
 * trong backend giữ danh sách thương hiệu 6 giờ.
 */
export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'Thương hiệu',
  description: 'Mọi thương hiệu laptop, PC và linh kiện đang bán tại cửa hàng.',
  alternates: { canonical: '/thuong-hieu' },
}

export default async function Page() {
  const crumbs = [{ label: 'Trang chủ', href: '/' }, { label: 'Thương hiệu' }]
  let brands: Brand[]
  try {
    brands = (await apiGet<{ data: Brand[] }>('/brands', { tags: ['brands'] })).data
  } catch (e) {
    // Bắt tại trang, cùng lý do với ProductListing: error.tsx không còn đọc
    // được code/requestId.
    if (!(e instanceof ApiError)) throw e
    return (
      <div>
        <Breadcrumb items={crumbs} />
        <h1 className="mt-3 mb-8 text-2xl font-semibold text-gray-900">Thương hiệu</h1>
        <ErrorState code={e.code} requestId={e.requestId} />
      </div>
    )
  }

  return (
    <div>
      <Breadcrumb items={crumbs} />
      <h1 className="mt-3 text-2xl font-semibold text-gray-900">Thương hiệu</h1>
      <ul className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {brands.map((b) => (
          <li key={b.id}>
            <Link
              href={brandHref(b.slug)}
              className="block rounded-lg border border-gray-200 px-4 py-6 text-center font-medium text-gray-900 hover:border-brand hover:text-brand"
            >
              {b.name}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
