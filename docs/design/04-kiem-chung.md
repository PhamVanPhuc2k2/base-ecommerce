# 04 — Kiểm chứng

Dự án **không dùng unit test**. Tài liệu này ghi lại: thay vào đó dùng gì, kiểm
chứng thế nào, và những rủi ro nào đã được chấp nhận một cách có ý thức.

---

## 1. Quyết định

Không có file `_test.go`. Không testify, không testcontainers, không CI job chạy test.

Đây là lựa chọn của chủ dự án sau khi đã cân nhắc đánh đổi. Tài liệu này **không**
tranh luận lại quyết định đó — nó tồn tại để phần kiểm chứng còn lại được làm cho
tử tế, và để người đọc sau này biết chính xác điều gì không được bảo vệ.

---

## 2. Lưới an toàn tự động: `task check`

Đây là thứ duy nhất chạy tự động, cả ở máy dev lẫn CI.

```bash
task check     # = build + vet + lint + arch
```

| Bước | Bắt được gì |
|---|---|
| `go build ./...` | Lỗi biên dịch, sai kiểu, thiếu/thừa import |
| `go vet ./...` | Format string sai, lỗi copy mutex, shadow biến nguy hiểm |
| `golangci-lint run` | Bỏ qua lỗi trả về, quên đóng body, quên `rows.Err()`, so sánh lỗi bằng `==` thay vì `errors.Is`, `return nil` khi `err != nil`, thiếu context |
| `scripts/check-arch.sh` | `domain` chạm hạ tầng, `app` import `net/http`, adapter giữ `*pgxpool.Pool` |

**Không có bước nào bắt được lỗi logic.** Một hàm biên dịch được, không vi phạm
linter, không phá kiến trúc — nhưng tính sai tiền — sẽ đi thẳng vào production.

Vì không có test, cấu hình linter được bật rộng hơn thông thường. **Đừng tắt
linter để cho code qua**; nếu một luật gây phiền thì sửa code, hoặc bàn rồi mới tắt.

---

## 3. Kiểm chứng thủ công

Mỗi task trong kế hoạch có mục **"Kiểm chứng"** ghi rõ lệnh phải chạy và kết quả
phải thấy. Đó là phần thay cho test. Bỏ qua nó thì không còn gì chứng minh code
chạy đúng.

Nguyên tắc viết một bước kiểm chứng cho tốt:

- **Nêu kết quả mong đợi cụ thể**, không viết "kiểm tra xem có chạy không".
  Viết "phải trả 503 và `checks.postgres = fail`".
- **Kiểm luôn ca thất bại**, đừng chỉ kiểm ca thành công. Dừng Postgres rồi gọi
  `/readyz` mới là phép thử thật.
- **Kiểm cả cái không được xảy ra.** Ví dụ: dừng Postgres thì `/healthz` phải
  **vẫn** trả 200 — nếu nó cũng 503 là đã cài sai.
- **Ghi lại output thật** vào mô tả commit hoặc PR khi hành vi khó dựng lại.

### 3.1. Công cụ theo loại thay đổi

| Loại thay đổi | Cách kiểm chứng |
|---|---|
| Logic thuần (tính tiền, quy tắc nghiệp vụ) | Viết `cmd/scratch/main.go` tạm, in kết quả, đối chiếu bằng mắt, rồi **xóa file** |
| Endpoint HTTP | `curl -i` — kiểm mã trạng thái, `Content-Type`, và hình dạng JSON |
| SQL / repository | Chạy câu lệnh trong `psql`, xem `EXPLAIN ANALYZE` nếu là query có filter |
| Migration | `task migrate` rồi `task migrate-down` rồi `task migrate` lại — round-trip phải sạch |
| Transaction | Kịch bản `cmd/scratch` như Task 7 của P0.1: commit, rollback, lồng nhau, context bị hủy, và `pool.Stat().AcquiredConns() == 0` |
| Đồng thời (tồn kho, giữ chỗ) | Xem mục 4 — đây là chỗ nguy hiểm nhất |
| Frontend | Mở trình duyệt, thao tác thật, xem tab Network và Console |

### 3.2. Danh sách kiểm nhanh sau mỗi thay đổi lớn

Chạy hết, mất khoảng hai phút:

```bash
task check
task up && task migrate
task run
```
rồi ở terminal khác:
```bash
curl -i localhost:8080/healthz      # 200
curl -i localhost:8080/readyz       # 200, checks.postgres = ok
curl -i localhost:8080/khong-co     # 404 + log JSON có request_id
```
rồi `Ctrl+C` — phải thấy `"đã dừng"` và thoát mã 0.

---

## 4. Rủi ro đã chấp nhận

Ghi thẳng, không giảm nhẹ. Đây là những thứ không có gì bảo vệ:

⚠️ **Và một cảnh báo về chính cách kiểm chứng thủ công.** Mọi bước kiểm trong các
kế hoạch đều chạy trên vài dòng dữ liệu. Ở quy mô đó Postgres luôn chọn `Seq Scan`
và **mọi kế hoạch truy vấn trông giống hệt nhau** — nghĩa là bước kiểm chứng không
phân biệt được index tốt với không có index. `SET enable_seqscan = off` chứng minh
index *tồn tại*, không chứng minh nó *được chọn*. Chỗ nào đo hiệu năng thì phải nạp
vài trăm nghìn dòng bằng `generate_series`, chạy `ANALYZE`, rồi mới `EXPLAIN`.

**1. Lỗi hồi quy khi sửa code cũ.** Sửa `errs` hay `httpx` sẽ không có gì báo là
đã làm hỏng chỗ gọi tới nó. Phải tự nhớ và tự chạy lại phần kiểm chứng liên quan.
Rủi ro này lớn dần theo số module — tới P0.4 sẽ có 4 module cùng dùng `platform/*`.

**2. Sai sót về đồng thời.** Đây là rủi ro nghiêm trọng nhất, vì nghiệp vụ có
**bán vượt tồn kho**. Một lỗi kiểu 50 người cùng mua 10 sản phẩm cuối và 12 đơn
thành công thì gần như không thể phát hiện bằng tay: gọi `curl` tuần tự luôn cho
kết quả đúng. Khi làm P3 (Inventory), tối thiểu phải:

- Dùng `UPDATE inventory SET available = available - $1 WHERE ... AND available >= $1`
  và kiểm `RowsAffected() == 0`, **không** `SELECT` rồi `UPDATE`
- Viết một kịch bản `cmd/scratch` bắn 50 goroutine cùng lúc, đối chiếu số đơn
  thành công với tồn kho ban đầu, rồi xóa file
- Đối soát tồn kho với tổng đơn đã đặt định kỳ trên production

**3. Vòng lặp trong cây danh mục.** `categories.parent_id` tự tham chiếu, và khóa
ngoại **không** ngăn được vòng lặp. Một câu `UPDATE` là đủ:

```sql
-- A → B → C đã tồn tại
UPDATE categories SET parent_id = C_id WHERE id = A_id;   -- vòng lặp, không lỗi
```

Migration `catalog_guards` đã chặn trường hợp một node (`CHECK (parent_id <> id)`),
nhưng vòng lặp nhiều node **vẫn tạo được**.

Hậu quả tệ hơn vẻ ngoài. `Tree.DescendantIDs` có tập visited nên tiến trình không
treo — nhưng `NewTree` chỉ đưa danh mục vào `roots` khi `parent_id IS NULL`, mà mọi
thành viên của vòng lặp đều có cha. Nghĩa là **cả nhánh biến mất khỏi `Roots()`**:
điều hướng cửa hàng mất nguyên một mảng danh mục, không có lỗi nào ở đâu cả.

P0.2 không có endpoint ghi danh mục nên chỉ chạm tới được bằng SQL viết tay — đúng
cách dữ liệu mẫu được nạp. Khi P1 thêm CRUD danh mục, endpoint đó **bắt buộc** phải
kiểm tổ tiên trước khi ghi.

**4. Lỗi mapping dữ liệu.** `JSONB`, mảng, `NUMERIC`, `timestamptz` rất dễ sai khi
chuyển qua lại giữa Go và Postgres. Không có round-trip test thì phải tự lưu rồi
đọc lại và so từng trường bằng mắt, ít nhất một lần cho mỗi entity.

**5. Trôi lệch hợp đồng API.** Backend đổi tên trường mà quên sửa `openapi.yaml`
sẽ không ai báo. Từ P0.2, job `openapi-drift` trong CI (sinh lại client TS rồi
kiểm `git diff`) bù được **một phần**: nó bắt được spec và code TS lệch nhau,
nhưng không bắt được spec và handler Go lệch nhau.

**6. Lỗi ở nhánh hiếm.** Nhánh xử lý lỗi, timeout, retry gần như không bao giờ
được chạy khi thao tác tay. `TxManager` retry lỗi 40001 là ví dụ: kiểm chứng thủ
công không dựng được tình huống tranh chấp tuần tự hóa.

---

## 5. Bù đắp

Những thứ này không thay được test, nhưng làm giảm rủi ro và **nên được giữ nghiêm**:

| | |
|---|---|
| **Linter bật rộng** | Xem `apps/api/.golangci.yml`. Đây là thứ duy nhất đọc code hộ bạn |
| **Kiểm tra kiến trúc bằng máy** | `scripts/check-arch.sh` chặn `domain` chạm hạ tầng — sai kiến trúc là loại lỗi đắt nhất để sửa về sau |
| **Domain giữ quy tắc nghiệp vụ** | Logic tập trung một chỗ thì đọc lại và kiểm bằng mắt dễ hơn nhiều so với khi nó rải khắp handler |
| **Kiểu dữ liệu chặt** | `Money` là value object chứ không phải `float64`; `ProductID` chứ không phải `string`. Trình biên dịch bắt được nhiều lỗi hơn |
| **Code review** | Không có test thì review là lớp kiểm tra logic duy nhất. Đọc kỹ hơn bình thường |
| **Ghi log tử tế** | Log JSON có `request_id` là công cụ chẩn đoán chính khi có sự cố production |
| **Sao lưu và khôi phục được kiểm thử** | Xem tài liệu 05 mục 8. Không có lưới an toàn ở tầng code thì lưới ở tầng dữ liệu càng quan trọng |

---

## 6. Khi nào nên xem lại quyết định này

Không phải để tranh luận, mà là các mốc đáng dừng lại cân nhắc:

- Khi có người thứ hai vào code cùng — không ai nhớ hết được ràng buộc của người khác
- Khi làm P3 (tồn kho) hoặc P5 (thanh toán) — sai một lần là mất tiền thật
- Khi cùng một lỗi hồi quy xuất hiện lần thứ hai
- Khi bắt đầu có khách hàng thật đặt đơn
