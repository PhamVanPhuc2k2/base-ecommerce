'use client'

/**
 * Loader của next/image: khóa media ("products/<id>.jpg") → đường dẫn TƯƠNG
 * ĐỐI tới Route Handler `/img/<preset>/<khóa>` (app/img/[preset]/[...key]).
 *
 * Vì sao tương đối, không trỏ thẳng imgproxy: file này chạy cả trong TRÌNH
 * DUYỆT, nên không đọc được biến môi trường server. Nhúng địa chỉ imgproxy
 * bằng NEXT_PUBLIC_* là khóa nó vào bundle lúc build — một image không dùng
 * lại được cho môi trường khác (cùng lý do P0.4 không dùng NEXT_PUBLIC_API_URL).
 *
 * Chỉ trả tên PRESET, không bao giờ truyền chiều rộng tùy ý: imgproxy chạy chế
 * độ chỉ-preset, tham số lạ bị trả 404 (đặc tả P1.4 mục 3.1). Danh sách này
 * PHẢI khớp IMGPROXY_PRESETS trong deploy/compose.*.yml và PRESETS trong Route
 * Handler; deviceSizes/imageSizes ở next.config.ts dùng đúng các bậc này để
 * srcset không sinh ra hai kích thước trùng preset.
 */
const PRESET_WIDTHS = [128, 256, 384, 640, 828, 1080, 1920] as const

export default function imageLoader({ src, width }: { src: string; width: number }): string {
  // Bậc nhỏ nhất ĐỦ LỚN cho chiều rộng yêu cầu; vượt bậc cuối thì dùng bậc cuối
  // — ảnh gốc hiếm khi rộng hơn 1920 và không cần hơn trên trang bán hàng.
  const w = PRESET_WIDTHS.find((p) => p >= width) ?? 1920
  return `/img/w${w}/${src}`
}
