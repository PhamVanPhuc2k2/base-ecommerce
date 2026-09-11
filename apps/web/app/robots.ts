import type { MetadataRoute } from 'next'
import { connection } from 'next/server'
import { absoluteUrl } from '@/lib/site'

/**
 * `/robots.txt` — file đầu tiên mọi con bot đọc trước khi cào bất cứ thứ gì.
 *
 * `await connection()` vì đúng lý do như `app/sitemap.ts`: mặc định Next.js
 * dựng sẵn file này lúc `next build`, và khi đó `SITE_URL` bị đóng băng vào
 * image. Hậu quả cụ thể: image build ở staging đem lên production sẽ phát ra
 * dòng `Sitemap: https://staging.../sitemap.xml`, tức là ta tự tay chỉ Google
 * sang tên miền khác. Xem lib/site.ts.
 *
 * Ở đây không có lời gọi API nào nên `export const dynamic = 'force-dynamic'`
 * cũng cho kết quả y hệt; dùng `connection()` để hai file cạnh nhau nói cùng
 * một ngôn ngữ.
 */
export default async function robots(): Promise<MetadataRoute.Robots> {
  await connection()

  return {
    rules: [
      {
        userAgent: '*',
        /*
          Mở toàn bộ phần bán hàng. Với một site sống bằng SEO, mặc định phải
          là CHO PHÉP — mỗi dòng `Disallow` là một phần cửa hàng tự nguyện biến
          mất khỏi Google, nên chỉ thêm khi có lý do rõ ràng.
        */
        allow: '/',
        /*
          `/admin` là khu quản trị (P2). Chặn để nó không bao giờ lọt vào kết
          quả tìm kiếm.

          CHÚ Ý — robots.txt KHÔNG phải cơ chế bảo mật. Nó chỉ là lời đề nghị
          với bot lịch sự; ai gõ thẳng URL vẫn vào được, và bản thân file này
          công khai liệt kê đường dẫn cho người tò mò. Việc chặn thật sự là của
          xác thực phía backend (`X-Admin-Key` hiện tại, JWT + RBAC ở P2). Đừng
          bao giờ liệt kê ở đây một đường dẫn mà bí mật của nó là thứ cần giữ.
        */
        disallow: '/admin',
      },
    ],
    /*
      URL tuyệt đối là BẮT BUỘC theo chuẩn robots.txt — dòng `Sitemap:` chỉ
      chấp nhận URL đầy đủ, đường dẫn tương đối bị bỏ qua không báo lỗi.
      Đây cũng là cách chính để Google tìm ra sitemap khi chưa ai khai báo nó
      trong Search Console.
    */
    sitemap: absoluteUrl('/sitemap.xml'),
  }
}
