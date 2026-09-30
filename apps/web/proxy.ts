import type { NextRequest } from 'next/server'
import { NextResponse } from 'next/server'

/**
 * `/danh-muc?category=<slug>` (URL của P0.4–P1.4) → 308 `/danh-muc/<slug>`,
 * giữ nguyên mọi tham số còn lại. Đặc tả P1.5 mục 2.2.
 *
 * ======================================================================
 * VÌ SAO Ở ĐÂY MÀ KHÔNG `permanentRedirect()` TRONG PAGE
 * ======================================================================
 * Đã thử và ĐO: trang `/danh-muc` nằm dưới `loading.tsx` (skeleton), tức một
 * boundary Suspense. Next xả phần vỏ kèm HTTP **200** trước, rồi mới chạy page
 * — tới lúc `permanentRedirect()` chạy thì mã trạng thái đã đi mất, Next đành
 * chèn `<meta http-equiv="refresh">`. Trình duyệt vẫn chuyển trang, nhưng
 * Google thấy 200 + meta refresh chứ không thấy 308: link cũ KHÔNG được dồn
 * tín hiệu sang URL mới. Cùng họ với cái bẫy soft 404 của P0.4.
 *
 * Proxy chạy TRƯỚC khi render, nên đặt được mã trạng thái thật.
 */
export function proxy(req: NextRequest) {
  const category = req.nextUrl.searchParams.get('category')
  if (category === null || category === '') return NextResponse.next()

  const url = req.nextUrl.clone()
  url.pathname = `/danh-muc/${encodeURIComponent(category)}`
  url.searchParams.delete('category')
  return NextResponse.redirect(url, 308)
}

export const config = {
  // CHỈ đúng `/danh-muc`: proxy không có matcher chạy cho MỌI request, kể cả
  // /_next/static và /img/* — tốn một lượt xử lý cho mỗi file tĩnh.
  matcher: '/danh-muc',
}
