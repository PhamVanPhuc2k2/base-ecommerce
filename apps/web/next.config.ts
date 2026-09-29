import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  // Cần cho Docker (Task 6): gom đúng những gì runtime cần vào .next/standalone,
  // image từ ~1 GB xuống ~150 MB.
  output: 'standalone',
  images: {
    // Next.js từ chối tối ưu ảnh từ host chưa khai báo, và đó là chủ ý: mở toang
    // thì server của mình thành proxy ảnh miễn phí cho cả internet.
    //
    // TODO P1: siết hostname về đúng CDN thật. '**' chỉ là tạm cho P0.4 vì dữ
    // liệu mẫu dùng URL bịa (https://vi.du/anh.jpg). Để nguyên trên production
    // là lỗ hổng lạm dụng băng thông và SSRF.
    remotePatterns: [{ protocol: 'https', hostname: '**' }],
  },
}

export default nextConfig
