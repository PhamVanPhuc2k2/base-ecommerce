/**
 * Định dạng dữ liệu để hiển thị. Chỉ dùng cho hiển thị, không dùng để tính toán.
 */

// Chuỗi thập phân backend trả về: tùy chọn dấu âm, phần nguyên, tùy chọn phần
// thập phân. Kiểm bằng regex TRƯỚC khi gọi Number vì Number quá dễ dãi:
// Number('') === 0 và Number(' 12 ') === 12, nên chuỗi rỗng sẽ lặng lẽ thành
// "0 ₫" — hiển thị sai giá còn tệ hơn hiển thị nguyên dữ liệu hỏng.
const DECIMAL = /^-?\d+(\.\d+)?$/

const VND = new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' })

/**
 * Định dạng một số tiền để hiển thị, ví dụ `'25990000'` → `'25.990.000 ₫'`.
 *
 * Nhận CHUỖI vì trường `price` của API là chuỗi thập phân (README mục 8.2):
 * tiền không bao giờ được đi qua `number` của JS trên đường truyền.
 *
 * ---
 * RANH GIỚI CỦA `Number` — đọc kỹ trước khi sao chép đoạn này đi nơi khác:
 *
 *   HIỂN THỊ MỘT GIÁ  → dùng `Number` ĐƯỢC.
 *     Trần của cột NUMERIC(15,2) là 9_999_999_999_999.99 ≈ 10^13, trong khi
 *     Number.MAX_SAFE_INTEGER ≈ 9×10^15. Còn dư ba bậc, một giá trị đơn lẻ
 *     luôn biểu diễn chính xác.
 *
 *   CỘNG NHIỀU GIÁ, NHÂN SỐ LƯỢNG, TÍNH THUẾ → KHÔNG được dùng `Number`.
 *     Phép cộng dấu phẩy động làm sai số tích lũy (0.1 + 0.2 !== 0.3), và tiền
 *     thì không được sai một đồng. Giỏ hàng ở P4 phải dùng thư viện decimal,
 *     hoặc để backend tính rồi trả về tổng đã tính sẵn.
 *
 * Vì vậy việc chuyển sang `Number` chỉ xảy ra ở ĐÚNG MỘT CHỖ: bên trong hàm
 * này, ngay trước khi in ra màn hình.
 * ---
 *
 * Ghi chú: `Intl` làm tròn VND về đồng chẵn (VND không có đơn vị nhỏ hơn),
 * nên `'25990000.5'` hiển thị thành `'25.990.001 ₫'`. Đó là đúng ý cho việc
 * hiển thị — nhưng cũng là một lý do nữa để không bao giờ tính toán trên kết
 * quả của hàm này.
 *
 * Chuỗi không parse được (dữ liệu hỏng, chuỗi rỗng, chữ) được trả về NGUYÊN
 * VĂN. Một bản ghi lỗi không được phép làm vỡ cả trang danh mục, và `'NaN ₫'`
 * thì vừa vô nghĩa với khách vừa giấu mất dữ liệu thật khi đi tìm nguyên nhân.
 */
export function formatVND(amount: string): string {
  const raw = amount.trim()
  if (!DECIMAL.test(raw)) return amount
  const n = Number(raw)
  if (!Number.isFinite(n)) return amount
  return VND.format(n)
}

const DATE = new Intl.DateTimeFormat('vi-VN', {
  // Backend trả RFC 3339 ở UTC (hậu tố Z). Ép múi giờ Việt Nam thay vì để chạy
  // theo máy chủ: container thường đặt TZ=UTC, không ép thì khách ở VN thấy
  // ngày lệch bảy tiếng — và sai rõ nhất vào buổi tối, lúc đông người mua nhất.
  timeZone: 'Asia/Ho_Chi_Minh',
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

/**
 * Định dạng mốc thời gian ISO 8601 UTC sang giờ Việt Nam để hiển thị.
 *
 * Chuỗi không phải ngày hợp lệ được trả về nguyên văn, cùng lý do với
 * `formatVND`: `'Invalid Date'` trên trang sản phẩm còn tệ hơn dữ liệu thô.
 */
export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return DATE.format(d)
}
