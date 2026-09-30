package domain

import "base-ecommerce/api/pkg/errs"

// Sentinel error của catalog.
//
// Message phải là câu tiếng Việt viết sẵn, KHÔNG BAO GIỜ nội suy từ lỗi gốc —
// nó ra thẳng trường title của response và sẽ làm lộ tên bảng, tên ràng buộc.
var (
	ErrInvalidSKU = errs.New(errs.KindValidation, "INVALID_SKU",
		"SKU không hợp lệ: bắt buộc có và tối đa 64 ký tự")
	ErrNameRequired = errs.New(errs.KindValidation, "NAME_REQUIRED",
		"Tên sản phẩm là bắt buộc")
	ErrNameTooLong = errs.New(errs.KindValidation, "NAME_TOO_LONG",
		"Tên sản phẩm tối đa 200 ký tự")
	ErrInvalidPrice = errs.New(errs.KindValidation, "INVALID_PRICE",
		"Giá không hợp lệ")
	ErrUnsupportedCurrency = errs.New(errs.KindValidation, "UNSUPPORTED_CURRENCY",
		"Hệ thống hiện chỉ hỗ trợ tiền VND")
	ErrInvalidSlug = errs.New(errs.KindValidation, "INVALID_SLUG",
		"Đường dẫn sản phẩm không hợp lệ")
	ErrNoImage = errs.New(errs.KindValidation, "NO_IMAGE",
		"Sản phẩm phải có ít nhất một ảnh trước khi đăng bán")
	ErrPriceRequired = errs.New(errs.KindValidation, "PRICE_REQUIRED",
		"Sản phẩm phải có giá lớn hơn 0 trước khi đăng bán")
	ErrInvalidStatus = errs.New(errs.KindValidation, "INVALID_STATUS",
		"Trạng thái sản phẩm không hợp lệ")

	ErrAlreadyPublished = errs.New(errs.KindConflict, "ALREADY_PUBLISHED",
		"Sản phẩm đã được đăng bán")
	ErrDuplicateSKU = errs.New(errs.KindConflict, "DUPLICATE_SKU",
		"SKU này đã tồn tại")
	ErrDuplicateSlug = errs.New(errs.KindConflict, "DUPLICATE_SLUG",
		"Đường dẫn này đã được dùng cho sản phẩm khác")

	ErrProductNotFound = errs.New(errs.KindNotFound, "PRODUCT_NOT_FOUND",
		"Không tìm thấy sản phẩm")
	ErrCategoryNotFound = errs.New(errs.KindValidation, "CATEGORY_NOT_FOUND",
		"Danh mục không tồn tại")
	ErrBrandNotFound = errs.New(errs.KindValidation, "BRAND_NOT_FOUND",
		"Thương hiệu không tồn tại")

	// Danh mục và thương hiệu (P1.1).
	ErrCategoryNameInvalid = errs.New(errs.KindValidation, "CATEGORY_NAME_INVALID",
		"Tên danh mục là bắt buộc và tối đa 100 ký tự")
	ErrBrandNameInvalid = errs.New(errs.KindValidation, "BRAND_NAME_INVALID",
		"Tên thương hiệu là bắt buộc và tối đa 100 ký tự")
	ErrCategoryCycle = errs.New(errs.KindValidation, "CATEGORY_CYCLE",
		"Không thể chuyển danh mục vào bên trong chính nó hoặc danh mục con của nó")
	ErrDuplicateCategorySlug = errs.New(errs.KindConflict, "DUPLICATE_CATEGORY_SLUG",
		"Đường dẫn này đã được dùng cho danh mục khác")
	ErrDuplicateBrandSlug = errs.New(errs.KindConflict, "DUPLICATE_BRAND_SLUG",
		"Đường dẫn này đã được dùng cho thương hiệu khác")
	ErrCategoryHasChildren = errs.New(errs.KindConflict, "CATEGORY_HAS_CHILDREN",
		"Danh mục còn danh mục con, hãy chuyển hoặc xóa chúng trước")
	ErrCategoryHasProducts = errs.New(errs.KindConflict, "CATEGORY_HAS_PRODUCTS",
		"Danh mục còn sản phẩm (kể cả sản phẩm đã xóa), hãy chuyển chúng sang danh mục khác trước")
	ErrBrandHasProducts = errs.New(errs.KindConflict, "BRAND_HAS_PRODUCTS",
		"Thương hiệu còn sản phẩm (kể cả sản phẩm đã xóa), hãy chuyển chúng sang thương hiệu khác trước")

	// UNKNOWN_* là 404 cho tài nguyên nằm TRÊN ĐƯỜNG DẪN. Khác CATEGORY_NOT_FOUND /
	// BRAND_NOT_FOUND ở trên: hai mã đó là 422 và nghĩa là "dữ liệu gửi lên tham
	// chiếu một thứ không tồn tại" — frontend đang dựa vào nghĩa đó. Dùng lại
	// chúng cho đường dẫn sẽ trả 422 cho một URL sai, và đổi nghĩa là phá hợp đồng.
	ErrUnknownCategory = errs.New(errs.KindNotFound, "UNKNOWN_CATEGORY",
		"Không tìm thấy danh mục")
	ErrUnknownBrand = errs.New(errs.KindNotFound, "UNKNOWN_BRAND",
		"Không tìm thấy thương hiệu")

	// Biến thể (P1.2).
	ErrVariantRequired = errs.New(errs.KindValidation, "VARIANT_REQUIRED",
		"Sản phẩm phải có ít nhất một phiên bản")
	ErrInvalidVariantOptions = errs.New(errs.KindValidation, "INVALID_VARIANT_OPTIONS",
		"Tùy chọn phiên bản không hợp lệ: tối đa 10 cặp, tên và giá trị không được rỗng")
	ErrInvalidVariantStatus = errs.New(errs.KindValidation, "INVALID_VARIANT_STATUS",
		"Trạng thái phiên bản không hợp lệ")
	ErrNoActiveVariant = errs.New(errs.KindValidation, "NO_ACTIVE_VARIANT",
		"Sản phẩm đang bán phải còn ít nhất một phiên bản đang bán")
	ErrDuplicateVariantOptions = errs.New(errs.KindConflict, "DUPLICATE_VARIANT_OPTIONS",
		"Đã có phiên bản khác với đúng các tùy chọn này")
	ErrUnknownVariant = errs.New(errs.KindNotFound, "UNKNOWN_VARIANT",
		"Không tìm thấy phiên bản này của sản phẩm")

	// Thuộc tính động (P1.3). Lỗi validate GIÁ TRỊ thuộc tính của sản phẩm
	// không nằm ở đây mà là VALIDATION_FAILED kèm errors[] theo trường — xem
	// Field* trong attribute.go.
	ErrInvalidAttributeCode = errs.New(errs.KindValidation, "INVALID_ATTRIBUTE_CODE",
		"Mã thuộc tính chỉ gồm chữ thường không dấu, số và dấu gạch dưới")
	ErrAttributeNameInvalid = errs.New(errs.KindValidation, "ATTRIBUTE_NAME_INVALID",
		"Tên thuộc tính là bắt buộc và tối đa 100 ký tự")
	ErrInvalidAttributeType = errs.New(errs.KindValidation, "INVALID_ATTRIBUTE_TYPE",
		"Kiểu thuộc tính chỉ có thể là text, number, boolean hoặc enum")
	ErrInvalidAttributeOptions = errs.New(errs.KindValidation, "INVALID_ATTRIBUTE_OPTIONS",
		"Danh sách giá trị không hợp lệ: kiểu enum cần ít nhất một giá trị không trùng, kiểu khác không được có")
	ErrAttributeNotFound = errs.New(errs.KindValidation, "ATTRIBUTE_NOT_FOUND",
		"Thuộc tính không tồn tại")
	ErrDuplicateAttributeAssignment = errs.New(errs.KindValidation, "DUPLICATE_ATTRIBUTE_ASSIGNMENT",
		"Một thuộc tính chỉ được gán một lần cho mỗi danh mục")
	ErrDuplicateAttributeCode = errs.New(errs.KindConflict, "DUPLICATE_ATTRIBUTE_CODE",
		"Mã thuộc tính này đã tồn tại")
	ErrAttributeInUse = errs.New(errs.KindConflict, "ATTRIBUTE_IN_USE",
		"Thuộc tính đang được gán cho danh mục, hãy gỡ khỏi các danh mục trước khi xóa")
	ErrUnknownAttribute = errs.New(errs.KindNotFound, "UNKNOWN_ATTRIBUTE",
		"Không tìm thấy thuộc tính")

	// Media (P1.4).
	ErrUnsupportedImageType = errs.New(errs.KindValidation, "UNSUPPORTED_IMAGE_TYPE",
		"Chỉ nhận ảnh JPEG, PNG hoặc WebP")
	ErrImageTooLarge = errs.New(errs.KindTooLarge, "IMAGE_TOO_LARGE",
		"Ảnh phải lớn hơn 0 và không quá 10 MB")
	ErrInvalidImage = errs.New(errs.KindValidation, "INVALID_IMAGE",
		"File tải lên không phải ảnh JPEG, PNG hay WebP hợp lệ")
	ErrUploadNotFound = errs.New(errs.KindValidation, "UPLOAD_NOT_FOUND",
		"Chưa thấy file trên kho lưu trữ — hãy tải file lên trước khi xác nhận")
	ErrUnknownMedia = errs.New(errs.KindNotFound, "UNKNOWN_MEDIA",
		"Không tìm thấy ảnh")

	// Tài khoản và phiên (P2.1).
	ErrInvalidEmail = errs.New(errs.KindValidation, "INVALID_EMAIL",
		"Địa chỉ email không hợp lệ")
	ErrWeakPassword = errs.New(errs.KindValidation, "WEAK_PASSWORD",
		"Mật khẩu phải dài từ 8 đến 128 ký tự")
	ErrFullNameInvalid = errs.New(errs.KindValidation, "FULL_NAME_INVALID",
		"Họ tên là bắt buộc và tối đa 100 ký tự")
	ErrEmailTaken = errs.New(errs.KindConflict, "EMAIL_TAKEN",
		"Email này đã được dùng để đăng ký")
	// INVALID_CREDENTIALS dùng CHUNG cho "không có email này" và "sai mật khẩu":
	// hai lỗi khác nhau thì kẻ dò biết ngay email nào đã đăng ký.
	ErrInvalidCredentials = errs.New(errs.KindUnauthenticated, "INVALID_CREDENTIALS",
		"Email hoặc mật khẩu không đúng")
	ErrInvalidRefreshToken = errs.New(errs.KindUnauthenticated, "INVALID_REFRESH_TOKEN",
		"Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại")
	ErrAccountDisabled = errs.New(errs.KindForbidden, "ACCOUNT_DISABLED",
		"Tài khoản đã bị khóa")
	ErrRateLimited = errs.New(errs.KindRateLimited, "RATE_LIMITED",
		"Bạn thao tác quá nhanh, vui lòng thử lại sau ít phút")

	// Phân quyền (P2.2).
	ErrForbidden = errs.New(errs.KindForbidden, "FORBIDDEN",
		"Bạn không có quyền thực hiện thao tác này")
	ErrInvalidRoleCode = errs.New(errs.KindValidation, "INVALID_ROLE_CODE",
		"Mã vai trò chỉ gồm chữ thường không dấu, số và dấu gạch dưới")
	ErrRoleNameInvalid = errs.New(errs.KindValidation, "ROLE_NAME_INVALID",
		"Tên vai trò là bắt buộc và tối đa 100 ký tự")
	ErrUnknownPermission = errs.New(errs.KindValidation, "UNKNOWN_PERMISSION",
		"Có quyền không tồn tại trong danh sách quyền của hệ thống")
	ErrSystemRoleImmutable = errs.New(errs.KindValidation, "SYSTEM_ROLE_IMMUTABLE",
		"Không thể sửa hoặc xóa vai trò hệ thống")
	ErrCannotChangeOwnRoles = errs.New(errs.KindValidation, "CANNOT_CHANGE_OWN_ROLES",
		"Không thể tự thay đổi vai trò của chính mình")
	ErrRoleNotFound = errs.New(errs.KindValidation, "ROLE_NOT_FOUND",
		"Vai trò không tồn tại")
	ErrDuplicateRoleCode = errs.New(errs.KindConflict, "DUPLICATE_ROLE_CODE",
		"Mã vai trò này đã tồn tại")
	ErrRoleInUse = errs.New(errs.KindConflict, "ROLE_IN_USE",
		"Vai trò đang được gán cho người dùng, hãy gỡ trước khi xóa")
	ErrUnknownRole = errs.New(errs.KindNotFound, "UNKNOWN_ROLE",
		"Không tìm thấy vai trò")
	ErrUnknownUser = errs.New(errs.KindNotFound, "UNKNOWN_USER",
		"Không tìm thấy người dùng")

	// OTP qua email (P2.3). INVALID_OTP dùng CHUNG cho mã sai, hết hạn, đã
	// dùng, đã sai quá 5 lần, và email không tồn tại khi đặt lại mật khẩu —
	// tách ra thì kẻ dò biết được email nào có tài khoản.
	ErrInvalidOTP = errs.New(errs.KindValidation, "INVALID_OTP",
		"Mã xác nhận không đúng hoặc đã hết hạn")
	ErrEmailAlreadyVerified = errs.New(errs.KindConflict, "EMAIL_ALREADY_VERIFIED",
		"Email đã được xác minh")

	// Sổ địa chỉ (P2.4). Địa chỉ của người khác cũng là UNKNOWN_ADDRESS (404)
	// chứ không phải 403 — 403 là thừa nhận id đó tồn tại.
	ErrAddressLimitReached = errs.New(errs.KindValidation, "ADDRESS_LIMIT_REACHED",
		"Sổ địa chỉ đã đủ 10 địa chỉ, hãy xóa bớt trước khi thêm")
	ErrUnknownAddress = errs.New(errs.KindNotFound, "UNKNOWN_ADDRESS",
		"Không tìm thấy địa chỉ")

	// Kho và tồn kho (P3.1).
	ErrInvalidLocationCode = errs.New(errs.KindValidation, "INVALID_LOCATION_CODE",
		"Mã kho chỉ gồm chữ thường không dấu, số và dấu gạch ngang, tối đa 32 ký tự")
	ErrLocationNameInvalid = errs.New(errs.KindValidation, "LOCATION_NAME_INVALID",
		"Tên kho là bắt buộc và tối đa 100 ký tự")
	ErrInvalidLocationKind = errs.New(errs.KindValidation, "INVALID_LOCATION_KIND",
		"Loại kho phải là warehouse hoặc showroom")
	ErrLocationAddressRequired = errs.New(errs.KindValidation, "LOCATION_ADDRESS_REQUIRED",
		"Showroom phải có địa chỉ (tối đa 255 ký tự)")
	ErrDuplicateLocationCode = errs.New(errs.KindConflict, "DUPLICATE_LOCATION_CODE",
		"Mã kho này đã tồn tại")
	ErrUnknownLocation = errs.New(errs.KindNotFound, "UNKNOWN_LOCATION",
		"Không tìm thấy kho")
	ErrLocationNotFound = errs.New(errs.KindValidation, "LOCATION_NOT_FOUND",
		"Kho không tồn tại")
	ErrLocationInactive = errs.New(errs.KindValidation, "LOCATION_INACTIVE",
		"Kho đã ngừng hoạt động, không nhập hàng vào được")
	ErrVariantNotFound = errs.New(errs.KindValidation, "VARIANT_NOT_FOUND",
		"Phiên bản sản phẩm không tồn tại")
	ErrInvalidMovementKind = errs.New(errs.KindValidation, "INVALID_MOVEMENT_KIND",
		"Loại thao tác phải là receipt, adjustment hoặc count")
	ErrInvalidQuantity = errs.New(errs.KindValidation, "INVALID_QUANTITY",
		"Số lượng không hợp lệ")
	ErrStockReasonRequired = errs.New(errs.KindValidation, "STOCK_REASON_REQUIRED",
		"Điều chỉnh và kiểm kê phải ghi lý do (tối đa 500 ký tự; mã chứng từ tối đa 100)")
	ErrInsufficientStock = errs.New(errs.KindConflict, "INSUFFICIENT_STOCK",
		"Không đủ tồn kho — số tồn không được thấp hơn phần đã giữ cho đơn hàng")
	ErrIdempotencyKeyReused = errs.New(errs.KindValidation, "IDEMPOTENCY_KEY_REUSED",
		"Idempotency-Key này đã dùng cho một yêu cầu khác")
	ErrInvalidIdempotencyKey = errs.New(errs.KindInvalid, "INVALID_IDEMPOTENCY_KEY",
		"Idempotency-Key tối đa 100 ký tự")
	ErrInvalidMovementFilter = errs.New(errs.KindInvalid, "INVALID_MOVEMENT_FILTER",
		"variant_id, location_id, cursor phải là UUID; limit phải là số")

	// Giữ chỗ (P3.2).
	ErrInvalidReservation = errs.New(errs.KindValidation, "INVALID_RESERVATION",
		"Yêu cầu giữ hàng không hợp lệ: 1–50 dòng, không trùng phiên bản, mỗi dòng 1–1000, ttl 60–86400 giây, mã đơn tối đa 100 ký tự")
	ErrReservationRefReused = errs.New(errs.KindValidation, "RESERVATION_REF_REUSED",
		"Mã đơn này đã giữ hàng cho một danh sách khác")
	ErrUnknownReservation = errs.New(errs.KindNotFound, "UNKNOWN_RESERVATION",
		"Không tìm thấy lượt giữ hàng")
	ErrReservationNotActive = errs.New(errs.KindConflict, "RESERVATION_NOT_ACTIVE",
		"Lượt giữ hàng đã kết thúc (đã xuất, đã nhả hoặc đã hết hạn)")

	ErrInvalidPagination = errs.New(errs.KindInvalid, "INVALID_PAGINATION",
		"Tham số phân trang không hợp lệ")

	ErrInvalidSort = errs.New(errs.KindInvalid, "INVALID_SORT",
		"Giá trị sắp xếp không hợp lệ")
	ErrPageTooDeep = errs.New(errs.KindInvalid, "PAGE_TOO_DEEP",
		"Không hỗ trợ truy cập quá sâu vào danh sách")
)
