package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// Rate limit cho endpoint xác thực: 5/phút theo IP VÀ theo email (thiết kế 03
// mục 7). Chỉ theo IP thì mạng công ty chung NAT bị chặn oan; chỉ theo email
// thì kẻ dò đổi email là thoát.
const (
	authLimit  = 5
	authWindow = time.Minute
)

// RateLimitError mang thời gian phải chờ tới delivery (header Retry-After), và
// Unwrap về domain.ErrRateLimited để httpx map ra đúng 429.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s (chờ %s)", domain.ErrRateLimited.Error(), e.RetryAfter)
}
func (e *RateLimitError) Unwrap() error { return domain.ErrRateLimited }

// Session là kết quả của đăng ký / đăng nhập / refresh.
type Session struct {
	User             *domain.User
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type Auth struct {
	tx         TxManager
	users      UserRepository
	tokens     RefreshTokenRepository
	hasher     PasswordHasher
	issuer     TokenIssuer
	limiter    RateLimiter
	refreshTTL time.Duration
	// dummyHash: băm sẵn một mật khẩu giả lúc khởi động. Đăng nhập với email
	// KHÔNG tồn tại vẫn chạy argon2 trên nó — không thì phản hồi nhanh hơn
	// hàng chục mili-giây và tự khai ra email nào có thật (đặc tả P2.1 mục 2.2).
	dummyHash string
}

func NewAuth(tx TxManager, users UserRepository, tokens RefreshTokenRepository,
	hasher PasswordHasher, issuer TokenIssuer, limiter RateLimiter, refreshTTL time.Duration) (*Auth, error) {
	dummy, err := hasher.Hash("mat-khau-gia-de-can-bang-thoi-gian")
	if err != nil {
		return nil, err
	}
	return &Auth{tx: tx, users: users, tokens: tokens, hasher: hasher, issuer: issuer,
		limiter: limiter, refreshTTL: refreshTTL, dummyHash: dummy}, nil
}

func (a *Auth) limit(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if ok, wait := a.limiter.Allow(ctx, k, authLimit, authWindow); !ok {
			return &RateLimitError{RetryAfter: wait}
		}
	}
	return nil
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
	ClientIP string
}

func (a *Auth) Register(ctx context.Context, in RegisterInput) (*Session, error) {
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	if err := a.limit(ctx, "auth:register:ip:"+in.ClientIP, "auth:register:email:"+email); err != nil {
		return nil, err
	}
	if err := domain.CheckPassword(in.Password); err != nil {
		return nil, err
	}
	// Băm NGOÀI transaction: argon2 tốn vài chục mili-giây CPU, không giữ một
	// kết nối database trong lúc đó.
	hash, err := a.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	u, err := domain.NewUser(email, hash, in.FullName)
	if err != nil {
		return nil, err
	}
	var s *Session
	if err := a.tx.Run(ctx, func(ctx context.Context) error {
		if err := a.users.Insert(ctx, u); err != nil {
			return err
		}
		s, err = a.startSession(ctx, u, uuid.Must(uuid.NewV7()))
		return err
	}); err != nil {
		return nil, err
	}
	return s, nil
}

type LoginInput struct {
	Email    string
	Password string
	ClientIP string
}

func (a *Auth) Login(ctx context.Context, in LoginInput) (*Session, error) {
	// Email sai cú pháp cũng trả INVALID_CREDENTIALS chứ không phải
	// INVALID_EMAIL — với đăng nhập, mọi thất bại trông như nhau.
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		email = in.Email
	}
	if err := a.limit(ctx, "auth:login:ip:"+in.ClientIP, "auth:login:email:"+email); err != nil {
		return nil, err
	}
	var u *domain.User
	if err == nil {
		if u, err = a.users.ByEmail(ctx, email); err != nil {
			return nil, err
		}
	}
	hash := a.dummyHash
	if u != nil {
		hash = u.PasswordHash
	}
	// LUÔN chạy Verify, kể cả khi không có tài khoản — xem dummyHash.
	match, err := a.hasher.Verify(in.Password, hash)
	if err != nil {
		return nil, err
	}
	if u == nil || !match {
		return nil, domain.ErrInvalidCredentials
	}
	// Kiểm khóa tài khoản SAU khi mật khẩu đúng: báo "đã khóa" cho người không
	// biết mật khẩu là cho họ biết email này tồn tại.
	if err := u.CanLogin(); err != nil {
		return nil, err
	}
	var s *Session
	if err := a.tx.Run(ctx, func(ctx context.Context) error {
		s, err = a.startSession(ctx, u, uuid.Must(uuid.NewV7()))
		return err
	}); err != nil {
		return nil, err
	}
	return s, nil
}

// Refresh đổi refresh token lấy cặp token mới (xoay vòng).
func (a *Auth) Refresh(ctx context.Context, raw, clientIP string) (*Session, error) {
	if err := a.limit(ctx, "auth:refresh:ip:"+clientIP); err != nil {
		return nil, err
	}
	var (
		s      *Session
		reused bool
	)
	err := a.tx.Run(ctx, func(ctx context.Context) error {
		reused = false
		now := time.Now().UTC()
		tok, err := a.tokens.ByHashForUpdate(ctx, hashToken(raw))
		if err != nil {
			return err
		}
		if tok == nil {
			return domain.ErrInvalidRefreshToken
		}
		if tok.UsedAt != nil || tok.RevokedAt != nil {
			// DÙNG LẠI một token đã đổi (hoặc đã thu hồi): hoặc kẻ trộm hoặc chủ
			// thật đang cầm bản sao — không phân biệt được, nên cắt CẢ family.
			//
			// ⚠️ Trả nil để transaction COMMIT lệnh thu hồi; lỗi báo cho client
			// trả SAU khi Run xong. Trả lỗi ngay tại đây thì TxManager rollback
			// và lệnh thu hồi biến mất — phép phát hiện thành vô dụng.
			reused = true
			return a.tokens.RevokeFamily(ctx, tok.FamilyID, now)
		}
		if now.After(tok.ExpiresAt) {
			return domain.ErrInvalidRefreshToken
		}
		u, err := a.users.ByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		if u == nil || u.CanLogin() != nil {
			reused = true // tài khoản mất/bị khóa: cắt phiên, cùng đường với ở trên
			return a.tokens.RevokeFamily(ctx, tok.FamilyID, now)
		}
		if err := a.tokens.MarkUsed(ctx, tok.ID, now); err != nil {
			return err
		}
		s, err = a.startSession(ctx, u, tok.FamilyID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if reused {
		return nil, domain.ErrInvalidRefreshToken
	}
	return s, nil
}

// Logout thu hồi CẢ family của refresh token. Token lạ hay đã thu hồi thì vẫn
// coi là thành công — đăng xuất phải luôn "được", client không cần biết gì thêm.
func (a *Auth) Logout(ctx context.Context, raw string) error {
	return a.tx.Run(ctx, func(ctx context.Context) error {
		tok, err := a.tokens.ByHashForUpdate(ctx, hashToken(raw))
		if err != nil || tok == nil {
			return err
		}
		return a.tokens.RevokeFamily(ctx, tok.FamilyID, time.Now().UTC())
	})
}

func (a *Auth) Me(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	u, err := a.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.CanLogin() != nil {
		// Token còn hạn nhưng tài khoản đã mất/bị khóa: đối xử như chưa đăng nhập.
		return nil, domain.ErrInvalidCredentials
	}
	return u, nil
}

// startSession sinh refresh token mới trong family và access token đi kèm.
func (a *Auth) startSession(ctx context.Context, u *domain.User, family uuid.UUID) (*Session, error) {
	raw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rt := RefreshToken{
		ID: uuid.Must(uuid.NewV7()), FamilyID: family, UserID: u.ID,
		Hash: hashToken(raw), ExpiresAt: now.Add(a.refreshTTL), CreatedAt: now,
	}
	if err := a.tokens.Insert(ctx, rt); err != nil {
		return nil, err
	}
	// sid = family: một lần đăng nhập là một "phiên", xuyên suốt mọi lần refresh.
	access, accessExp, err := a.issuer.Issue(u.ID, family, now)
	if err != nil {
		return nil, err
	}
	return &Session{User: u, AccessToken: access, AccessExpiresAt: accessExp,
		RefreshToken: raw, RefreshExpiresAt: rt.ExpiresAt}, nil
}

// newRefreshToken: 32 byte ngẫu nhiên từ crypto/rand, base64url. 256 bit — đoán
// là bất khả thi, nên không cần băm chậm như mật khẩu; SHA-256 là đủ.
func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
