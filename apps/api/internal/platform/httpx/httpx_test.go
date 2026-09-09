package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"base-ecommerce/api/internal/platform/errs"
	"base-ecommerce/api/internal/platform/httpx"
	"github.com/stretchr/testify/require"
)

type problem struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id"`
	Errors    []errs.FieldError `json:"errors"`
}

func doRequest(t *testing.T, h http.HandlerFunc, body string) (*httptest.ResponseRecorder, problem) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	var p problem
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
	}
	return rec, p
}

func TestWrap_KhongLoiThiKhongDungToiResponse(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.JSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	})
	rec, _ := doRequest(t, h, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestWrap_MapKindSangMaHTTP(t *testing.T) {
	tests := []struct {
		name   string
		kind   errs.Kind
		status int
	}{
		{"internal", errs.KindInternal, 500},
		{"invalid", errs.KindInvalid, 400},
		{"unauthenticated", errs.KindUnauthenticated, 401},
		{"forbidden", errs.KindForbidden, 403},
		{"not found", errs.KindNotFound, 404},
		{"conflict", errs.KindConflict, 409},
		{"validation", errs.KindValidation, 422},
		{"rate limited", errs.KindRateLimited, 429},
		{"unavailable", errs.KindUnavailable, 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
				return errs.New(tt.kind, "SOME_CODE", "Thông điệp")
			})
			rec, p := doRequest(t, h, "")
			require.Equal(t, tt.status, rec.Code)
			require.Equal(t, tt.status, p.Status)
			require.Equal(t, "SOME_CODE", p.Code)
			require.Equal(t, "application/problem+json; charset=utf-8",
				rec.Header().Get("Content-Type"))
		})
	}
}

func TestWrap_LoiNoiBoKhongLamLoThongTinBenTrong(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New(`pq: duplicate key value violates unique constraint "products_sku_key"`)
	})
	rec, p := doRequest(t, h, "")

	require.Equal(t, 500, rec.Code)
	require.Equal(t, "INTERNAL_ERROR", p.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "products_sku_key")
	require.NotContains(t, body, "duplicate key")
}

func TestWrap_LoiValidateTraVeDanhSachTruong(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errs.Validation(errs.FieldError{
			Field: "price", Code: "REQUIRED", Message: "Giá là bắt buộc",
		})
	})
	rec, p := doRequest(t, h, "")
	require.Equal(t, 422, rec.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "price", p.Errors[0].Field)
}

func TestProblemType_SinhTuCode(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errs.New(errs.KindNotFound, "PRODUCT_NOT_FOUND", "Không tìm thấy")
	})
	_, p := doRequest(t, h, "")
	require.Equal(t, "/errors/product-not-found", p.Type)
}

func TestDecode_JSONHongThiTraMalformed(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	rec, p := doRequest(t, h, `{"name": `)
	require.Equal(t, 400, rec.Code)
	require.Equal(t, "MALFORMED_REQUEST", p.Code)
}

func TestDecode_TruongLaThiTuChoi(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	rec, p := doRequest(t, h, `{"name":"a","khong_ton_tai":1}`)
	require.Equal(t, 400, rec.Code)
	require.Equal(t, "MALFORMED_REQUEST", p.Code)
}

func TestDecode_HopLe(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	var got input
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		v, err := httpx.Decode[input](w, r)
		if err != nil {
			return err
		}
		got = v
		return httpx.JSON(w, http.StatusOK, v)
	})
	rec, _ := doRequest(t, h, `{"name":"Asus ROG"}`)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "Asus ROG", got.Name)
}

func TestWrap_DaGhiResponseRoiThiKhongGhiDe(t *testing.T) {
	// Handler ghi response rồi mới lỗi: KHÔNG được ghi đè, vì body hai document
	// khiến client đọc document đầu và tưởng thành công.
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		if err := httpx.JSON(w, http.StatusOK, map[string]string{"partial": "yes"}); err != nil {
			return err
		}
		return errs.New(errs.KindNotFound, "TOO_LATE", "Quá muộn")
	})
	rec, _ := doRequest(t, h, "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"partial":"yes"}`, rec.Body.String(),
		"body phải là đúng MỘT document, không được nối thêm problem+json")
}

func TestJSON_KhongMaHoaDuocThiTraLoi(t *testing.T) {
	// Marshal hỏng phải thành 500 tử tế, không phải 200 với body rỗng.
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.JSON(w, http.StatusOK, map[string]any{"ch": make(chan int)})
	})
	rec, p := doRequest(t, h, "")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "INTERNAL_ERROR", p.Code)
	require.NotContains(t, rec.Body.String(), "chan int")
}

func TestDecode_BodyQuaLonTraVe413(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	big := `{"name":"` + strings.Repeat("a", 2<<20) + `"}`
	rec, p := doRequest(t, h, big)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Equal(t, "PAYLOAD_TOO_LARGE", p.Code)
}

func TestDecode_BodyRongThiTraMalformed(t *testing.T) {
	// Không gửi body là dạng request hỏng phổ biến nhất trong thực tế.
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	rec, p := doRequest(t, h, "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "MALFORMED_REQUEST", p.Code)
}

func TestWriteError_LuonCoRequestID(t *testing.T) {
	// request_id phải luôn xuất hiện, kể cả rỗng — tài liệu 02 mục 2.1 nói
	// người dùng sẽ đọc mã này khi báo lỗi.
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errs.New(errs.KindNotFound, "X_NOT_FOUND", "Không thấy")
	})
	rec, _ := doRequest(t, h, "")
	require.Contains(t, rec.Body.String(), `"request_id"`)
}
