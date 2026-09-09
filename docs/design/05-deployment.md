# 05 — Triển khai & vận hành

---

## 1. Nguyên tắc

1. **Image bất biến.** Cùng một image chạy ở staging và production, chỉ khác biến
   môi trường. Không bao giờ build lại khi deploy.
2. **Migration là bước riêng**, chạy trước khi khởi động phiên bản mới. Không bao
   giờ gọi trong `main()` — hai bản app khởi động cùng lúc sẽ tranh nhau chạy migration.
3. **Migration luôn tương thích ngược.** Đây là điều kiện để rollback bằng cách
   quay lại image cũ mà không phải đụng vào database.
4. **Rollback = đổi tag image.** Database gần như không bao giờ rollback.

---

## 2. Build

### 2.1. Go

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download                       # tầng này được cache khi go.mod không đổi
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

- `CGO_ENABLED=0` → binary tĩnh, chạy được trên distroless (~15 MB toàn bộ image)
- `-trimpath` → không nhúng đường dẫn máy build vào binary
- `version` nhúng lúc build, hiện ở `/healthz` và gắn vào Sentry release
- Chạy bằng user `nonroot`, không phải root

Ba binary (`api`, `worker`, `outboxrelay`) dùng chung Dockerfile, khác `--build-arg`.

### 2.2. Next.js

`output: 'standalone'` trong `next.config.js` — image giảm từ ~1 GB xuống ~150 MB
vì chỉ chép những package thật sự được dùng.

⚠️ Biến `NEXT_PUBLIC_*` bị **nhúng vào bundle lúc build**, không đọc lúc chạy. Nghĩa
là image của staging và production khác nhau nếu chúng trỏ tới API khác nhau. Cách
tránh: dùng đường dẫn tương đối `/api` và để reverse proxy định tuyến — giữ đúng
nguyên tắc "một image cho mọi môi trường".

### 2.3. Đặt tag

```
ghcr.io/<org>/base-ecommerce-api:a1b2c3d      # git SHA — dùng để deploy
ghcr.io/<org>/base-ecommerce-api:v1.4.0       # tag release
```

**Không bao giờ deploy `latest`.** Không truy được đang chạy code nào, và
`docker compose pull` có thể lấy về image khác với cái vừa test.

---

## 3. Bố trí production

```
                    Cloudflare (CDN + WAF)
                            │
                    Caddy (TLS tự động)
                    ┌───────┴───────┐
                  web            /api/*
              (Next.js ×2)    (api ×2)
                                  │
        ┌─────────────────────────┼──────────────────┐
   PostgreSQL                  Redis            RabbitMQ
        │                                            │
        └────── outboxrelay ×1 ────► worker ×N ──────┘
```

| Dịch vụ | Số bản | Ghi chú |
|---|---|---|
| `api` | 2 | Để rolling restart không mất request |
| `web` | 2 | |
| `worker` | 1–N | Scale theo độ trễ hàng đợi |
| `outboxrelay` | **đúng 1** | Nhiều bản sẽ phá vỡ thứ tự event |
| `postgres` | 1 | Máy riêng khi đủ lớn |
| `redis` | 2 instance logic | `cache` và `data` — xem tài liệu 03 |

### ⚠️ Việc bắt buộc khi lên production: cấu hình IP client

Router hiện dùng `middleware.ClientIPFromRemoteAddr` — chỉ lấy IP từ socket, không
tin header nào. Đó là mặc định an toàn cho môi trường dev không có proxy.

Nhưng ở sơ đồ trên, API nằm sau **hai** lớp (Cloudflare rồi Caddy), nên IP socket
sẽ luôn là IP của Caddy. Mọi request trông như đến từ cùng một IP → log vô dụng và
rate limit theo IP chặn nhầm toàn bộ khách cùng lúc.

Trước khi mở cho khách thật, đổi trong `internal/server/router.go`:

```go
r.Use(middleware.ClientIPFromXFFTrustedProxies(2))   // Cloudflare + Caddy
```

Con số phải khớp **chính xác** số proxy đứng trước. Đặt cao quá thì client bịa
thêm phần tử vào `X-Forwarded-For` là giả mạo được IP; đặt thấp quá thì lấy nhầm
IP của proxy. Kiểm chứng bằng cách gọi qua đường công khai rồi so `ip` trong log
với IP thật của máy gọi.

**Không dùng `middleware.RealIP`** — đã deprecated vì tin header vô điều kiện
(GHSA-3fxj-6jh8-hvhx).

**Cấu hình VPS khởi điểm:** 4 vCPU / 8 GB cho tầng ứng dụng, 4 vCPU / 8 GB riêng
cho Postgres. Không đặt Postgres chung máy với app khi đã có đơn hàng thật — một
đợt build ngốn CPU sẽ kéo theo cả database.

### 3.1. Health check trong compose

```yaml
api:
  image: ghcr.io/org/base-ecommerce-api:${VERSION}
  healthcheck:
    test: ["CMD", "/api", "healthcheck"]     # binary tự gọi /healthz, không cần curl
    interval: 10s
    timeout: 3s
    retries: 3
    start_period: 15s
  depends_on:
    postgres: { condition: service_healthy }
    redis:    { condition: service_healthy }
```

Distroless không có `curl`/`wget` — thêm subcommand `healthcheck` vào chính binary.

`/healthz` = tiến trình còn sống (luôn 200 khi chưa tắt).
`/readyz` = ping được Postgres, Redis, RabbitMQ. **Hai cái này khác nhau, đừng gộp.**
Gộp lại thì Redis chập chờn sẽ khiến Docker khởi động lại một API vốn vẫn phục vụ tốt.

---

## 4. Graceful shutdown

Sai thứ tự ở đây sẽ làm mất đơn hàng lúc deploy.

### 4.1. `api`

```go
<-ctx.Done()                              // nhận SIGTERM

ready.Store(false)                        // 1. /readyz trả 503
time.Sleep(5 * time.Second)               // 2. chờ proxy nhận ra và ngừng gửi request

shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = srv.Shutdown(shutCtx)                 // 3. xử lý nốt request đang dở

publisher.Close()                         // 4. đóng theo chiều ngược lúc khởi tạo
redisClient.Close()
pool.Close()
```

Bước 2 là bước hay bị bỏ sót nhất. Không có nó, proxy vẫn gửi request tới trong
lúc server đã bắt đầu tắt → khách nhận 502 ngay giữa lúc thanh toán.

Đặt `stop_grace_period: 45s` trong compose — dài hơn tổng thời gian trên, nếu
không Docker sẽ `SIGKILL` giữa chừng.

### 4.2. `worker`

```
SIGTERM → ngừng nhận message mới (cancel consumer)
        → chờ message đang xử lý xong, tối đa 30s
        → nack(requeue=true) những message chưa kịp xử lý
        → đóng kết nối
```

Vì consumer đã idempotent (tài liệu 01), message bị requeue và xử lý lại không gây
hại. Đó là lý do tính idempotent không phải chuyện tùy chọn.

---

## 5. Migration — expand/contract

**Quy tắc: một lần deploy không bao giờ vừa đổi schema theo kiểu phá vỡ, vừa đổi code.**
Tách thành nhiều lần deploy, mỗi lần đều rollback được.

Ví dụ đổi tên cột `price` → `base_price`:

| Bước | Migration | Code | Rollback được? |
|---|---|---|---|
| 1 | `ADD COLUMN base_price NUMERIC` (nullable) | chưa đổi | ✔ |
| 2 | — | ghi **cả hai** cột, đọc `price` | ✔ |
| 3 | backfill `base_price = price` theo lô | — | ✔ |
| 4 | — | đọc `base_price`, vẫn ghi cả hai | ✔ |
| 5 | `SET NOT NULL` trên `base_price` | — | ✔ |
| 6 | — | chỉ dùng `base_price` | ✔ |
| 7 | `DROP COLUMN price` | — | ✔ (bản cũ đã không dùng nữa) |

Rườm rà, nhưng đây là cái giá của việc rollback được bất cứ lúc nào.

**Cấm trong migration:**
- `DROP COLUMN` / `DROP TABLE` cùng lần deploy với code dùng nó
- `ALTER COLUMN ... SET NOT NULL` khi chưa backfill xong
- `CREATE INDEX` không có `CONCURRENTLY` trên bảng lớn (khóa ghi toàn bảng)
- `UPDATE` toàn bảng trong một transaction (bloat + khóa lâu) — chia lô 1000 dòng

⚠️ **`CONCURRENTLY` không chạy được trong transaction block**, mà goose bọc mọi
migration trong một transaction. Migration tạo index phải mở đầu bằng:

```sql
-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS products_attrs_idx ON products USING gin (attributes);
```

Đánh đổi: migration đó **mất tính nguyên tử**. Hỏng giữa chừng sẽ để lại index ở
trạng thái `INVALID` — phải `DROP INDEX CONCURRENTLY` rồi tạo lại bằng tay. Kiểm
tra bằng `SELECT indexrelid::regclass FROM pg_index WHERE NOT indisvalid;`

**Chạy migration:**

```bash
docker compose run --rm migrate goose -dir db/migrations postgres "$DSN" up
```

Container một lần, chạy **trước** khi deploy app. Goose tự dùng bảng
`goose_db_version`; thêm `pg_advisory_lock` bao ngoài để hai lần deploy đồng thời
không tranh nhau.

---

## 6. Quy trình deploy

```
1. CI xanh trên main (build, vet, lint, arch, openapi-drift)
2. Build và push image, tag = git SHA
3. Sao lưu database (snapshot nhanh trước mọi lần deploy có migration)
4. Chạy migration
5. Cập nhật VERSION trong .env, docker compose up -d --no-deps api
   → Compose thay lần lượt từng bản, health check xanh mới chuyển sang bản tiếp theo
6. Deploy web, worker, outboxrelay
7. Kiểm tra: /healthz có đúng version, Sentry không có lỗi mới, độ trễ hàng đợi ~0
```

**Rollback:** đặt lại `VERSION` về SHA trước đó, `docker compose up -d`. Dưới một
phút. Không đụng database — nhờ mục 5 nên bản cũ vẫn chạy được với schema mới.

---

## 7. Cấu hình & bí mật

- Toàn bộ qua biến môi trường, `.env.example` là danh sách đầy đủ và luôn cập nhật
- `platform/config` **validate lúc khởi động**, thiếu biến thì thoát ngay với thông
  báo rõ ràng — không để lỗi lộ ra lúc 3 giờ sáng khi có request đầu tiên chạm tới
- Bí mật production: **SOPS** mã hóa bằng `age`, file `.env.enc` commit được vào
  git, giải mã trên máy chủ lúc deploy
- Không bao giờ log giá trị bí mật. `config.String()` phải che các trường nhạy cảm
- Xoay vòng khóa: JWT secret, khóa cổng thanh toán — mỗi 6 tháng, có quy trình ghi lại

---

## 8. Sao lưu

| | |
|---|---|
| Công cụ | **pgBackRest** (hoặc wal-g) |
| Lịch | Full hằng ngày 03:00 + WAL archive liên tục |
| Lưu giữ | 7 bản full, WAL 14 ngày |
| Nơi lưu | Object storage **khác nhà cung cấp VPS** |
| Khôi phục điểm thời gian | Có (PITR), nhờ WAL archive |

**Diễn tập khôi phục mỗi quý.** Sao lưu chưa từng được khôi phục thử thì chưa
phải sao lưu. Ghi lại thời gian khôi phục thực tế — đó mới là RTO thật.

Ngoài ra sao lưu: định nghĩa RabbitMQ (`definitions.json`), Redis DB `data` (AOF),
MinIO (từ P1 trở đi).

---

## 9. Giám sát & cảnh báo

| Chỉ số | Ngưỡng | Mức độ |
|---|---|---|
| Tỉ lệ lỗi 5xx | > 1% trong 5 phút | Khẩn cấp |
| Độ trễ API p99 | > 1s trong 5 phút | Cảnh báo |
| Độ sâu hàng đợi outbox chưa gửi | > 1000 hoặc cũ hơn 5 phút | Khẩn cấp |
| Message trong DLQ | > 0 | Cảnh báo |
| Kết nối Postgres đang dùng | > 80% `max_connections` | Cảnh báo |
| Replication lag (khi có replica) | > 10s | Cảnh báo |
| Dung lượng đĩa | > 80% | Cảnh báo |
| Redis `evicted_keys` trên instance `data` | > 0 | Khẩn cấp |
| Webhook thanh toán thất bại | > 0 | Khẩn cấp |
| Chứng chỉ TLS | hết hạn < 14 ngày | Cảnh báo |

Cảnh báo phải gửi tới nơi thật sự có người đọc (Telegram/Slack). Cảnh báo không ai
đọc thì bằng không có.

---

## 10. Sổ tay xử lý sự cố

**Outbox ứ đọng** → xem `outboxrelay` còn sống không; RabbitMQ đầy đĩa? Kiểm tra
`attempts` và `last_error` của các dòng chưa gửi.

**API chậm đột ngột** → `pg_stat_activity` tìm query chạy lâu; kiểm tra pool đã
cạn chưa; kiểm tra tỉ lệ hit của Redis.

**Cạn kết nối Postgres** → tìm chỗ rò rỉ (`pool.Acquire` quên `Release`); tạm thời
giảm `MaxConns` của worker; xem xét PgBouncer.

**DLQ có message** → đọc `last_error`, sửa lỗi, chuyển message về queue chính.
**Không bao giờ xóa DLQ mà chưa đọc** — đó là những sự kiện nghiệp vụ đã mất.

**Nghi ngờ bán vượt** → dừng bán SKU đó ngay, đối chiếu `inventory` với tổng đơn
đã đặt, rồi mới tìm nguyên nhân.

---

## 11. Việc cần làm

- [ ] Dockerfile multi-stage cho api/worker/outboxrelay (distroless, nonroot)
- [ ] Dockerfile cho web (`output: standalone`)
- [ ] Subcommand `healthcheck` trong binary Go
- [ ] `/healthz` và `/readyz` tách biệt, `/healthz` trả version
- [ ] Graceful shutdown đúng thứ tự cho cả 3 binary + `stop_grace_period`
- [ ] `deploy/compose.prod.yml` + cấu hình Caddy
- [ ] Container migration một lần + `pg_advisory_lock`
- [ ] SOPS + `age`, quy trình giải mã lúc deploy
- [ ] CI: build và push image gắn tag SHA lên GHCR
- [ ] Script deploy + script rollback
- [ ] pgBackRest + kiểm thử khôi phục lần đầu
- [ ] Sentry (gắn release theo version), Prometheus + Grafana, cảnh báo mục 9
- [ ] Viết sổ tay sự cố vào `docs/runbook.md` khi gặp lần đầu
