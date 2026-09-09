# 04 — Chiến lược test

---

## 1. Nguyên tắc

**Không mock database.** Mock repository chỉ kiểm tra được code Go của bạn, trong
khi phần dễ sai nhất lại là SQL: sai `JOIN`, quên `WHERE deleted_at IS NULL`, hiểu
nhầm isolation level, index không được dùng. Test với Postgres thật qua
testcontainers.

**Không dùng framework mock** (gomock, mockery). Kiến trúc hexagonal đã cho sẵn
interface nhỏ — viết fake bằng tay 20 dòng đọc dễ hơn và không cần bước sinh code.

**Mỗi tầng có kiểu test riêng.** Đây là phần thưởng thật sự của kiến trúc đã chọn:
domain test được mà không cần Docker, chạy trong mili-giây.

---

## 2. Bản đồ test theo tầng

| Tầng | Loại test | Phụ thuộc | Tốc độ | Mục tiêu bao phủ |
|---|---|---|---|---|
| `domain/` | Unit thuần | Không | < 10ms | **≥ 90%** |
| `app/` | Use case + fake port | Không | < 50ms | ≥ 80% |
| `adapter/pgstore/` | Integration | Postgres thật | ~vài trăm ms | Đường đi chính + các nhánh lỗi |
| `adapter/httpapi/` | `httptest` + contract | Fake use case | < 50ms | Mã trạng thái + hình dạng JSON |
| `platform/` | Integration | Postgres/Redis/Rabbit | | Hành vi lỗi, timeout |
| Toàn hệ thống | E2E | docker-compose | vài giây | 3–5 luồng quan trọng nhất |
| `apps/web` | Vitest + Playwright | API thật | | Luồng mua hàng |

Số lượng test nên giảm dần khi đi từ trên xuống. Nếu đảo ngược — nhiều E2E, ít
unit — thì bộ test sẽ chậm, hay hỏng vặt và không ai buồn chạy.

---

## 3. Tầng `domain`

Không có Docker, không có `context`, không có I/O. Chỉ là hàm thuần.

```go
func TestProduct_Publish(t *testing.T) {
    tests := []struct {
        name    string
        product func() *domain.Product
        wantErr error
    }{
        {"thiếu ảnh",     func() *domain.Product { return newDraft(withoutImages) }, domain.ErrNoImage},
        {"chưa có giá",   func() *domain.Product { return newDraft(withZeroPrice) }, domain.ErrPriceRequired},
        {"đã đăng rồi",   func() *domain.Product { return newLive() },                domain.ErrAlreadyPublished},
        {"hợp lệ",        func() *domain.Product { return newDraft() },               nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := tt.product()
            err := p.Publish()
            require.ErrorIs(t, err, tt.wantErr)      // ErrorIs, không so sánh chuỗi
            if tt.wantErr == nil {
                require.Equal(t, domain.StatusLive, p.Status)
                require.Len(t, p.PullEvents(), 1)     // có phát domain event
            }
        })
    }
}
```

Bắt buộc test ở tầng này:
- Constructor từ chối trạng thái sai
- Mọi nhánh của quy tắc nghiệp vụ
- Số học tiền tệ: làm tròn, cộng trừ, so sánh (**không được dùng float**)
- Domain event được phát đúng lúc

---

## 4. Tầng `app` — fake viết tay

```go
type fakeProductRepo struct {
    items   map[domain.ProductID]*domain.Product
    saveErr error                                  // ép lỗi để test nhánh hỏng
}

func (f *fakeProductRepo) Save(_ context.Context, p *domain.Product) error {
    if f.saveErr != nil { return f.saveErr }
    f.items[p.ID] = p
    return nil
}

// TxManager giả: chạy thẳng fn, không có transaction thật
type fakeTx struct{}
func (fakeTx) Run(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
```

Test use case tập trung vào **điều phối**, không lặp lại quy tắc nghiệp vụ đã test
ở domain:

```go
func TestCreateProduct_GhiOutboxCungTransaction(t *testing.T) {
    repo, ob := &fakeProductRepo{items: map[...]{}}, &fakeOutbox{}
    uc := app.NewCreateProduct(fakeTx{}, repo, ob)

    _, err := uc.Execute(ctx, validInput())

    require.NoError(t, err)
    require.Len(t, ob.appended, 1)
    require.Equal(t, "product.created", ob.appended[0].Type)
}

func TestCreateProduct_LuuLoiThiKhongGhiOutbox(t *testing.T) {
    repo := &fakeProductRepo{saveErr: errors.New("boom")}
    ...
    require.Empty(t, ob.appended)
}
```

---

## 5. Tầng `pgstore` — testcontainers

### 5.1. Một container cho mỗi package, một database cho mỗi test

Khởi động container mất vài giây; tạo database từ template mất vài chục mili-giây.
Vì vậy: container dựng một lần trong `TestMain`, chạy migration một lần vào
database mẫu, mỗi test `CREATE DATABASE ... TEMPLATE`.

```go
// internal/platform/testdb/testdb.go
var pool *pgxpool.Pool   // trỏ tới database quản trị để tạo/xóa DB

// Start dựng container và chạy migration. KHÔNG gọi m.Run() thay bạn.
func Start() (stop func(), err error) { ... }

// New tạo database riêng cho một test, tự dọn khi test xong.
// Chạy với -short thì không có container → bỏ qua test một cách nhìn thấy được.
func New(t *testing.T) *pgxpool.Pool {
    t.Helper()
    if pool == nil {
        t.Skip("cần Docker; bỏ qua vì -short")
    }
    name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
    _, err := pool.Exec(context.Background(),
        fmt.Sprintf("CREATE DATABASE %s TEMPLATE template_test", name))
    require.NoError(t, err)
    t.Cleanup(func() { pool.Exec(context.Background(), "DROP DATABASE "+name) })
    ...
}
```

Mỗi test có database sạch của riêng nó → **chạy song song được** (`t.Parallel()`),
không có test nào phụ thuộc thứ tự chạy của test khác.

*(Cách đơn giản hơn cho P0: một database dùng chung, `TRUNCATE ... CASCADE` giữa
các test. Chấp nhận được cho tới khi bộ test lớn lên và cần chạy song song.)*

### 5.1b. `TestMain` phải LUÔN gọi `m.Run()`

Package nào cần database thì viết `TestMain` đúng mẫu này:

```go
func TestMain(m *testing.M) {
    flag.Parse()                       // bắt buộc trước khi đọc testing.Short()

    var stop func()
    if !testing.Short() {
        var err error
        if stop, err = testdb.Start(); err != nil {
            log.Fatalf("không khởi động được database test: %v", err)
        }
    }

    code := m.Run()                    // LUÔN chạy, kể cả khi -short
    if stop != nil { stop() }
    os.Exit(code)
}
```

⚠️ **Đừng viết `if testing.Short() { os.Exit(0) }`.** Nó bỏ qua **toàn bộ** package
chứ không riêng test cần Docker: một test thuần thêm vào package đó sau này sẽ im
lặng ngừng chạy, mà `go test` vẫn in `ok`. Với mẫu trên, test cần database tự
`t.Skip` bên trong `testdb.New(t)` và `go test` in ra dòng `SKIP` nhìn thấy được,
còn test thuần vẫn chạy bình thường.

### 5.2. Những gì phải test ở tầng này

- Round-trip: lưu entity → đọc lại → **so sánh bằng toàn bộ**, gồm cả `JSONB`,
  mảng, `NUMERIC`, `timestamptz` (đây là chỗ lộ ra lỗi mapping)
- Ràng buộc unique trả đúng `errs.Error` với `Code: "DUPLICATE_SKU"`, không phải
  lỗi thô của pgx lọt ra ngoài
- `ListProducts` với mọi tổ hợp bộ lọc, đặc biệt là filter `JSONB @>`
- Phân trang: trang cuối, trang rỗng, `limit` biên
- Bản ghi đã soft-delete không xuất hiện trong danh sách
- Transaction rollback thì **không** còn dấu vết trong database

### 5.3. Test tính đúng đắn khi chạy song song — bắt buộc cho tồn kho

Đây là test quan trọng nhất của cả dự án. Không có nó thì bán vượt sẽ được phát
hiện bởi khách hàng, không phải bởi bạn.

```go
func TestReserveStock_KhongBanVuot(t *testing.T) {
    db := testdb.New(t)
    seedStock(t, db, skuID, 10)          // còn đúng 10 cái

    var ok, failed atomic.Int32
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {            // 50 người cùng mua 1 cái
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := repo.Reserve(ctx, skuID, 1); err != nil {
                failed.Add(1)
            } else {
                ok.Add(1)
            }
        }()
    }
    wg.Wait()

    require.EqualValues(t, 10, ok.Load())
    require.EqualValues(t, 40, failed.Load())
    require.EqualValues(t, 0, readStock(t, db, skuID))   // không âm
}
```

Viết test này **trước** khi viết `Reserve` — nó là định nghĩa của "đúng".

---

## 6. Tầng `httpapi`

```go
func TestGetProduct_KhongTonTai(t *testing.T) {
    h := httpapi.New(&fakeGetProduct{err: domain.ErrProductNotFound})
    req := httptest.NewRequest("GET", "/api/v1/products/khong-co", nil)
    rec := httptest.NewRecorder()

    h.ServeHTTP(rec, req)

    require.Equal(t, 404, rec.Code)
    require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

    var p problem
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
    require.Equal(t, "PRODUCT_NOT_FOUND", p.Code)

    assertMatchesSpec(t, req, rec.Result())     // contract test, xem tài liệu 02
}
```

Bắt buộc có test: response 500 **không** làm lộ thông tin nội bộ (tên bảng, tên
cột, câu SQL, đường dẫn file).

---

## 7. E2E backend

Chạy trên `compose.dev.yml` thật, gọi API qua HTTP. Chỉ giữ **3–5 luồng** quan
trọng nhất — E2E chậm và dễ hỏng vặt, không phải chỗ để phủ hết trường hợp biên.

Luồng cho P0:
1. Tạo sản phẩm → đọc lại qua API → có dòng trong `outbox`
2. Relay publish → worker nhận được và đánh dấu đã xử lý
3. Gửi lại cùng `event_id` → không xử lý lần hai

---

## 8. Frontend

**Vitest** — hàm thuần: format tiền VND, dựng query string bộ lọc, map mã lỗi sang
tiếng Việt.

**Playwright** — chỉ luồng người dùng thật:
1. Trang danh mục → lọc theo thương hiệu → URL đổi → kết quả đúng
2. Vào chi tiết sản phẩm → thêm vào giỏ → giỏ hiện đúng số lượng
3. (từ P4) Đặt hàng đến khi có mã đơn

Chống hỏng vặt: chọn phần tử bằng `data-testid` hoặc vai trò (role), **không** dùng
class CSS hay chuỗi văn bản — đổi chữ hiển thị không được làm hỏng test.

---

## 9. Quy ước

**Đặt tên:** `Test{Hàm}_{TìnhHuống}` — mô tả tình huống bằng tiếng Việt không dấu
hoặc tiếng Anh, miễn là đọc lên hiểu ngay: `TestReserveStock_KhongDuHang`.

**Builder cho dữ liệu test** — đừng lặp lại việc dựng entity ở mọi test:

```go
func aProduct(opts ...func(*domain.Product)) *domain.Product { ... }
p := aProduct(withPrice("25990000"), withoutImages)
```

**`require` hay `assert`:** dùng `require` khi test không thể đi tiếp nếu sai (đa
số trường hợp), `assert` khi muốn thấy hết các sai lệch trong một lần chạy.

**Test hỏng vặt (flaky):** đánh dấu `t.Skip` kèm link issue **ngay trong ngày** và
sửa trong tuần. Một test hỏng vặt còn tệ hơn không có test — nó dạy cả nhóm thói
quen bỏ qua CI đỏ.

**Không có `time.Sleep` trong test.** Chờ điều kiện bằng `require.Eventually`.

---

## 10. Chạy test

```yaml
test-unit:    # nhanh, không cần Docker — chạy khi đang code
  # -short chứ không liệt kê cứng tên package: danh sách cứng vừa hỏng khi
  # package chưa tồn tại, vừa phải sửa mỗi lần thêm module.
  cmd: go test -short ./...

test:         # cần Docker
  cmd: go test ./...

test-race:    # cần CGO (gcc/mingw-w64 trên PATH)
  env: { CGO_ENABLED: 1 }
  cmd: go test ./... -race
```

**`-race` là bắt buộc ở CI, tùy chọn ở máy dev.** Với hệ thống có worker và cache,
race condition là loại lỗi tốn nhiều thời gian nhất để tìm bằng tay — nên không
được phép merge mà chưa qua race detector.

Nhưng `-race` **đòi cgo**, mà máy dev Windows thường không có trình biên dịch C:

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

Nếu để `-race` trong lệnh test mặc định thì lập trình viên trên Windows không chạy
được test nào cả. Vì vậy: local mặc định không bật, CI (ubuntu-latest, có sẵn gcc)
luôn bật. Ai cài mingw-w64 thì dùng được `task test-race` ở máy.

**Mục tiêu thời gian:** `test-unit` dưới 5 giây (chạy được mỗi lần lưu file),
toàn bộ dưới 3 phút trên CI. Vượt quá thì người ta sẽ ngừng chạy nó.

---

## 11. Việc cần làm

- [ ] `platform/testdb`: `Setup(m)`, `New(t)` theo mẫu template database
- [ ] Fixture/builder cho `domain.Product` và các entity chính
- [ ] Fake cho từng port: repository, cache, outbox, TxManager
- [ ] Helper `assertMatchesSpec` bằng `kin-openapi`
- [ ] Test song song chống bán vượt (mục 5.3) — viết trước khi làm P3
- [ ] Test Redis chết → API vẫn chạy
- [ ] Playwright: cấu hình + 3 luồng ở mục 8
- [ ] CI: tách job `test-unit` (nhanh, chạy trước) và `test-int`
- [ ] Báo cáo coverage, cảnh báo khi `domain` tụt dưới 90%
