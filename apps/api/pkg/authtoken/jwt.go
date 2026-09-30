// Package authtoken ký và kiểm access token JWT (HS256).
//
// Token CHỈ mang định danh: user id (sub) và id chuỗi phiên (sid). Không mang
// vai trò hay quyền — quyền tra mỗi request (P2.2) để gỡ quyền có hiệu lực ngay
// thay vì chờ token hết hạn. Đặc tả P2.1 mục 2.3.
package authtoken

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const issuer = "base-ecommerce"

// ErrInvalid gộp MỌI lý do token không dùng được (hết hạn, sai chữ ký, sai
// định dạng, sai thuật toán). Người gọi không cần — và không nên để client
// biết — token hỏng theo kiểu nào.
var ErrInvalid = errors.New("authtoken: token không hợp lệ")

type Claims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

func (i *Issuer) TTL() time.Duration { return i.ttl }

func (i *Issuer) Issue(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error) {
	exp := now.Add(i.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID.String(),
		"sid": sessionID.String(),
		"iss": issuer,
		"iat": now.Unix(),
		"exp": exp.Unix(),
	})
	s, err := tok.SignedString(i.secret)
	return s, exp, err
}

func (i *Issuer) Parse(raw string) (Claims, error) {
	var mc jwt.MapClaims
	_, err := jwt.ParseWithClaims(raw, &mc, func(t *jwt.Token) (any, error) {
		return i.secret, nil
	},
		// ⚠️ Chốt ĐÚNG MỘT thuật toán. Không có dòng này thì thư viện nhận mọi
		// thuật toán token tự khai trong header — gồm cả "none" (không chữ ký)
		// và kiểu nhầm lẫn HS/RS kinh điển. Kẻ tấn công tự viết token cho mình.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		// Lệch đồng hồ giữa các máy: cho 30 giây, không hơn.
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	sub, _ := mc["sub"].(string)
	sid, _ := mc["sid"].(string)
	uid, err1 := uuid.Parse(sub)
	sessID, err2 := uuid.Parse(sid)
	if err1 != nil || err2 != nil {
		return Claims{}, ErrInvalid
	}
	exp, err := mc.GetExpirationTime()
	if err != nil || exp == nil {
		return Claims{}, ErrInvalid
	}
	return Claims{UserID: uid, SessionID: sessID, ExpiresAt: exp.Time}, nil
}
