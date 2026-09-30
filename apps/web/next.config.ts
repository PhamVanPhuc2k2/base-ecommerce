import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  // Cần cho Docker (Task 6): gom đúng những gì runtime cần vào .next/standalone,
  // image từ ~1 GB xuống ~150 MB.
  output: 'standalone',
  images: {
    /*
      Ảnh đi qua imgproxy (P1.4), KHÔNG qua bộ tối ưu /_next/image của Next.

      Nhờ vậy bỏ hẳn được `remotePatterns: '**'` của P0.4 — cấu hình đó biến
      server thành proxy tải và thu nhỏ ảnh từ BẤT KỲ đâu trên internet (lạm
      dụng băng thông, SSRF). Giờ không còn đường nào để Next.js tự đi lấy ảnh
      ngoài: loader chỉ sinh `/img/<preset>/<khóa>`, và Route Handler chỉ nhận
      khóa của bucket mình.
    */
    loader: 'custom',
    loaderFile: './lib/image-loader.ts',
    // Đúng các bậc preset của imgproxy (xem lib/image-loader.ts). Để mặc định
    // (640, 750, 828, 1080, 1200, 1920, 2048, 3840) thì srcset sinh ra nhiều
    // chiều rộng cùng rơi vào một preset — trùng URL, phí dòng HTML.
    deviceSizes: [640, 828, 1080, 1920],
    imageSizes: [128, 256, 384],
  },
}

export default nextConfig
