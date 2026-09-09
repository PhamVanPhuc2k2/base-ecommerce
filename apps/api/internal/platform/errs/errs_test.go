package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"base-ecommerce/api/internal/platform/errs"
	"github.com/stretchr/testify/require"
)

var errProductNotFound = errs.New(errs.KindNotFound, "PRODUCT_NOT_FOUND", "Không tìm thấy sản phẩm")

func TestError_ErrorsIs_KhopTheoCode(t *testing.T) {
	// Lỗi đi qua nhiều tầng và bị bọc lại vẫn phải nhận diện được.
	wrapped := fmt.Errorf("pgstore: %w", errProductNotFound)
	require.ErrorIs(t, wrapped, errProductNotFound)
}

func TestError_ErrorsIs_KhacCodeThiKhongKhop(t *testing.T) {
	other := errs.New(errs.KindNotFound, "BRAND_NOT_FOUND", "Không tìm thấy thương hiệu")
	require.NotErrorIs(t, other, errProductNotFound)
}

func TestError_ErrorsIs_KhacKindThiKhongKhop(t *testing.T) {
	// Cùng Code nhưng khác Kind phải KHÔNG khớp: nếu khớp thì errors.Is báo
	// "đây là lỗi không tìm thấy" trong khi client nhận mã HTTP của Kind kia.
	conflict := errs.Wrap(errors.New("db"), errs.KindConflict, "PRODUCT_NOT_FOUND", "Trùng")
	require.NotErrorIs(t, conflict, errProductNotFound)
}

func TestError_Unwrap_GiuLoiGoc(t *testing.T) {
	cause := errors.New("connection refused")
	e := errs.Wrap(cause, errs.KindUnavailable, "SERVICE_UNAVAILABLE", "Dịch vụ tạm thời gián đoạn")
	require.ErrorIs(t, e, cause)
	require.Contains(t, e.Error(), "connection refused")
}

func TestFrom_LoiLaThiTraVeInternal(t *testing.T) {
	e := errs.From(errors.New("boom"))
	require.Equal(t, errs.KindInternal, e.Kind)
	require.Equal(t, "INTERNAL_ERROR", e.Code)
	// Thông điệp trả cho người dùng KHÔNG được chứa nội dung lỗi gốc.
	require.NotContains(t, e.Message, "boom")
}

func TestFrom_TrichDuocErrorDaBoc(t *testing.T) {
	wrapped := fmt.Errorf("app: %w", errProductNotFound)
	e := errs.From(wrapped)
	require.Equal(t, "PRODUCT_NOT_FOUND", e.Code)
}

func TestFrom_NilThiVanTraVeErrorNoiBo(t *testing.T) {
	// WriteError gọi From trên mọi lỗi; From(nil) không được panic.
	e := errs.From(nil)
	require.NotNil(t, e)
	require.Equal(t, errs.KindInternal, e.Kind)
	require.Equal(t, "INTERNAL_ERROR", e.Code)
}

func TestValidation_GomNhieuLoiTruong(t *testing.T) {
	e := errs.Validation(
		errs.FieldError{Field: "price", Code: "REQUIRED", Message: "Giá là bắt buộc"},
		errs.FieldError{Field: "sku", Code: "TOO_LONG", Message: "SKU tối đa 64 ký tự"},
	)
	require.Equal(t, errs.KindValidation, e.Kind)
	require.Equal(t, "VALIDATION_FAILED", e.Code)
	require.Len(t, e.Fields, 2)
}

func TestKindInternal_LaGiaTriKhong(t *testing.T) {
	// Quên gán Kind thì phải mặc định thành lỗi nội bộ (500), không phải 200/404.
	var e errs.Error
	require.Equal(t, errs.KindInternal, e.Kind)
}

func TestKindTooLarge_NamCuoiKhoiIota(t *testing.T) {
	// Chèn Kind mới vào giữa sẽ đổi giá trị số của các Kind sau nó.
	require.Greater(t, uint8(errs.KindTooLarge), uint8(errs.KindUnavailable))
}
