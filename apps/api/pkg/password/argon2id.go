// Package password băm và kiểm mật khẩu bằng argon2id.
//
// Lưu dạng chuỗi PHC: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>. Tham số
// nằm TRONG chuỗi — tăng tham số sau này (máy mạnh hơn) không vô hiệu mật khẩu
// cũ, vì mỗi hash tự mang tham số đã dùng để tạo ra nó.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Tham số khuyến nghị của OWASP cho argon2id (2024): 19 MiB, 2 vòng, 1 luồng.
// Đủ đắt để dò mật khẩu hàng loạt không kinh tế, đủ rẻ để một lần đăng nhập
// chỉ tốn vài chục mili-giây CPU.
const (
	memoryKiB  = 19 * 1024
	iterations = 2
	threads    = 1
	saltLen    = 16
	keyLen     = 32
)

var b64 = base64.RawStdEncoding

// ErrMalformed: chuỗi hash không phải định dạng ta sinh ra. Không bao giờ nên
// xảy ra — xảy ra nghĩa là dữ liệu hỏng hoặc ai đó ghi tay vào bảng users.
var ErrMalformed = errors.New("password: chuỗi hash không đúng định dạng argon2id")

type Hasher struct{}

func NewHasher() *Hasher { return &Hasher{} }

func (Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, iterations, memoryKiB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify so mật khẩu với hash, dùng ĐÚNG tham số ghi trong hash.
//
// So sánh bằng ConstantTimeCompare: so từng byte rồi dừng ở byte sai đầu tiên
// làm thời gian phản hồi lộ ra bao nhiêu byte đầu đã đúng.
func (Hasher) Verify(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrMalformed
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrMalformed
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrMalformed
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, ErrMalformed
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, ErrMalformed
	}
	got := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
