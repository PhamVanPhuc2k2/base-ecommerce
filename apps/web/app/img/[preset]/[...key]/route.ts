import type { NextRequest } from 'next/server'

/**
 * Chuyển tiếp ảnh từ imgproxy: `/img/w640/products/<id>.jpg`.
 *
 * Route này tồn tại vì loader của next/image chạy trong trình duyệt và không
 * biết địa chỉ imgproxy (xem lib/image-loader.ts). Ở đây là server: đọc
 * IMGPROXY_URL LÚC CHẠY, nên cùng một image Docker dùng được mọi môi trường.
 *
 * Production nên cho reverse proxy (Caddy) chuyển `/img/*` thẳng tới imgproxy,
 * bỏ một chặng qua Node — route này vẫn đúng, chỉ tốn thêm một lượt chuyển.
 *
 * Chặn TRƯỚC khi gọi đi: preset ngoài danh sách hay khóa sai định dạng trả 404
 * ngay tại đây. imgproxy vốn đã chặn cả hai (chỉ-preset, chỉ đọc bucket), đây
 * là lớp thứ hai — và nó chặn luôn dữ liệu cũ kiểu "https://vi.du/anh.jpg" để
 * không có request nào mang URL ngoài đi xa hơn Next.js.
 */

// PHẢI khớp IMGPROXY_PRESETS trong compose và PRESET_WIDTHS ở lib/image-loader.ts.
const PRESETS = new Set(['w128', 'w256', 'w384', 'w640', 'w828', 'w1080', 'w1920'])

// Khóa do API sinh: products/<uuid>.<đuôi>. Không cho "..", không cho lồng thư
// mục, không cho URL — mọi thứ khác đều không phải ảnh của ta.
const KEY_PATTERN =
  /^products\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$/

const IMGPROXY_URL = process.env.IMGPROXY_URL ?? 'http://localhost:8081'
const SOURCE_PREFIX = process.env.IMGPROXY_SOURCE_PREFIX ?? 's3://catalog/'

export async function GET(req: NextRequest, ctx: RouteContext<'/img/[preset]/[...key]'>) {
  const { preset, key } = await ctx.params
  const objectKey = key.join('/')
  if (!PRESETS.has(preset) || !KEY_PATTERN.test(objectKey)) {
    return new Response('Not found', { status: 404 })
  }

  let upstream: Response
  try {
    upstream = await fetch(
      `${IMGPROXY_URL}/insecure/${preset}/plain/${SOURCE_PREFIX}${objectKey}`,
      {
        // Chuyển nguyên Accept: imgproxy dựa vào nó để chọn AVIF/WebP/JPEG.
        headers: { Accept: req.headers.get('accept') ?? '*/*' },
        // Ảnh không được làm treo trang: imgproxy treo thì trả lỗi sau 10 giây,
        // cùng trần với apiGet.
        signal: AbortSignal.timeout(10_000),
        // Không để Data Cache của Next giữ byte ảnh — trình duyệt/CDN cache theo
        // Cache-Control bên dưới là đủ, còn giữ trong bộ nhớ Node thì phình ra
        // theo số ảnh × số định dạng × số preset.
        cache: 'no-store',
      },
    )
  } catch {
    return new Response('Bad gateway', { status: 502 })
  }
  if (!upstream.ok) {
    // 404 của imgproxy (ảnh đã xóa) giữ nguyên 404; lỗi khác là lỗi của ta.
    return new Response(null, { status: upstream.status === 404 ? 404 : 502 })
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      'Content-Type': upstream.headers.get('content-type') ?? 'application/octet-stream',
      // Khóa là UUID, nội dung dưới một khóa không bao giờ đổi → immutable.
      'Cache-Control': 'public, max-age=31536000, immutable',
      // BẮT BUỘC: cùng URL trả AVIF hay WebP tùy Accept. Thiếu Vary thì CDN
      // cache bản AVIF rồi đưa cho trình duyệt không đọc được AVIF.
      Vary: 'Accept',
    },
  })
}
