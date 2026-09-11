package domain

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// Money là value object cho tiền. KHÔNG BAO GIỜ dùng float64 cho tiền:
// 0.1 + 0.2 != 0.3 trong dấu phẩy động, và sai số đó cộng dồn qua từng đơn hàng.
type Money struct {
	amount   decimal.Decimal
	currency string
}

// moneyScale khớp với NUMERIC(15,2) trong migration.
const moneyScale = 2

// SupportedCurrency là loại tiền duy nhất P0.2 chấp nhận. Migration
// catalog_guards có CHECK (currency = 'VND') — không chặn ở tầng domain thì
// lỗi 23514 của Postgres nổi lên thành 500 thay vì 422 sạch sẽ.
const SupportedCurrency = "VND"

// maxMoney là giá trị lớn nhất NUMERIC(15,2) chứa được: 13 chữ số phần nguyên.
var maxMoney = decimal.RequireFromString("9999999999999.99")

// NewMoney nhận chuỗi thập phân, ví dụ "25990000" hoặc "25990000.50".
func NewMoney(amount, currency string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, ErrInvalidPrice
	}
	if d.IsNegative() {
		return Money{}, ErrInvalidPrice
	}
	// Postgres NUMERIC(15,2) làm tròn IM LẶNG: "25990000.999" thành 25990001.00.
	// Không chặn ở đây thì giá client đọc lại sau khi ghi KHÁC giá vừa gửi lên,
	// và không có lỗi nào ở giữa để lần ra.
	if d.Exponent() < -moneyScale {
		return Money{}, ErrInvalidPrice
	}
	// Vượt ngưỡng thì Postgres báo lỗi tràn số — bắt ở đây để trả 422 sạch sẽ
	// thay vì để lỗi driver nổi lên thành 500.
	if d.GreaterThan(maxMoney) {
		return Money{}, ErrInvalidPrice
	}
	if currency == "" {
		currency = SupportedCurrency
	}
	if currency != SupportedCurrency {
		return Money{}, ErrUnsupportedCurrency
	}
	return Money{amount: d, currency: currency}, nil
}

// MoneyFromDecimal dùng khi đọc từ database, nơi giá trị đã hợp lệ.
func MoneyFromDecimal(d decimal.Decimal, currency string) Money {
	if currency == "" {
		currency = "VND"
	}
	return Money{amount: d, currency: currency}
}

func (m Money) IsZero() bool             { return m.amount.IsZero() }
func (m Money) Currency() string         { return m.currency }
func (m Money) Decimal() decimal.Decimal { return m.amount }

// String trả chuỗi thập phân, dạng dùng trong JSON. Không định dạng hiển thị —
// việc đó là của frontend.
func (m Money) String() string { return m.amount.String() }

// MarshalJSON và UnmarshalJSON cần cho việc cache entity.
//
// Money có field không xuất khẩu, nên nếu không có hai hàm này thì giá sẽ biến
// mất khi đi qua cache — sản phẩm đọc từ cache có giá bằng 0, còn đọc thẳng từ
// database thì đúng. Đây là loại lỗi rất khó tìm vì nó chỉ xuất hiện ở lần đọc
// thứ hai trở đi.
type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(moneyJSON{Amount: m.amount.String(), Currency: m.currency})
}

func (m *Money) UnmarshalJSON(b []byte) error {
	var raw moneyJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	d, err := decimal.NewFromString(raw.Amount)
	if err != nil {
		return ErrInvalidPrice
	}
	m.amount = d
	m.currency = raw.Currency
	return nil
}
