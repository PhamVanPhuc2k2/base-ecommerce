/**
 * Địa chỉ công khai của storefront — gốc của mọi URL tuyệt đối.
 *
 * Dùng cho ba thứ mà Google đọc và KHÔNG chấp nhận đường dẫn tương đối:
 * `<link rel="canonical">`, `sitemap.xml`, và trường `offers.url` của JSON-LD.
 *
 * ---
 * VÌ SAO ĐỌC `process.env.SITE_URL` CHỨ KHÔNG PHẢI `NEXT_PUBLIC_SITE_URL`
 * ---
 * Cùng đúng một lý do đã ghi ở `lib/api/server.ts`: Next.js **thay thẳng giá
 * trị của mọi biến `NEXT_PUBLIC_*` vào bundle lúc `next build`**. Với URL của
 * site thì hậu quả nặng hơn cả URL của API: image build ở staging sẽ mang sẵn
 * `https://staging.../` trong JS, và khi đem chính image đó lên production thì
 * canonical, sitemap, JSON-LD đều trỏ về staging. Google sẽ ngoan ngoãn làm
 * đúng thứ ta bảo nó — gộp mọi tín hiệu xếp hạng về tên miền staging, rồi bỏ
 * qua tên miền thật vì nó tự khai là bản sao. Đây là kiểu hỏng âm thầm: không
 * có lỗi, không có cảnh báo, chỉ là vài tuần sau traffic tự nhiên biến mất.
 *
 * Đọc `process.env` lúc CHẠY thì một image dùng được cho mọi môi trường, chỉ
 * cần đổi biến môi trường lúc khởi động container.
 *
 * Hệ quả kỹ thuật kèm theo: route nào dùng hàm này mà được Next.js dựng sẵn
 * lúc build thì giá trị vẫn bị đóng băng — nên `app/robots.ts` và
 * `app/sitemap.ts` đều phải ép render lúc chạy. Xem chú thích tại hai file đó.
 */
export function siteUrl(): string {
  const raw = (process.env.SITE_URL ?? 'http://localhost:3000').trim()
  // Cắt dấu `/` cuối để `absoluteUrl('/san-pham/x')` không sinh ra `//san-pham`.
  // Người đặt biến môi trường rất hay thêm dấu này, và một URL sai vì thừa một
  // dấu gạch chéo thì canonical trỏ vào hư không.
  return raw.endsWith('/') ? raw.slice(0, -1) : raw
}

/** Ghép một đường dẫn nội bộ (`/san-pham/abc`) thành URL tuyệt đối. */
export function absoluteUrl(path: string): string {
  return `${siteUrl()}${path.startsWith('/') ? path : `/${path}`}`
}
