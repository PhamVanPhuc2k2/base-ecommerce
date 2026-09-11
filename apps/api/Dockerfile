# Dockerfile dùng chung cho cả ba binary của backend.
#
#   docker build --build-arg BINARY=api         -t bec-api .
#   docker build --build-arg BINARY=worker      -t bec-worker .
#   docker build --build-arg BINARY=outboxrelay -t bec-outboxrelay .
#
# Build context là thư mục apps/api (xem deploy/compose.dev.yml).

FROM golang:1.26-alpine AS build
WORKDIR /src

# Chép go.mod/go.sum TRƯỚC rồi mới chép mã nguồn. Tầng `go mod download` nhờ đó
# được cache lại khi chỉ có code thay đổi — sửa một dòng Go không phải tải lại
# toàn bộ dependency.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BINARY=api
ARG VERSION=dev

# CGO_ENABLED=0: binary tĩnh, không phụ thuộc libc — bắt buộc để chạy được trên
# distroless/static (image đó KHÔNG có libc).
# -trimpath: không nhúng đường dẫn máy build vào binary.
# -s -w: bỏ bảng ký hiệu và thông tin debug, image nhỏ hơn đáng kể.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/app ./cmd/${BINARY}

# Binary thứ hai, chỉ để HEALTHCHECK gọi được. Xem chú thích ở tầng runtime.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /out/healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot

# distroless/static đã có sẵn ca-certificates và tzdata, nhưng KHÔNG có shell,
# không có package manager, không có busybox. Nghĩa là:
#   - không `docker exec ... sh` được vào container này
#   - healthcheck không dùng được CMD-SHELL hay curl — xem compose
# Đánh đổi có ý thức: bề mặt tấn công gần như bằng không, đổi lấy việc chẩn đoán
# phải làm qua log và endpoint /healthz thay vì chui vào trong.
COPY --from=build /out/app /app
COPY --from=build /out/healthcheck /healthcheck

USER nonroot:nonroot
EXPOSE 8080

# Dạng exec (JSON array), KHÔNG phải dạng shell.
#
# Đây là chi tiết quan trọng nhất của file này. `docker stop` gửi SIGTERM cho
# PID 1. Viết `ENTRYPOINT /app` dạng shell thì PID 1 là /bin/sh, sh không
# chuyển tiếp tín hiệu, nên toàn bộ đoạn graceful shutdown trong main.go không
# bao giờ chạy — container bị SIGKILL sau 10 giây và mọi request đang dở bị cắt
# giữa chừng. Đã dựng lại được điều này: xem docs/design/05-deployment.md mục 6.
ENTRYPOINT ["/app"]
