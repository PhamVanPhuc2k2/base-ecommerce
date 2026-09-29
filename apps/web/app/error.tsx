'use client'

import { messageFor } from '@/lib/errors'

/**
 * LƯỚI AN TOÀN CUỐI CÙNG cho lỗi lọt ra ngoài mọi chỗ khác. KHÔNG phải nơi
 * hiển thị lỗi API thông thường — chỗ đó là `components/error-state.tsx`.
 *
 * ------------------------------------------------------------------------
 * VÌ SAO Ở ĐÂY KHÔNG ĐỌC `ApiError.code` — đọc kỹ trước khi "dọn dẹp" file này
 * ------------------------------------------------------------------------
 * `error.tsx` bắt buộc là Client Component (React error boundary chỉ chạy được
 * ở phía client). Khi một Server Component ném lỗi, Next.js phải tuần tự hóa
 * lỗi đó rồi gửi xuống trình duyệt — và hai bản chạy hành xử KHÁC NHAU:
 *
 *   next dev  → gửi kèm nguyên văn `error.message` để lập trình viên dễ sửa.
 *   next build → CỐ Ý thay bằng câu chung chung "An error occurred in the
 *                Server Components render...", chỉ giữ lại `error.digest`.
 *
 * Đó là tính năng chống rò rỉ, không phải lỗi: thông điệp lỗi phía server hay
 * chứa chuỗi kết nối, đường dẫn nội bộ, câu SQL. Hệ quả với chúng ta: thuộc
 * tính `code` của `ApiError` KHÔNG tới được file này trên production. Prop
 * `error` ở đây là một `Error` trơn do Next.js dựng lại, không phải đối tượng
 * `ApiError` gốc — có ép kiểu `(error as ApiError).code` cũng chỉ nhận về
 * `undefined`.
 *
 * Cái bẫy nằm ở chỗ: viết `error.code` rồi thử bằng `npm run dev` sẽ thấy chạy
 * đúng y như mong đợi. Nó chỉ hỏng sau khi deploy, và hỏng âm thầm — khách
 * nhận câu chung chung thay vì "Không tìm thấy sản phẩm này".
 *
 * Cách làm đúng của dự án (README mục 7.7): Server Component tự `try/catch`
 * quanh `apiGet` rồi render `<ErrorState code={e.code} requestId={e.requestId} />`.
 * Ở đó vẫn đang chạy trên server nên không mất mát gì cả.
 *
 * Còn lại file này chỉ lo những gì thật sự bất ngờ (lỗi lập trình, backend trả
 * rác, lỗi render ở client): một câu chung chung + `digest` để tra log server.
 * ------------------------------------------------------------------------
 */
export default function RouteError({
  error,
  retry,
}: {
  error: Error & { digest?: string }
  /**
   * `retry` chứ không phải `reset` (Next 16.3 chuyển `retry` thành API chính
   * thức). Khác biệt quan trọng: `reset()` chỉ xóa trạng thái lỗi của boundary
   * rồi render lại bằng dữ liệu CŨ — với lỗi phát sinh khi gọi API thì nó dựng
   * lại đúng cái lỗi vừa rồi, bấm bao nhiêu lần cũng vậy. `retry()` gọi thêm
   * `router.refresh()`, tức là hỏi lại server — đó mới là điều khách mong đợi
   * khi bấm "Thử lại".
   */
  retry: () => void
}) {
  return (
    <div className="mx-auto max-w-xl py-10 text-center">
      <h1 className="text-xl font-semibold text-gray-900">Trang đang gặp sự cố</h1>
      {/*
        Dùng mã 'UNKNOWN' là CHỦ Ý, không phải tạm bợ: tới được đây nghĩa là ta
        thật sự không biết mã lỗi (xem phần giải thích ở trên). Vẫn tra qua
        messageFor để câu chữ đi chung một nguồn với mọi chỗ khác, thay vì chế
        thêm một câu tiếng Việt thứ hai trong file này.
      */}
      <p className="mt-2 text-base text-gray-600">{messageFor('UNKNOWN')}</p>

      <button
        type="button"
        onClick={() => retry()}
        className="mt-6 rounded-md bg-brand px-5 py-2.5 text-sm font-medium text-white hover:bg-brand-dark"
      >
        Thử lại
      </button>

      {/*
        `digest` là mã băm của lỗi, được Next.js ghi kèm vào log của server.
        Đây là thứ DUY NHẤT nối trang lỗi khách đang nhìn với nguyên nhân thật
        trong log, nên dù không đẹp vẫn phải hiện ra — nhỏ và mờ ở góc.
      */}
      {error.digest ? (
        <p className="mt-8 text-xs text-gray-400">
          Mã tra cứu: <span className="font-mono">{error.digest}</span>
        </p>
      ) : null}
    </div>
  )
}
