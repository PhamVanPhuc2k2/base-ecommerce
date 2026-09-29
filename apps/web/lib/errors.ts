/**
 * Bảng tra mã lỗi backend → thông điệp tiếng Việt cho KHÁCH đọc.
 *
 * Đây là nơi DUY NHẤT dịch mã lỗi sang chữ (README mục 7.7). Trang không được
 * tự chế thông điệp riêng, nếu không cùng một sự cố sẽ hiện mỗi chỗ một kiểu.
 *
 * Nguyên tắc viết câu: viết cho người mua hàng, không viết cho lập trình viên.
 * Nói cho khách biết CHUYỆN GÌ và LÀM GÌ TIẾP, không nói tên ràng buộc trong
 * cơ sở dữ liệu. "Mã sản phẩm này đã tồn tại." chứ không phải "DUPLICATE_SKU:
 * vi phạm ràng buộc duy nhất trên cột sku".
 *
 * Danh sách mã là hợp đồng, lấy từ enum `Problem.code` trong api/openapi.yaml.
 * `scripts/check-error-messages.sh` đối chiếu hai bên và báo đỏ cả hai chiều —
 * thiếu một mã nghĩa là đúng lúc có sự cố khách lại nhận câu chung chung vô
 * dụng, còn thừa một khóa nghĩa là bảng này đang nói về mã không tồn tại.
 */
export const errorMessages: Record<string, string> = {
  ALREADY_PUBLISHED: 'Sản phẩm này đã được đăng bán trước đó rồi.',
  BRAND_NOT_FOUND: 'Không tìm thấy thương hiệu này.',
  CATEGORY_NOT_FOUND: 'Không tìm thấy danh mục này.',
  DUPLICATE_SKU: 'Mã sản phẩm này đã tồn tại. Vui lòng dùng mã khác.',
  DUPLICATE_SLUG: 'Đường dẫn này đã có sản phẩm khác sử dụng. Vui lòng đổi tên sản phẩm.',
  INTERNAL_ERROR: 'Hệ thống đang gặp sự cố. Vui lòng thử lại sau ít phút.',
  INVALID_PAGINATION: 'Số trang hoặc số sản phẩm mỗi trang không hợp lệ.',
  INVALID_PRICE: 'Giá sản phẩm không hợp lệ.',
  INVALID_SKU: 'Mã sản phẩm không hợp lệ.',
  INVALID_SLUG: 'Đường dẫn sản phẩm không hợp lệ.',
  INVALID_SORT: 'Cách sắp xếp này không được hỗ trợ.',
  INVALID_STATUS: 'Trạng thái sản phẩm không hợp lệ.',
  MALFORMED_REQUEST: 'Dữ liệu gửi đi không đọc được. Vui lòng tải lại trang rồi thử lại.',
  METHOD_NOT_ALLOWED: 'Thao tác này không được hỗ trợ ở đây.',
  NAME_REQUIRED: 'Vui lòng nhập tên sản phẩm.',
  NAME_TOO_LONG: 'Tên sản phẩm quá dài. Vui lòng rút ngắn lại.',
  NO_IMAGE: 'Sản phẩm cần có ít nhất một ảnh.',
  PAGE_TOO_DEEP: 'Bạn đã xem tới trang cuối cùng hiển thị được. Hãy lọc lại để thu hẹp kết quả.',
  PAYLOAD_TOO_LARGE: 'Nội dung gửi lên quá lớn. Vui lòng giảm bớt rồi thử lại.',
  PRICE_REQUIRED: 'Vui lòng nhập giá sản phẩm.',
  PRODUCT_NOT_FOUND: 'Không tìm thấy sản phẩm này. Có thể sản phẩm đã ngừng kinh doanh.',
  REQUEST_CANCELED: 'Yêu cầu đã bị hủy giữa chừng. Vui lòng thử lại.',
  REQUEST_TIMEOUT: 'Hệ thống xử lý quá lâu nên đã dừng lại. Vui lòng thử lại.',
  ROUTE_NOT_FOUND: 'Không tìm thấy trang bạn đang tìm.',
  UNAUTHENTICATED: 'Bạn cần đăng nhập để thực hiện thao tác này.',
  UNSUPPORTED_CURRENCY: 'Loại tiền tệ này chưa được hỗ trợ.',
  VALIDATION_FAILED: 'Thông tin chưa hợp lệ. Vui lòng kiểm tra lại các ô đã nhập.',

  UNKNOWN: 'Đã có lỗi xảy ra. Vui lòng thử lại.',
  NETWORK_ERROR: 'Không kết nối được tới máy chủ. Vui lòng kiểm tra mạng rồi thử lại.',
}

/**
 * Trả thông điệp tiếng Việt cho một mã lỗi.
 *
 * Luôn trả về chuỗi dùng được, kể cả với mã lạ: backend có thể lên phiên bản
 * mới trước frontend, và lúc đó khách vẫn phải thấy một câu tử tế thay vì
 * `undefined` hay chuỗi rỗng.
 */
export function messageFor(code: string): string {
  return errorMessages[code] ?? 'Đã có lỗi xảy ra. Vui lòng thử lại.'
}
