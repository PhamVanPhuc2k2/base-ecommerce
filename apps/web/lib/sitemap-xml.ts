/*
  Dựng XML sitemap bằng tay (P1.5) — không dùng `app/sitemap.ts` của Next nữa:
  nó không sinh được sitemap INDEX, và `generateSitemaps` có thể tính số file
  lúc build, khi API chưa chạy. Đặc tả P1.5 mục 2.4.
*/

/**
 * Thoát 5 ký tự đặc biệt của XML. Slug và SITE_URL hiện không chứa chúng,
 * nhưng một `&` lọt vào `<loc>` làm HỎNG cả file — Google bỏ qua toàn bộ
 * sitemap chứ không riêng dòng đó. Rẻ hơn nhiều so với tin rằng dữ liệu sạch.
 */
function esc(s: string): string {
  return s
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}

export type UrlEntry = { loc: string; lastmod?: string }

export function urlset(entries: UrlEntry[]): string {
  const body = entries
    .map(
      (e) =>
        `<url><loc>${esc(e.loc)}</loc>${e.lastmod ? `<lastmod>${esc(e.lastmod)}</lastmod>` : ''}</url>`,
    )
    .join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>\n`
}

export function sitemapIndex(locs: string[]): string {
  const body = locs.map((l) => `<sitemap><loc>${esc(l)}</loc></sitemap>`).join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</sitemapindex>\n`
}

export function xmlResponse(xml: string, init?: ResponseInit): Response {
  return new Response(xml, {
    ...init,
    headers: {
      'Content-Type': 'application/xml; charset=utf-8',
      // Bot đọc sitemap vài lần một ngày; cache một giờ ở CDN là đủ tươi mà
      // không để mỗi lượt đọc đều gọi xuống API.
      'Cache-Control': 'public, max-age=3600',
      ...init?.headers,
    },
  })
}
