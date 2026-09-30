package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

const (
	minPasswordLen = 8
	// Trần 128 ký tự: argon2 băm được chuỗi dài tùy ý, và đó chính là vấn đề —
	// một "mật khẩu" 10 MB gửi liên tục là đòn DoS rẻ vào CPU.
	maxPasswordLen = 128
	maxEmailLen    = 254
)

// User là tài khoản. PasswordHash là chuỗi PHC của argon2id — domain không
// băm (không được import thư viện mật mã ngoài danh sách trắng), việc đó là của
// adapter PasswordHasher ở tầng use case.
type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	FullName        string
	Status          UserStatus
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NormalizeEmail cắt khoảng trắng và chuyển chữ thường, rồi kiểm cú pháp.
//
// Chữ thường TOÀN BỘ, kể cả phần trước @: về lý thuyết RFC 5321 cho phép phần
// đó phân biệt hoa thường, nhưng không nhà cung cấp phổ biến nào làm vậy, và
// để "An@x.vn" với "an@x.vn" thành hai tài khoản thì khách quên mình đã đăng ký
// bằng kiểu nào. Cột users.email có ràng buộc CHECK cùng quy tắc.
func NormalizeEmail(s string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(s))
	if e == "" || len(e) > maxEmailLen {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(e)
	// ParseAddress nhận cả "Tên <a@x.vn>" — chỉ chấp nhận khi nó trả lại đúng
	// chuỗi đã nhập, tức là địa chỉ trần.
	if err != nil || addr.Address != e || !strings.Contains(e[strings.LastIndex(e, "@")+1:], ".") {
		return "", ErrInvalidEmail
	}
	return e, nil
}

// CheckPassword kiểm độ dài — gọi TRƯỚC khi băm.
func CheckPassword(plain string) error {
	n := utf8.RuneCountInString(plain)
	if n < minPasswordLen || n > maxPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// NewUser dựng tài khoản mới. passwordHash là mật khẩu ĐÃ băm.
func NewUser(email, passwordHash, fullName string) (*User, error) {
	e, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	name, err := normalizeTaxonomyName(fullName, ErrFullNameInvalid)
	if err != nil {
		return nil, err
	}
	if passwordHash == "" {
		return nil, ErrWeakPassword
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &User{ID: id, Email: e, PasswordHash: passwordHash, FullName: name,
		Status: UserActive, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename đổi họ tên hiển thị — cùng luật với lúc đăng ký.
func (u *User) Rename(fullName string) error {
	name, err := normalizeTaxonomyName(fullName, ErrFullNameInvalid)
	if err != nil {
		return err
	}
	u.FullName = name
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) CanLogin() error {
	if u.Status != UserActive {
		return ErrAccountDisabled
	}
	return nil
}
