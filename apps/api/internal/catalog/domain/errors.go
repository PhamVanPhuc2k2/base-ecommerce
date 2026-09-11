package domain

import "base-ecommerce/api/internal/platform/errs"

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

	ErrInvalidSort = errs.New(errs.KindInvalid, "INVALID_SORT",
		"Giá trị sắp xếp không hợp lệ")
	ErrPageTooDeep = errs.New(errs.KindInvalid, "PAGE_TOO_DEEP",
		"Không hỗ trợ truy cập quá sâu vào danh sách")
)
