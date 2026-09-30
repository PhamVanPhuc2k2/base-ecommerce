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
  BRAND_HAS_PRODUCTS:
    'Thương hiệu này vẫn còn sản phẩm. Hãy chuyển các sản phẩm sang thương hiệu khác trước khi xóa.',
  BRAND_NAME_INVALID: 'Vui lòng nhập tên thương hiệu, tối đa 100 ký tự.',
  BRAND_NOT_FOUND: 'Không tìm thấy thương hiệu này.',
  CATEGORY_CYCLE: 'Không thể chuyển danh mục vào bên trong chính nó hoặc danh mục con của nó.',
  CATEGORY_HAS_CHILDREN: 'Danh mục này vẫn còn danh mục con. Hãy chuyển hoặc xóa chúng trước.',
  CATEGORY_HAS_PRODUCTS:
    'Danh mục này vẫn còn sản phẩm. Hãy chuyển các sản phẩm sang danh mục khác trước khi xóa.',
  CATEGORY_NAME_INVALID: 'Vui lòng nhập tên danh mục, tối đa 100 ký tự.',
  CATEGORY_NOT_FOUND: 'Không tìm thấy danh mục này.',
  DUPLICATE_BRAND_SLUG: 'Đường dẫn này đã có thương hiệu khác sử dụng.',
  DUPLICATE_CATEGORY_SLUG: 'Đường dẫn này đã có danh mục khác sử dụng.',
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
  UNKNOWN_BRAND: 'Không tìm thấy thương hiệu này.',
  UNKNOWN_CATEGORY: 'Không tìm thấy danh mục này.',
  UNSUPPORTED_CURRENCY: 'Loại tiền tệ này chưa được hỗ trợ.',
  VALIDATION_FAILED: 'Thông tin chưa hợp lệ. Vui lòng kiểm tra lại các ô đã nhập.',
  DUPLICATE_VARIANT_OPTIONS: 'Đã có phiên bản khác với đúng các tùy chọn này.',
  INVALID_VARIANT_OPTIONS: 'Tùy chọn phiên bản chưa hợp lệ. Vui lòng kiểm tra lại tên và giá trị.',
  INVALID_VARIANT_STATUS: 'Trạng thái phiên bản không hợp lệ.',
  NO_ACTIVE_VARIANT: 'Sản phẩm đang bán cần còn ít nhất một phiên bản đang bán.',
  UNKNOWN_VARIANT: 'Không tìm thấy phiên bản này của sản phẩm.',
  VARIANT_REQUIRED: 'Sản phẩm cần có ít nhất một phiên bản.',
  ATTRIBUTE_IN_USE:
    'Thuộc tính này đang được dùng ở một số danh mục. Hãy gỡ khỏi các danh mục trước khi xóa.',
  ATTRIBUTE_NAME_INVALID: 'Vui lòng nhập tên thuộc tính, tối đa 100 ký tự.',
  ATTRIBUTE_NOT_FOUND: 'Không tìm thấy thuộc tính này.',
  DUPLICATE_ATTRIBUTE_ASSIGNMENT: 'Mỗi thuộc tính chỉ được gán một lần cho một danh mục.',
  DUPLICATE_ATTRIBUTE_CODE: 'Mã thuộc tính này đã tồn tại.',
  INVALID_ATTRIBUTE_CODE: 'Mã thuộc tính chỉ gồm chữ thường không dấu, số và dấu gạch dưới.',
  INVALID_ATTRIBUTE_OPTIONS: 'Danh sách giá trị của thuộc tính chưa hợp lệ.',
  INVALID_ATTRIBUTE_TYPE: 'Kiểu thuộc tính không được hỗ trợ.',
  UNKNOWN_ATTRIBUTE: 'Không tìm thấy thuộc tính này.',
  IMAGE_TOO_LARGE: 'Ảnh quá lớn. Vui lòng chọn ảnh không quá 10 MB.',
  INVALID_IMAGE: 'File tải lên không phải ảnh hợp lệ. Vui lòng chọn ảnh JPEG, PNG hoặc WebP.',
  UNKNOWN_MEDIA: 'Không tìm thấy ảnh này.',
  UNSUPPORTED_IMAGE_TYPE: 'Chỉ nhận ảnh JPEG, PNG hoặc WebP.',
  UPLOAD_NOT_FOUND: 'Chưa thấy ảnh được tải lên. Vui lòng tải ảnh lên trước.',
  ACCOUNT_DISABLED: 'Tài khoản của bạn đã bị khóa. Vui lòng liên hệ bộ phận hỗ trợ.',
  EMAIL_TAKEN: 'Email này đã được dùng để đăng ký. Bạn có thể đăng nhập hoặc lấy lại mật khẩu.',
  FULL_NAME_INVALID: 'Vui lòng nhập họ tên, tối đa 100 ký tự.',
  INVALID_CREDENTIALS: 'Email hoặc mật khẩu không đúng.',
  INVALID_EMAIL: 'Địa chỉ email không hợp lệ.',
  INVALID_REFRESH_TOKEN: 'Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.',
  RATE_LIMITED: 'Bạn thao tác quá nhanh. Vui lòng thử lại sau ít phút.',
  WEAK_PASSWORD: 'Mật khẩu phải dài từ 8 đến 128 ký tự.',
  INVALID_OTP: 'Mã xác nhận không đúng hoặc đã hết hạn. Hãy kiểm tra lại hoặc yêu cầu mã mới.',
  EMAIL_ALREADY_VERIFIED: 'Email của bạn đã được xác minh.',
  CANNOT_CHANGE_OWN_ROLES: 'Bạn không thể tự thay đổi vai trò của chính mình.',
  DUPLICATE_ROLE_CODE: 'Mã vai trò này đã tồn tại.',
  FORBIDDEN: 'Bạn không có quyền thực hiện thao tác này.',
  INVALID_ROLE_CODE: 'Mã vai trò chỉ gồm chữ thường không dấu, số và dấu gạch dưới.',
  ROLE_IN_USE: 'Vai trò này vẫn đang được gán cho người dùng. Hãy gỡ trước khi xóa.',
  ROLE_NAME_INVALID: 'Vui lòng nhập tên vai trò, tối đa 100 ký tự.',
  ROLE_NOT_FOUND: 'Vai trò không tồn tại.',
  SYSTEM_ROLE_IMMUTABLE: 'Không thể sửa hoặc xóa vai trò hệ thống.',
  UNKNOWN_PERMISSION: 'Có quyền không tồn tại trong hệ thống.',
  UNKNOWN_ROLE: 'Không tìm thấy vai trò này.',
  UNKNOWN_USER: 'Không tìm thấy người dùng này.',

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
