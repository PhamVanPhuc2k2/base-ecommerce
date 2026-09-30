# P2.1 — Tài khoản và phiên đăng nhập: Thiết kế

> Thuộc P2, xem [tổng quan](2026-09-30-p2-tong-quan.md).

## 1. Phạm vi

**Có:** `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`,
`GET /me`; middleware xác thực Bearer; rate limit các endpoint xác thực.

**Không:** phân quyền (P2.2 — `X-Admin-Key` vẫn giữ cho tới đó), xác minh email
và quên mật khẩu (P2.3), giao diện (P2.4).

## 2. Quyết định

### 2.1. Mật khẩu: argon2id, tham số OWASP

`m = 19 MiB, t = 2, p = 1`, salt 16 byte, lưu dạng chuỗi PHC
(`$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`) — tham số nằm TRONG chuỗi, nên
tăng tham số sau này không vô hiệu mật khẩu cũ. So sánh bằng
`subtle.ConstantTimeCompare`. Mật khẩu 8–128 ký tự (trần để một mật khẩu 10 MB
không thành đòn DoS vào argon2).

### 2.2. Không tiết lộ email nào đã đăng ký — khi ĐĂNG NHẬP

Email không tồn tại và sai mật khẩu trả CÙNG 401 `INVALID_CREDENTIALS`, và email
không tồn tại vẫn chạy argon2 trên một hash giả — không thì thời gian phản hồi
(~vài chục ms vs ~0 ms) tự khai ra email nào có thật.

**Đăng ký thì CÓ lộ** (409 `EMAIL_TAKEN`) — chấp nhận: giấu được cần luồng "gửi
thư xác nhận, người đã có tài khoản nhận thư khác", tức phụ thuộc P2.3. Rate
limit làm việc dò hàng loạt đắt.

### 2.3. Access token: JWT HS256, 15 phút, CHỈ mang định danh

Claims: `sub` (user id), `sid` (id chuỗi phiên), `iat`, `exp`, `iss`. **Không**
mang vai trò/quyền: P2.2 tra quyền mỗi request (có cache) để gỡ quyền có hiệu
lực ngay, không chờ token hết hạn. `JWT_SECRET` ≥ 32 byte; production chặn khởi
động nếu còn giá trị dev (như `S3_SECRET_KEY`).

Chấp nhận: đăng xuất không làm access token đang có hết hạn sớm — nó sống tối đa
15 phút. Muốn thu hồi tức thì phải tra DB mỗi request, là thứ JWT được chọn để
tránh.

### 2.4. Refresh token: xoay vòng, phát hiện dùng lại

Chuỗi ngẫu nhiên 32 byte (base64url). DB giữ `sha256(token)` — lộ bảng
`refresh_tokens` không lộ được token dùng được. Mỗi lần refresh: token cũ bị
đánh dấu `used_at`, token mới cùng `family_id`. Token đã `used_at` (hay đã thu
hồi) mà bị đưa lên lần nữa → **thu hồi CẢ family** và trả 401: hoặc kẻ trộm hoặc
chủ thật đang cầm bản sao, không phân biệt được, nên cắt cả hai.

Hai request refresh đồng thời với cùng token (hai tab) là ca THẬT, không phải
tấn công — nhưng không phân biệt được với tấn công. Khóa dòng `FOR UPDATE`: bên
sau thấy `used_at` và kéo sập phiên. Chấp nhận ở P2.1; storefront (P2.4) chỉ
refresh từ server, một chỗ, nên ca này hiếm.

Refresh sống 30 ngày; đăng xuất thu hồi cả family.

### 2.5. Storefront là BFF — API không đặt cookie

API nhận `Authorization: Bearer`, trả token trong JSON. Next.js (P2.4) giữ token
trong cookie HttpOnly **của chính nó**; trình duyệt không bao giờ gọi thẳng API,
nên không cần CORS có credential. Admin dùng curl với Bearer như cũ.

### 2.6. Rate limit: Redis INCR + EXPIRE, theo IP và theo email

Đăng nhập, đăng ký, refresh: 5/phút/IP **và** 5/phút/email (refresh: theo
IP). Vượt → 429 `RATE_LIMITED` + `Retry-After`. Redis chết → cho qua (thiết kế
03). IP lấy qua `middleware.GetClientIP` — hiện chỉ tin socket.

## 3. Schema

```sql
users (id, email UNIQUE + CHECK (email = lower(btrim(email))), password_hash,
       full_name, status active|disabled, email_verified_at, created_at, updated_at)
refresh_tokens (id, family_id, user_id, token_hash UNIQUE, expires_at,
                used_at, revoked_at, created_at)
```

(Bản đầu ghi `email citext`. Đổi khi viết migration: domain vốn phải chuẩn hóa
email để so sánh trong Go, nên thêm extension chỉ để so sánh không phân biệt
hoa thường là thừa; ràng buộc CHECK chặn SQL tay ghi email chưa chuẩn hóa.)

## 4. Kiểm chứng

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check` | xanh |
| 2 | Đăng ký → `GET /me` với access token | 200 đúng người |
| 3 | Email hoa/thường (`A@x.vn` vs `a@x.vn`) | cùng một tài khoản; đăng ký lại → 409 |
| 4 | Sai mật khẩu vs email không tồn tại | cùng 401 `INVALID_CREDENTIALS`, thời gian phản hồi xấp xỉ nhau |
| 5 | Refresh → token mới; refresh lại token CŨ | lần hai 401 và token MỚI cũng chết (cả family) |
| 6 | Đăng xuất rồi refresh | 401 |
| 7 | Token hết hạn / chữ ký sai / `alg: none` | 401 |
| 8 | 6 lần đăng nhập sai trong 1 phút | lần 6 → 429 + `Retry-After` |
| 9 | Redis chết | đăng nhập vẫn chạy (fail-open) |
| 10 | Bảng `users`, `refresh_tokens`, log | không có mật khẩu/token thô ở đâu |
| 11 | `APP_ENV=production` + `JWT_SECRET` dev | từ chối khởi động |
