package domain

import (
	"github.com/shopspring/decimal"
)

// Money là value object cho tiền. KHÔNG BAO GIỜ dùng float64 cho tiền:
// 0.1 + 0.2 != 0.3 trong dấu phẩy động, và sai số đó cộng dồn qua từng đơn hàng.
type Money struct {
	amount   decimal.Decimal
	currency string
}

// NewMoney nhận chuỗi thập phân, ví dụ "25990000" hoặc "25990000.50".
func NewMoney(amount, currency string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, ErrInvalidPrice
	}
	if d.IsNegative() {
		return Money{}, ErrInvalidPrice
	}
	if currency == "" {
		currency = "VND"
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
