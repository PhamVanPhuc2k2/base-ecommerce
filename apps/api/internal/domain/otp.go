package domain

import (
	"time"

	"github.com/google/uuid"
)

// OTPPurpose ràng buộc một mã vào đúng một việc: mã xác minh email không đặt
// lại được mật khẩu (mục đích nằm trong thông điệp HMAC, và trong truy vấn).
type OTPPurpose string

const (
	OTPVerifyEmail   OTPPurpose = "verify_email"
	OTPResetPassword OTPPurpose = "reset_password"
)

// Luật của một mã — đặc tả P2.3 mục 2.3.
const (
	OTPTTL         = 10 * time.Minute
	OTPMaxAttempts = 5
	OTPCooldown    = 60 * time.Second
	OTPDigits      = 6
)

// OTP là một mã đã phát. CodeHash là HMAC của mã — mã thô chỉ nằm trong thư.
type OTP struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Purpose    OTPPurpose
	CodeHash   []byte
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

func NewOTP(userID uuid.UUID, purpose OTPPurpose, codeHash []byte, now time.Time) *OTP {
	return &OTP{ID: uuid.Must(uuid.NewV7()), UserID: userID, Purpose: purpose, CodeHash: codeHash,
		ExpiresAt: now.Add(OTPTTL), CreatedAt: now}
}

// Usable: chưa dùng, chưa hết hạn, chưa sai đủ số lần. Hết một trong ba là
// mã chết — người dùng phải xin mã mới.
func (o *OTP) Usable(now time.Time) bool {
	return o.ConsumedAt == nil && now.Before(o.ExpiresAt) && o.Attempts < OTPMaxAttempts
}

// CooldownLeft là thời gian còn phải chờ trước khi được phát mã mới cùng mục
// đích, tính từ mã phát gần nhất (last có thể nil).
func CooldownLeft(last *OTP, now time.Time) time.Duration {
	if last == nil {
		return 0
	}
	if left := last.CreatedAt.Add(OTPCooldown).Sub(now); left > 0 {
		return left
	}
	return 0
}
