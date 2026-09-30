package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"time"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// Verification phát và kiểm mã OTP qua email: xác minh email, quên / đặt lại
// mật khẩu. Đặc tả P2.3.
type Verification struct {
	tx      TxManager
	users   UserRepository
	tokens  RefreshTokenRepository
	otps    OTPRepository
	emails  EmailRepository
	events  EventPublisher
	hasher  PasswordHasher
	limiter RateLimiter
	// key là khóa HMAC cho mã — suy từ JWT_SECRET, xem NewVerification.
	key []byte
}

// NewVerification nhận secret của ứng dụng và SUY ra khóa riêng cho OTP:
// HMAC(secret, nhãn). Dùng thẳng secret làm khóa HMAC cho cả JWT lẫn OTP thì
// hai cơ chế chia nhau một khóa; khóa con có nhãn tách chúng ra mà không bắt
// ai quản lý thêm một biến môi trường (đặc tả P2.3 mục 2.2).
func NewVerification(tx TxManager, users UserRepository, tokens RefreshTokenRepository, otps OTPRepository,
	emails EmailRepository, events EventPublisher, hasher PasswordHasher, limiter RateLimiter, secret []byte) *Verification {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("base-ecommerce/otp/v1"))
	return &Verification{tx: tx, users: users, tokens: tokens, otps: otps, emails: emails, events: events,
		hasher: hasher, limiter: limiter, key: m.Sum(nil)}
}

// RequestEmailVerification gửi (lại) mã xác minh cho người đang đăng nhập.
func (v *Verification) RequestEmailVerification(ctx context.Context, userID uuid.UUID) error {
	return v.tx.Run(ctx, func(ctx context.Context) error {
		u, err := v.users.ByIDForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if u == nil || u.CanLogin() != nil {
			return domain.ErrInvalidCredentials
		}
		if u.EmailVerifiedAt != nil {
			return domain.ErrEmailAlreadyVerified
		}
		return v.issue(ctx, u, domain.OTPVerifyEmail)
	})
}

// VerifyEmail kiểm mã và đánh dấu email đã xác minh.
func (v *Verification) VerifyEmail(ctx context.Context, userID uuid.UUID, code string) (*domain.User, error) {
	// Theo người dùng: 5 lần sai đã giết mã, nhưng không có trần này thì kẻ
	// cầm token có thể xin mã mới mỗi phút và thử 5 lần mỗi mã, mãi mãi.
	if err := limitAll(ctx, v.limiter, "auth:verify:user:"+userID.String()); err != nil {
		return nil, err
	}
	var (
		u      *domain.User
		failed bool
	)
	err := v.tx.Run(ctx, func(ctx context.Context) error {
		failed = false
		var err error
		if u, err = v.users.ByIDForUpdate(ctx, userID); err != nil {
			return err
		}
		if u == nil || u.CanLogin() != nil {
			return domain.ErrInvalidCredentials
		}
		if u.EmailVerifiedAt != nil {
			return domain.ErrEmailAlreadyVerified
		}
		now := time.Now().UTC()
		ok, err := v.check(ctx, u.ID, domain.OTPVerifyEmail, code, now)
		if err != nil {
			return err
		}
		if !ok {
			// ⚠️ nil để COMMIT lần sai vừa ghi — cùng bẫy với phát hiện dùng
			// lại refresh token (P2.1). Lỗi trả sau Run.
			failed = true
			return nil
		}
		u.EmailVerifiedAt = &now
		return v.users.MarkEmailVerified(ctx, u.ID, now)
	})
	if err != nil {
		return nil, err
	}
	if failed {
		return nil, domain.ErrInvalidOTP
	}
	return u, nil
}

// ForgotPassword kiểm cú pháp và rate limit, rồi trả về job phát mã để người
// gọi chạy NỀN sau khi đã trả lời client (đặc tả P2.3 mục 2.4).
//
// Vì sao tách job: phát mã là một transaction ghi bốn dòng rồi commit — đo
// được ~20 ms, so với ~0,8 ms khi email không tồn tại. Chạy đồng bộ thì thời
// gian phản hồi tự khai ra email nào đã đăng ký, dù mã HTTP như nhau. Tách ra
// thì mọi email đúng cú pháp trả lời sau đúng cùng một lượng việc.
//
// Job trả lỗi chỉ khi hạ tầng hỏng; "không có tài khoản" hay "đang chờ 60
// giây" đều là nil.
func (v *Verification) ForgotPassword(ctx context.Context, rawEmail, clientIP string) (func(context.Context) error, error) {
	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return nil, err
	}
	if err := limitAll(ctx, v.limiter, "auth:forgot:ip:"+clientIP, "auth:forgot:email:"+email); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return v.sendResetCode(ctx, email) }, nil
}

func (v *Verification) sendResetCode(ctx context.Context, email string) error {
	u, err := v.users.ByEmail(ctx, email)
	if err != nil || u == nil || u.CanLogin() != nil {
		return err
	}
	err = v.tx.Run(ctx, func(ctx context.Context) error {
		if u, err = v.users.ByIDForUpdate(ctx, u.ID); err != nil || u == nil {
			return err
		}
		return v.issue(ctx, u, domain.OTPResetPassword)
	})
	// Đang trong 60 giây chờ: im lặng. 429 ở đây là xác nhận email có thật.
	if _, cooling := errors.AsType[*RateLimitError](err); cooling {
		return nil
	}
	return err
}

type ResetPasswordInput struct {
	Email       string
	Code        string
	NewPassword string
	ClientIP    string
}

// ResetPassword đổi mật khẩu bằng mã qua thư, rồi cắt mọi phiên.
func (v *Verification) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	// Email sai cú pháp → INVALID_OTP chứ không phải INVALID_EMAIL: với bước
	// này mọi thất bại về danh tính trông như nhau.
	email, emailErr := domain.NormalizeEmail(in.Email)
	if emailErr != nil {
		email = in.Email
	}
	if err := limitAll(ctx, v.limiter, "auth:reset:ip:"+in.ClientIP, "auth:reset:email:"+email); err != nil {
		return err
	}
	if err := domain.CheckPassword(in.NewPassword); err != nil {
		return err
	}
	// Băm TRƯỚC khi tra email và NGOÀI transaction: argon2 tốn vài chục
	// mili-giây — tra trước rồi mới băm thì email không tồn tại trả lời nhanh
	// hơn hẳn và tự khai ra.
	hash, err := v.hasher.Hash(in.NewPassword)
	if err != nil {
		return err
	}
	if emailErr != nil {
		return domain.ErrInvalidOTP
	}
	u, err := v.users.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	if u == nil || u.CanLogin() != nil {
		return domain.ErrInvalidOTP
	}
	var failed bool
	err = v.tx.Run(ctx, func(ctx context.Context) error {
		failed = false
		if _, err := v.users.ByIDForUpdate(ctx, u.ID); err != nil {
			return err
		}
		now := time.Now().UTC()
		ok, err := v.check(ctx, u.ID, domain.OTPResetPassword, in.Code, now)
		if err != nil {
			return err
		}
		if !ok {
			failed = true // commit lần sai — xem VerifyEmail
			return nil
		}
		if err := v.users.UpdatePassword(ctx, u.ID, hash, now); err != nil {
			return err
		}
		// Nhận được mã qua thư là đã chứng minh sở hữu hộp thư.
		if err := v.users.MarkEmailVerified(ctx, u.ID, now); err != nil {
			return err
		}
		return v.tokens.RevokeAllForUser(ctx, u.ID, now)
	})
	if err != nil {
		return err
	}
	if failed {
		return domain.ErrInvalidOTP
	}
	return nil
}

// issue phát mã mới cho mục đích purpose và xếp thư chứa mã. PHẢI chạy trong
// transaction, sau khi đã khóa dòng người dùng (ByIDForUpdate) — khóa đó xếp
// hàng hai request song song, để cả hai không cùng lọt qua 60 giây chờ.
func (v *Verification) issue(ctx context.Context, u *domain.User, purpose domain.OTPPurpose) error {
	now := time.Now().UTC()
	last, err := v.otps.Latest(ctx, u.ID, purpose)
	if err != nil {
		return err
	}
	if left := domain.CooldownLeft(last, now); left > 0 {
		return &RateLimitError{RetryAfter: left}
	}
	code, err := newOTPCode()
	if err != nil {
		return err
	}
	if err := v.otps.KillActive(ctx, u.ID, purpose, now); err != nil {
		return err
	}
	if err := v.otps.Insert(ctx, domain.NewOTP(u.ID, purpose, v.hash(u.ID, purpose, code), now)); err != nil {
		return err
	}
	subject, body := otpEmail(u.FullName, purpose, code)
	email, ev := domain.NewOutboundEmail(u.Email, subject, body)
	if err := v.emails.Insert(ctx, email); err != nil {
		return err
	}
	return v.events.Publish(ctx, ev)
}

// check so mã với mã mới nhất còn sống. Sai → tăng bộ đếm (người gọi phải
// COMMIT). Đúng → mã chết ngay, không dùng được lần hai.
func (v *Verification) check(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose, code string, now time.Time) (bool, error) {
	if !validOTPFormat(code) {
		// Không tính là một lần thử: chuỗi không phải 6 chữ số thì không thể
		// trùng mã nào, kẻ dò chẳng học được gì.
		return false, nil
	}
	o, err := v.otps.Latest(ctx, userID, purpose)
	if err != nil {
		return false, err
	}
	if o == nil || !o.Usable(now) {
		return false, nil
	}
	if !hmac.Equal(o.CodeHash, v.hash(userID, purpose, code)) {
		return false, v.otps.RecordFailure(ctx, o.ID)
	}
	return true, v.otps.Consume(ctx, o.ID, now)
}

// hash: HMAC(khóa, mục đích | user | mã). Mục đích nằm trong thông điệp nên
// mã xác minh email không khớp khi đem đi đặt lại mật khẩu, kể cả nếu truy
// vấn có sai sót trả nhầm dòng.
func (v *Verification) hash(userID uuid.UUID, purpose domain.OTPPurpose, code string) []byte {
	m := hmac.New(sha256.New, v.key)
	_, _ = fmt.Fprintf(m, "%s|%s|%s", purpose, userID, code) // hash.Hash.Write không bao giờ trả lỗi
	return m.Sum(nil)
}

// newOTPCode: 6 chữ số phân bố ĐỀU từ crypto/rand. rand.Int tự loại bỏ phần
// lệch — lấy 3 byte ngẫu nhiên rồi % 10⁶ thì các mã nhỏ xuất hiện nhiều hơn.
func newOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", domain.OTPDigits, n.Int64()), nil
}

func validOTPFormat(code string) bool {
	if len(code) != domain.OTPDigits {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// otpEmail soạn thư. Mã KHÔNG nằm trong tiêu đề: tiêu đề hiện trên màn hình
// khóa điện thoại, ai cầm máy cũng đọc được.
func otpEmail(name string, purpose domain.OTPPurpose, code string) (subject, body string) {
	minutes := int(domain.OTPTTL / time.Minute)
	switch purpose {
	case domain.OTPResetPassword:
		return "Đặt lại mật khẩu", fmt.Sprintf(
			"Xin chào %s,\n\nMã đặt lại mật khẩu của bạn là: %s\n\nMã có hiệu lực trong %d phút và chỉ dùng được một lần.\n"+
				"Nếu bạn không yêu cầu đặt lại mật khẩu, hãy bỏ qua thư này — mật khẩu của bạn không thay đổi.\n",
			name, code, minutes)
	default:
		return "Xác minh địa chỉ email", fmt.Sprintf(
			"Xin chào %s,\n\nMã xác minh email của bạn là: %s\n\nMã có hiệu lực trong %d phút và chỉ dùng được một lần.\n"+
				"Nếu bạn không đăng ký tài khoản, hãy bỏ qua thư này.\n",
			name, code, minutes)
	}
}

// Mailing gửi thư trong hàng đợi — worker gọi khi nhận sự kiện email.queued.
type Mailing struct {
	tx     TxManager
	emails EmailRepository
	sender MailSender
}

func NewMailing(tx TxManager, emails EmailRepository, sender MailSender) *Mailing {
	return &Mailing{tx: tx, emails: emails, sender: sender}
}

// Deliver gửi thư id, đúng một lần trong điều kiện bình thường. sent = false
// khi thư đã được gửi từ trước (bản trùng của sự kiện).
//
// Gửi SMTP nằm TRONG transaction đang giữ khóa dòng thư — cố ý: bản trùng
// của sự kiện (giao hàng at-least-once) tới worker khác thì chờ khóa, rồi thấy
// sent_at và bỏ qua. Cái giá: commit hỏng SAU khi gửi xong thì lần thử lại gửi
// thêm một thư — trùng một thư chấp nhận được, mất thư thì không (đặc tả P2.3
// mục 2.1).
func (m *Mailing) Deliver(ctx context.Context, id uuid.UUID) (sent bool, err error) {
	err = m.tx.Run(ctx, func(ctx context.Context) error {
		sent = false
		e, err := m.emails.ByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if e == nil {
			return domain.ErrUnknownEmail
		}
		if e.SentAt != nil {
			return nil
		}
		if err := m.sender.Send(ctx, e.To, e.Subject, e.Body); err != nil {
			return err
		}
		sent = true
		return m.emails.MarkSent(ctx, e.ID, time.Now().UTC())
	})
	return sent && err == nil, err
}
