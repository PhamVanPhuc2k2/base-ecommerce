import Link from 'next/link'
import type { components } from '@/lib/api/generated/schema'
import { hrefWith } from '@/lib/search-params'

type Meta = components['schemas']['ProductList']['meta']

/**
 * Phân trang cho danh sách công khai.
 *
 * ------------------------------------------------------------------------
 * CHỈ ĐƯỢC DÙNG `meta.has_next` / `meta.has_prev`. ĐỪNG tự tính
 * `page < total_pages` — đọc kỹ trước khi "đơn giản hóa" file này.
 * ------------------------------------------------------------------------
 * Backend chặn cứng ở trang 200 (`meta.max_page`): phân trang kiểu offset phải
 * quét và bỏ qua `(page-1) * limit` dòng, nên tới trang vài nghìn thì một lượt
 * xem danh mục đủ sức làm nghẽn cả cơ sở dữ liệu. Đây là cách chống lại việc bị
 * bò dữ liệu, không phải một giới hạn tùy tiện.
 *
 * Chỗ dễ sập bẫy: `total_pages` KHÔNG bị kẹp theo trần đó. Với 200.000 sản phẩm
 * và limit 24, API vẫn thành thật báo `total_pages: 8334` trong khi trang sâu
 * nhất còn vào được là 200. Tự tính `page < total_pages` thì ở trang 200 nút
 * "Trang sau" vẫn hiện, khách bấm vào và nhận 400 PAGE_TOO_DEEP — một trang lỗi
 * mọc ra từ một nút mà chính chúng ta vẽ ra.
 *
 * `has_next` đã tính tới trần này rồi: nó về `false` ngay tại trang `max_page`.
 * Vì vậy quy tắc là tin `meta`, đừng tự suy luận lại.
 */
export function Pagination({
  meta,
  basePath,
  params,
}: {
  meta: Meta
  basePath: string
  params: URLSearchParams
}) {
  if (!meta.has_prev && !meta.has_next) return null

  // Số trang hiển thị cho khách phải kẹp theo max_page, cùng lý do như trên:
  // ghi "Trang 200 / 8334" là hứa với khách một thứ hệ thống không giao được.
  const lastReachable = Math.min(meta.total_pages, meta.max_page)

  /*
    Trang hiện tại có thể NẰM NGOÀI phạm vi thật: `?page=200` khi danh sách chỉ
    có 2 trang vẫn được backend chấp nhận (200 vẫn trong trần max_page), chỉ là
    trả về `data` rỗng. Đã gặp thật khi kiểm chứng.

    Hai hệ quả phải xử lý, nếu không giao diện sẽ nói dối:
      1. Đừng ghi "Trang 200 / 2" — vô nghĩa với người đọc.
      2. "Trang trước" phải nhảy thẳng về trang cuối CÓ THẬT, chứ không phải
         page - 1. Bấm về trang 199 cũng rỗng nốt, và khách sẽ phải bấm 198 lần
         mới thấy lại sản phẩm.
  */
  const outOfRange = meta.page > lastReachable
  const prevPage = Math.min(meta.page - 1, lastReachable)

  return (
    <nav aria-label="Phân trang" className="mt-8 flex items-center justify-center gap-4">
      {meta.has_prev ? (
        <Link
          href={hrefWith(basePath, params, { page: String(prevPage) })}
          rel="prev"
          className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:border-brand hover:text-brand"
        >
          ← Trang trước
        </Link>
      ) : (
        // Chỗ giữ chỗ vô hình để hai nút không nhảy sang trái/phải khi đi qua
        // trang đầu và trang cuối.
        <span aria-hidden="true" className="invisible rounded-md px-4 py-2 text-sm">
          ← Trang trước
        </span>
      )}

      <span aria-live="polite" className="text-sm text-gray-600">
        {outOfRange ? (
          <>
            Trang <strong className="text-gray-900">{meta.page}</strong> — danh sách chỉ có{' '}
            {lastReachable} trang
          </>
        ) : (
          <>
            Trang <strong className="text-gray-900">{meta.page}</strong> / {lastReachable}
          </>
        )}
      </span>

      {meta.has_next ? (
        <Link
          href={hrefWith(basePath, params, { page: String(meta.page + 1) })}
          rel="next"
          className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:border-brand hover:text-brand"
        >
          Trang sau →
        </Link>
      ) : (
        <span aria-hidden="true" className="invisible rounded-md px-4 py-2 text-sm">
          Trang sau →
        </span>
      )}
    </nav>
  )
}
