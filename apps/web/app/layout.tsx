import type { Metadata } from 'next'
import { Geist, Geist_Mono } from 'next/font/google'
import './globals.css'

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
})

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
})

export const metadata: Metadata = {
  title: 'Base E-commerce',
  description: 'Storefront thương mại điện tử',
}

// lang="vi" chứ không phải "en": trình đọc màn hình và công cụ dịch của trình
// duyệt dựa vào thuộc tính này để chọn giọng đọc và bộ quy tắc ngắt dòng.
export default function RootLayout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="vi" className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}>
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  )
}
