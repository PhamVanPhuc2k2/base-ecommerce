/**
 * Lỗi thống nhất của lớp gọi API.
 *
 * Mọi thứ hỏng khi gọi backend đều tới tay người viết trang dưới đúng một dạng
 * này — dù nguyên nhân là response 4xx/5xx có problem+json, proxy trả HTML, hay
 * `fetch` ném vì không kết nối được. Nhờ vậy trang chỉ cần bắt một loại lỗi và
 * tra `code` sang thông điệp tiếng Việt (xem `lib/errors.ts`).
 *
 * `code` là hợp đồng ổn định với backend (RFC 7807, trường `code`), KHÔNG phải
 * `status`: cùng một 404 có thể là PRODUCT_NOT_FOUND hay ROUTE_NOT_FOUND, hai
 * thông điệp hoàn toàn khác nhau đối với khách.
 */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    readonly requestId?: string,
  ) {
    // Message này dành cho log của server, KHÔNG phải cho khách đọc. Thông điệp
    // tiếng Việt hiển thị lấy từ messageFor(code).
    super(`${code} (HTTP ${status})`)
    this.name = 'ApiError'
  }
}
