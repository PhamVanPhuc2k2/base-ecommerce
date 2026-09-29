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

	ErrInvalidPagination = errs.New(errs.KindInvalid, "INVALID_PAGINATION",
		"Tham số phân trang không hợp lệ")

	ErrInvalidSort = errs.New(errs.KindInvalid, "INVALID_SORT",
		"Giá trị sắp xếp không hợp lệ")
	ErrPageTooDeep = errs.New(errs.KindInvalid, "PAGE_TOO_DEEP",
		"Không hỗ trợ truy cập quá sâu vào danh sách")
)
