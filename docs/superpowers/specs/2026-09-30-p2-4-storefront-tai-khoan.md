# P2.4 — Storefront: tài khoản + sổ địa chỉ: Thiết kế

> Thuộc P2, xem [tổng quan](2026-09-30-p2-tong-quan.md). Giai đoạn cuối của P2:
> đưa P2.1–P2.3 lên giao diện, thêm sổ địa chỉ cho P4 (thanh toán) dùng.

## 1. Phạm vi

**Có:**

- **Trang:** `/dang-nhap`, `/dang-ky`, `/quen-mat-khau`, `/dat-lai-mat-khau`,
  `/tai-khoan` (thông tin, xác minh email, sửa họ tên, đăng xuất),
  `/tai-khoan/dia-chi` (sổ địa chỉ).
- **Phiên đăng nhập:** BFF bằng cookie httpOnly. `proxy.ts` chặn các trang
  `/tai-khoan/*` và làm mới token.
- **API:**
  - `PATCH /me` sửa họ tên.
  - CRUD `/me/addresses`, cộng thao tác đặt địa chỉ mặc định.
  - API tin IP khách từ `X-Forwarded-For` **chỉ khi** kết nối đến từ proxy
    tin cậy.

**Không:**

- **Đổi mật khẩu khi đang đăng nhập:** đã có quên / đặt lại mật khẩu, việc
  này để sau.
- **Đăng nhập Google/Zalo.**
- **Danh mục tỉnh/xã chuẩn:** giai đoạn này là chữ tự do, P5 (vận chuyển)
  quyết định.
- **Header hiện tên người dùng:** xem 2.5.
- **Giao diện quản trị.**

## 2. Quyết định

### 2.1. Cookie, không phải localStorage

| Cookie | Nội dung | Thuộc tính |
|---|---|---|
| `bec_at` | access token | httpOnly, SameSite=Lax, Secure khi production, `maxAge` = `expires_in` − 60 giây |
| `bec_rt` | refresh token | httpOnly, SameSite=Lax, Secure khi production, hết hạn đúng `refresh_expires_at` |

- **Không token nào chạm JavaScript phía trình duyệt**, nên XSS không lấy được
  token.
- **API không đổi:** vẫn chỉ nhận Bearer (P2.1). Next đọc cookie rồi gắn
  header.
- **Vì sao `bec_at` hết hạn sớm 60 giây:** để proxy làm mới *trước* khi API
  từ chối token.
- **Vì sao SameSite=Lax chứ không Strict:** người dùng bấm link trong email
  (mở từ ứng dụng khác) vẫn phải còn đăng nhập.
- **Chống CSRF:** Server Action của Next đã kiểm `Origin` khớp `Host`.

### 2.2. Làm mới token ở `proxy.ts`

Server Component không ghi được cookie; chỉ proxy, Server Action và Route
Handler ghi được. Vì vậy proxy chạy cho `/tai-khoan/:path*`:

1. **Còn `bec_at`:** cho qua.
2. **Hết `bec_at`, còn `bec_rt`:** gọi `POST /auth/refresh`, rồi ghi cookie
   mới lên **cả** response (cho trình duyệt) **lẫn** request đang đi tiếp
   (để trang render ngay bằng token mới).
3. **Không còn gì, hoặc refresh hỏng:** xóa cookie, rồi 307 về
   `/dang-nhap?next=<đường dẫn>`.

**Gộp refresh trùng.** Hai request song song cùng mang một `bec_rt` (trang và
prefetch của nó, hay hai tab) thì request thứ hai trông như *dùng lại token* và
làm API thu hồi cả phiên (P2.1 mục 2.4) — người dùng tự nhiên bị đăng xuất.
Cách xử lý:

- Proxy giữ một bản đồ trong bộ nhớ `hash(bec_rt) → promise` sống 10 giây, nên
  các lần refresh trùng nhận chung một kết quả.
- **Giới hạn:** cách này chỉ đúng khi web chạy **một bản**. Chạy nhiều bản thì
  cần gộp qua Redis — ghi vào bảng giới hạn đã biết.

`next` chỉ nhận đường dẫn nội bộ (bắt đầu bằng `/`, không bắt đầu bằng `//`),
nếu không đăng nhập xong sẽ bị đẩy sang trang lừa đảo (open redirect).

### 2.3. IP của khách khi đi qua BFF

Mọi request đăng nhập tới API giờ đến từ IP của server Next. Rate limit 5 lần/phút
theo IP (P2.1) thành **5 lần/phút cho toàn bộ khách**. Cách sửa:

- **Next chuyển tiếp nguyên chuỗi `X-Forwarded-For`** nó nhận được. Next tự gắn
  IP socket khi request chưa có header này.
- **API có `TRUSTED_PROXIES`** (danh sách CIDR). Chỉ khi **socket** đến từ dải
  tin cậy, API mới đọc `X-Forwarded-For` từ phải sang trái, bỏ qua các IP tin
  cậy; IP đầu tiên không tin cậy là IP khách. Socket lạ thì chỉ dùng IP socket,
  như cũ.
- **Không dùng thẳng `ClientIPFromXFF` của chi:** hàm đó đọc header bất kể ai
  gửi, nên người nối thẳng vào API tự ghi `X-Forwarded-For` là giả được IP.
- **Mặc định `TRUSTED_PROXIES` rỗng** = không tin ai, đúng như P2.1.
  - Dev compose: `172.16.0.0/12` (mạng Docker).
  - Production: dải mạng nội bộ của compose.
- **Next phải đứng sau reverse proxy.** Next giữ nguyên `X-Forwarded-For` do
  client gửi (`??=`), nên người nối thẳng vào Next giả được IP. Production đã
  vậy: cổng 3000 chỉ mở trên loopback, đứng sau Caddy.

### 2.4. Sổ địa chỉ

**Các trường** (địa chỉ theo 2 cấp hành chính sau sắp xếp 2025):

| Trường | Luật |
|---|---|
| `recipient_name` | 1–100 ký tự |
| `phone` | di động Việt Nam; chuẩn hóa `+84`/`84`/khoảng trắng/dấu chấm về `0xxxxxxxxx` (10 số, đầu `03/05/07/08/09`) |
| `province` | 1–100 ký tự |
| `ward` | 1–100 ký tự |
| `street` | 1–255 ký tự |
| `is_default` | xem các luật bên dưới |

**Luật:**

- Tối đa **10** địa chỉ một người. Vượt thì 422 `ADDRESS_LIMIT_REACHED`.
- Địa chỉ đầu tiên tự thành mặc định.
- Xóa địa chỉ mặc định thì địa chỉ mới nhất còn lại lên làm mặc định.
- Luôn có **đúng một** địa chỉ mặc định (khi còn địa chỉ). Unique index một
  phần `WHERE is_default` giữ luật này ở DB; khóa dòng người dùng xếp hàng
  các thao tác đổi.
- Địa chỉ của người khác trả 404 `UNKNOWN_ADDRESS`, không phải 403: 403 là thừa
  nhận id đó tồn tại.

### 2.5. Header chỉ có link "Tài khoản" tĩnh

Đọc cookie trong layout sẽ biến **mọi** trang thành render động — mất ISR của
trang danh mục và sản phẩm (P0.4). Header chỉ có link "Tài khoản" trỏ
`/tai-khoan`: đã đăng nhập thì vào thẳng, chưa thì proxy chuyển sang đăng
nhập. Hiện tên người dùng trên header để khi có giỏ hàng (P4) làm bằng một
đảo client nhỏ.

### 2.6. Đăng ký vẫn báo `EMAIL_TAKEN`

P2.3 để ngỏ quyết định này. Chốt là **giữ**:

- Giấu nó đòi đăng ký trả 202 rồi gửi thư, tức khách phải mở hộp thư mới
  biết mình đã có tài khoản. Đó là trải nghiệm tệ ở đúng bước dễ bỏ cuộc
  nhất.
- Rate limit 5 lần/phút theo IP **và** theo email (giờ là IP thật của khách)
  làm việc dò hàng loạt đắt.
- Đăng nhập, quên và đặt lại mật khẩu vẫn kín.

## 3. API mới

| Endpoint | Kết quả |
|---|---|
| `PATCH /me` `{full_name}` | 200 `User` |
| `GET /me/addresses` | 200 `{data: Address[]}`, mặc định đứng đầu rồi mới nhất |
| `POST /me/addresses` | 201 `Address`; 422 `VALIDATION_FAILED` (theo trường) / `ADDRESS_LIMIT_REACHED` |
| `PATCH /me/addresses/{id}` | 200 `Address`; 404 `UNKNOWN_ADDRESS` |
| `DELETE /me/addresses/{id}` | 204 |
| `POST /me/addresses/{id}/default` | 200 `Address` |

Tất cả dùng Bearer. Mã lỗi mới: `INVALID_PHONE`, `ADDRESS_FIELD_INVALID` (mã
theo trường), `ADDRESS_LIMIT_REACHED`, `UNKNOWN_ADDRESS`.

## 4. Kiểm chứng

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check` + `npm run typecheck/lint/build` | qua |
| 2 | `/tai-khoan` chưa đăng nhập | 307 → `/dang-nhap?next=%2Ftai-khoan` |
| 3 | Đăng ký trên trang → thư → nhập mã ở `/tai-khoan` | "Email đã xác minh"; cookie `bec_at`/`bec_rt` có HttpOnly, SameSite=Lax |
| 4 | Xóa `bec_at`, giữ `bec_rt`, mở `/tai-khoan` | trang render đúng, `Set-Cookie` có token mới |
| 5 | Hai request song song chỉ mang `bec_rt` | cả hai 200, phiên KHÔNG bị thu hồi |
| 6 | `?next=//evil.com` và `?next=https://evil.com` | đăng nhập xong về `/tai-khoan` |
| 7 | 6 lần đăng nhập sai từ hai IP khác nhau qua web | rate limit tính theo IP khách, không theo IP của Next |
| 8 | Nối thẳng API, tự gửi `X-Forwarded-For` | bị bỏ qua — vẫn là IP socket |
| 9 | Sổ địa chỉ: thêm 2, đổi mặc định, xóa mặc định, SĐT `+84 912.345.678` | luôn đúng một mặc định; SĐT lưu `0912345678` |
| 10 | Địa chỉ của người khác | 404 `UNKNOWN_ADDRESS` |
| 11 | Quên mật khẩu → đặt lại trên trang → đăng nhập bằng mật khẩu mới | được |
| 12 | Đăng xuất | cookie bị xóa; refresh token cũ 401 |
| 13 | Trang danh mục / sản phẩm | vẫn ISR (không thành dynamic vì cookie) |
