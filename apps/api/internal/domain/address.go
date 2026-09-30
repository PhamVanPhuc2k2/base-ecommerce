package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"base-ecommerce/api/pkg/errs"

	"github.com/google/uuid"
)

// MaxAddressesPerUser: sổ địa chỉ là để chọn nhanh lúc thanh toán, không phải
// danh bạ. Trần cũng chặn một tài khoản bị chiếm ghi rác vô hạn vào DB.
const MaxAddressesPerUser = 10

// Mã lỗi THEO TRƯỜNG (errors[].code) của địa chỉ. Không phải mã lỗi cấp
// response — mã cấp response là VALIDATION_FAILED.
const (
	FieldInvalidPhone = "INVALID_PHONE"
	FieldInvalid      = "FIELD_INVALID"
)

// Address là một địa chỉ giao hàng trong sổ của người dùng.
//
// Hai cấp hành chính (tỉnh/thành, phường/xã) theo sắp xếp đơn vị hành chính
// 2025 — không còn cấp quận/huyện. Chữ tự do: danh mục chuẩn để P5 (vận
// chuyển) quyết định, vì hãng vận chuyển mới là bên cần mã chuẩn.
type Address struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	RecipientName string
	Phone         string
	Province      string
	Ward          string
	Street        string
	IsDefault     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AddressPatch: nil = giữ nguyên. Dùng cho cả tạo mới (mọi trường phải có).
type AddressPatch struct {
	RecipientName *string
	Phone         *string
	Province      *string
	Ward          *string
	Street        *string
}

func NewAddress(userID uuid.UUID, in AddressPatch) (*Address, error) {
	now := time.Now().UTC()
	a := &Address{ID: uuid.Must(uuid.NewV7()), UserID: userID, CreatedAt: now, UpdatedAt: now}
	empty := ""
	for _, p := range []**string{&in.RecipientName, &in.Phone, &in.Province, &in.Ward, &in.Street} {
		if *p == nil {
			*p = &empty // thiếu trường khi tạo = rỗng = lỗi của đúng trường đó
		}
	}
	if err := a.apply(in); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Address) Update(in AddressPatch) error {
	if err := a.apply(in); err != nil {
		return err
	}
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// apply kiểm và gán; gom ĐỦ lỗi mọi trường rồi trả một lần, để form hiện đỏ
// tất cả các ô sai cùng lúc.
func (a *Address) apply(in AddressPatch) error {
	var fields []errs.FieldError
	text := func(field string, v *string, max int, dst *string) {
		if v == nil {
			return
		}
		s := strings.Join(strings.Fields(*v), " ") // gộp khoảng trắng thừa
		if s == "" || utf8.RuneCountInString(s) > max {
			fields = append(fields, errs.FieldError{Field: field, Code: FieldInvalid,
				Message: "Vui lòng nhập, tối đa " + strconv.Itoa(max) + " ký tự"})
			return
		}
		*dst = s
	}
	text("recipient_name", in.RecipientName, 100, &a.RecipientName)
	if in.Phone != nil {
		if p, ok := NormalizePhone(*in.Phone); ok {
			a.Phone = p
		} else {
			fields = append(fields, errs.FieldError{Field: "phone", Code: FieldInvalidPhone,
				Message: "Số điện thoại di động không hợp lệ, ví dụ 0912345678"})
		}
	}
	text("province", in.Province, 100, &a.Province)
	text("ward", in.Ward, 100, &a.Ward)
	text("street", in.Street, 255, &a.Street)
	if len(fields) > 0 {
		return errs.Validation(fields...)
	}
	return nil
}

// NormalizePhone chuẩn hóa số di động Việt Nam về dạng 0xxxxxxxxx.
//
// Nhận "+84 912.345.678", "84912345678", "0912-345-678". Chỉ đầu số di động
// 03/05/07/08/09: hãng vận chuyển gọi/nhắn tin cho người nhận, số bàn không
// nhận được SMS báo giao hàng.
func NormalizePhone(s string) (string, bool) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '.' || r == '-':
		default:
			return "", false
		}
	}
	d := b.String()
	if strings.HasPrefix(d, "84") && len(d) == 11 {
		d = "0" + d[2:]
	}
	if len(d) != 10 || d[0] != '0' || !strings.ContainsRune("35789", rune(d[1])) {
		return "", false
	}
	return d, true
}
