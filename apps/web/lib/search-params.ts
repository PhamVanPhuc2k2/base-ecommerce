/**
 * Dựng link cho trạng thái nằm trên query string (bộ lọc, sắp xếp, phân trang).
 *
 * Vì sao cần một chỗ riêng thay vì nối chuỗi tại từng component: quy tắc "giữ
 * nguyên các tham số khác, chỉ đổi một tham số, và reset `page` về 1" phải
 * giống hệt nhau ở mọi link lọc. Chỉ cần một chỗ quên reset `page` là khách
 * đang ở trang 5 bấm đổi danh mục sẽ rơi vào một trang rỗng — bộ lọc mới chỉ
 * có 2 trang, mà URL vẫn xin trang 5. Lỗi đó không làm vỡ gì cả, nó chỉ lặng
 * lẽ cho ra "không có sản phẩm nào", nên rất khó phát hiện khi thử tay.
 *
 * Không dùng `useState`: URL là nguồn sự thật (README mục 7.3) — nhờ vậy link
 * chia sẻ được, nút Back của trình duyệt hoạt động đúng, và trang vẫn là
 * Server Component nên Google đọc được kết quả đã lọc.
 */

/** Đúng hình dạng `searchParams` mà Next.js trao cho page sau khi `await`. */
export type RawSearchParams = Record<string, string | string[] | undefined>

/**
 * Chuẩn hóa `searchParams` của Next.js thành `URLSearchParams`.
 *
 * Next.js trả `string[]` khi một khóa xuất hiện nhiều lần (`?sort=a&sort=b`) —
 * chuyện này xảy ra thật khi người dùng sửa tay URL hoặc khi một link bị dựng
 * sai. Ở đây lấy GIÁ TRỊ ĐẦU TIÊN thay vì nối chúng lại: backend chỉ hiểu một
 * giá trị cho mỗi tham số, gửi cả hai lên thì hoặc bị từ chối, hoặc (tệ hơn)
 * được backend tự chọn một cái theo quy tắc mà frontend không biết — và khi đó
 * nút đang sáng trên giao diện có thể không phải nút đang thật sự áp dụng.
 */
export function toSearchParams(raw: RawSearchParams): URLSearchParams {
  const out = new URLSearchParams()
  for (const [key, value] of Object.entries(raw)) {
    if (value === undefined) continue
    if (Array.isArray(value)) {
      // noUncheckedIndexedAccess: mảng rỗng vẫn là mảng, value[0] có thể undefined.
      const first = value[0]
      if (first !== undefined && first !== '') out.set(key, first)
      continue
    }
    if (value !== '') out.set(key, value)
  }
  return out
}

/**
 * Dựng href mới từ tham số hiện tại, chỉ thay những khóa có trong `patch`.
 *
 * - Giá trị `undefined` hoặc chuỗi rỗng nghĩa là XÓA tham số đó (ví dụ bấm
 *   "Tất cả danh mục" thì bỏ hẳn `category`, chứ không phải `category=`).
 * - `page` bị xóa tự động, TRỪ KHI chính `patch` nói về `page`. Xem giải thích
 *   ở đầu file: đổi bộ lọc mà giữ nguyên số trang là ra trang rỗng.
 *
 * Tham số được sắp theo thứ tự chữ cái để cùng một bộ lọc luôn cho ra đúng một
 * chuỗi URL, bất kể người dùng bấm theo thứ tự nào. Nhờ vậy hai người lọc
 * giống nhau chia sẻ ra cùng một link, và cache tầng trên (CDN sau này) không
 * bị tách thành nhiều bản chỉ vì thứ tự tham số khác nhau.
 */
export function hrefWith(
  pathname: string,
  current: URLSearchParams,
  patch: Record<string, string | undefined>,
): string {
  const next = new URLSearchParams(current)
  for (const [key, value] of Object.entries(patch)) {
    if (value === undefined || value === '') next.delete(key)
    else next.set(key, value)
  }
  if (!('page' in patch)) next.delete('page')
  return hrefCurrent(pathname, next)
}

/**
 * Href của chính trạng thái đang xem, đã chuẩn hóa thứ tự tham số.
 *
 * Dùng để so sánh: một nút "Xóa bộ lọc" trỏ đúng về URL khách đang đứng thì
 * không làm gì cả, chỉ tổ khiến họ bấm rồi tưởng trang bị treo. So chuỗi href
 * là cách rẻ nhất để phát hiện và ẩn những nút vô nghĩa như vậy — và chỉ so
 * được nếu thứ tự tham số đã chuẩn hóa, nên phép sắp xếp nằm ở đây.
 */
export function hrefCurrent(pathname: string, current: URLSearchParams): string {
  const next = new URLSearchParams(current)
  next.sort()
  const qs = next.toString()
  return qs === '' ? pathname : `${pathname}?${qs}`
}
