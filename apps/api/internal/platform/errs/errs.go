// Package errs định nghĩa kiểu lỗi dùng chung cho toàn hệ thống.
//
// Đây là package kernel: chỉ phụ thuộc thư viện chuẩn và KHÔNG import net/http.
// Nhờ vậy tầng domain được phép import nó mà không phá vỡ quy tắc chiều phụ thuộc.
package errs

import (
	"errors"
	"fmt"
)

// Kind phân loại lỗi theo ngữ nghĩa, độc lập với giao thức truyền tải.
// Việc map Kind sang mã HTTP là trách nhiệm của tầng httpx.
type Kind uint8

const (
	// KindInternal cố ý là giá trị 0: quên gán Kind thì mặc định thành 500.
	KindInternal Kind = iota
	KindInvalid
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindConflict
	KindValidation
	KindRateLimited
	KindUnavailable
	// KindTooLarge phải nằm CUỐI khối iota. Chèn vào giữa sẽ đổi giá trị số
	// của mọi Kind đứng sau nó.
	KindTooLarge
)

// FieldError mô tả một lỗi ở cấp trường dữ liệu.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error là kiểu lỗi chuẩn của hệ thống.
//
// Code là hợp đồng với client, một khi công bố thì không đổi.
// Message là tiếng Việt, hiển thị được cho người dùng cuối.
// cause là lỗi gốc, chỉ dùng để ghi log — không bao giờ lộ ra response.
type Error struct {
	Kind    Kind `json:"-"`
	Code    string
	Message string
	Fields  []FieldError
	cause   error
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Wrap(cause error, kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message, cause: cause}
}

func Validation(fields ...FieldError) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    "VALIDATION_FAILED",
		Message: "Dữ liệu không hợp lệ",
		Fields:  fields,
	}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is so sánh theo Code và Kind thay vì theo con trỏ, nhờ vậy errors.Is vẫn đúng
// khi lỗi được tạo lại ở tầng khác hoặc đã bị bọc nhiều lần.
//
// Phải so cả Kind: nếu chỉ so Code thì một lỗi Wrap nhầm Kind vẫn khớp với
// sentinel của domain, và client nhận sai mã HTTP trong khi errors.Is báo đúng.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.Kind == t.Kind
}

// From trích *Error ra khỏi chuỗi lỗi.
// Lỗi không xác định được quy về lỗi nội bộ với thông điệp trung tính —
// không bao giờ để nội dung lỗi gốc lọt ra ngoài.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return New(KindInternal, "INTERNAL_ERROR", "Đã có lỗi xảy ra")
}
