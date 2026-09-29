import type { Metadata } from 'next'
import localFont from 'next/font/local'
import Link from 'next/link'
import './globals.css'

// Font tự host từ app/fonts/ (Geist 1.7.2 của Vercel, giấy phép OFL — xem
// app/fonts/OFL.txt), KHÔNG dùng next/font/google.
//
// Lý do đo được ở P0.4 Task 6: next/font/google tải font từ fonts.googleapis.com
// LÚC `next build`. Chặn riêng hai tên miền đó (docker build --add-host ...) thì
// build hỏng hẳn với "Failed to fetch Geist from Google Fonts" — tức mỗi lần
// deploy phụ thuộc vào việc máy build ra được Google. File .woff2 nằm trong repo
// thì build chạy được cả khi không có mạng ngoài.
//
// Bản variable chứa ĐỦ glyph tiếng Việt (đã kiểm từng chữ ắ ằ ễ ộ ự Đ Ơ Ư bằng
// fontTools), nên không còn chuyện chọn subsets 'latin-ext' như bản Google:
// thiếu subset đó thì chữ có dấu lấy từ font hệ điều hành, một dòng tiêu đề pha
// hai bộ chữ. Riêng ký hiệu ₫ thì Geist không có (bản Google cũng vậy) — nó vẫn
// hiện bằng font dự phòng trong --font-sans.
const geistSans = localFont({
  src: './fonts/Geist-Variable.woff2',
  variable: '--font-geist-sans',
  weight: '100 900',
  display: 'swap',
})

const geistMono = localFont({
  src: './fonts/GeistMono-Variable.woff2',
  variable: '--font-geist-mono',
  weight: '100 900',
  display: 'swap',
})

const STORE_NAME = 'Base E-commerce'

export const metadata: Metadata = {
  title: {
    // `default` dùng cho trang không tự khai title (kể cả trang lỗi, 404).
    default: `${STORE_NAME} — Máy tính, laptop, linh kiện chính hãng`,
    // `template` để mỗi trang con chỉ cần khai phần riêng của nó, ví dụ
    // 'Laptop gaming' → 'Laptop gaming | Base E-commerce'. Không có template
    // thì từng trang phải tự nối tên cửa hàng, và chỉ cần một trang quên là
    // kết quả tìm kiếm của Google mất hẳn tên thương hiệu ở dòng tiêu đề.
    template: `%s | ${STORE_NAME}`,
  },
  description:
    'Cửa hàng máy tính Base E-commerce: laptop, PC, linh kiện và phụ kiện chính hãng, giá niêm yết rõ ràng, bảo hành đầy đủ.',
}

// lang="vi" chứ không phải "en": trình đọc màn hình và công cụ dịch của trình
// duyệt dựa vào thuộc tính này để chọn giọng đọc và bộ quy tắc ngắt dòng.
export default function RootLayout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="vi" className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}>
      <body className="flex min-h-full flex-col">
        <SiteHeader />
        {/*
          <main> đặt tại layout, KHÔNG đặt trong từng page. Một tài liệu HTML
          chỉ được có đúng một <main> hiện hữu; để mỗi trang tự thêm thì sớm
          muộn sẽ có trang lồng <main> trong <main>, và phím tắt "nhảy tới nội
          dung chính" của trình đọc màn hình mất tác dụng. Hệ quả: trang con
          chỉ render nội dung, không bọc thêm <main> nữa.
        */}
        <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6">{children}</main>
        <SiteFooter />
      </body>
    </html>
  )
}

function SiteHeader() {
  return (
    <header className="border-b border-gray-200 bg-white">
      <div className="mx-auto flex h-16 w-full max-w-6xl items-center gap-8 px-4">
        <Link
          href="/"
          className="shrink-0 text-xl font-bold tracking-tight text-brand hover:text-brand-dark"
        >
          {STORE_NAME}
        </Link>
        <nav aria-label="Điều hướng chính" className="text-sm font-medium">
          <Link href="/danh-muc" className="text-gray-700 hover:text-brand">
            Danh mục sản phẩm
          </Link>
        </nav>
      </div>
    </header>
  )
}

function SiteFooter() {
  return (
    <footer className="border-t border-gray-200 bg-gray-50">
      <div className="mx-auto w-full max-w-6xl px-4 py-6 text-sm text-gray-500">
        {/*
          Năm viết cứng, KHÔNG dùng new Date().getFullYear(). Trang danh mục
          render theo ISR nên HTML được sinh lúc build rồi phục vụ lại nhiều
          tháng: hàm lấy năm sẽ đóng băng ở năm build và tới 1/1 là footer sai
          năm trên một trang chẳng ai ngờ tới. Sai thì sửa tay một lần mỗi năm,
          còn hơn sai âm thầm.
        */}
        <p>© 2026 {STORE_NAME}. Giá đã bao gồm VAT.</p>
      </div>
    </footer>
  )
}
