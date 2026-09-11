package domain

import (
	"regexp"
	"strings"
)

// vietnameseReplacer bỏ dấu tiếng Việt.
//
// Viết tay thay vì dùng golang.org/x/text/unicode/norm là có chủ đích:
//  1. Giữ domain chỉ phụ thuộc stdlib + 3 package trong danh sách trắng.
//  2. Bảng này xử lý cả dạng dựng sẵn (NFC) lẫn dạng tổ hợp (NFD), và xử lý
//     được "đ" — thứ mà chuẩn hóa NFD rồi bỏ dấu KHÔNG làm được, vì "đ" là
//     một chữ cái riêng chứ không phải "d" cộng dấu.
var vietnameseReplacer = strings.NewReplacer(
	"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a",
	"â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a",
	"ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
	"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e",
	"ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
	"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
	"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o",
	"ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o",
	"ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
	"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u",
	"ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
	"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
	"đ", "d",

	// Dạng tổ hợp (NFD): ký tự cơ sở + dấu rời, do macOS và một số nguồn dán ra.
	// Không xử lý thì mỗi dấu thành một gạch ngang — "Bàn" ra "ba-n".
	"̀", "", // huyền
	"́", "", // sắc
	"̃", "", // ngã
	"̉", "", // hỏi
	"̣", "", // nặng
	"̂", "", // dấu mũ â ê ô
	"̆", "", // dấu trăng ă
	"̛", "", // dấu móc ơ ư
)

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// NewSlug chuẩn hóa chuỗi tiếng Việt thành slug URL.
//
//	"Laptop Gaming Ổ Cứng SSD"  →  "laptop-gaming-o-cung-ssd"
func NewSlug(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = vietnameseReplacer.Replace(s)
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" || !slugPattern.MatchString(s) {
		return "", ErrInvalidSlug
	}
	return s, nil
}
