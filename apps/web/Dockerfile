# Image cho storefront Next.js.
#
#   docker build -t base-ecommerce-web:dev apps/web
#
# Build context là thư mục apps/web (xem deploy/compose.dev.yml).
#
# Khác apps/api/Dockerfile ở một điểm nền tảng: KHÔNG dùng được distroless.
# gcr.io/distroless/static chỉ chứa ca-certificates và tzdata, không có Node,
# mà Next.js là một tiến trình Node — nó cần runtime thật. Đổi lại ta siết
# những gì siết được: tầng chạy là node:24-alpine (không có toolchain build),
# user không phải root, và chỉ chép đúng output của `next build` vào.

FROM node:24-alpine AS build
WORKDIR /src

# libc6-compat: bản nhị phân của SWC (trình biên dịch của Next.js) được dựng cho
# glibc, còn Alpine dùng musl. Thiếu gói này thì `next build` chết ngay ở bước
# nạp native binding, với thông điệp về file .so không tìm thấy — hoàn toàn
# không gợi ý gì tới nguyên nhân thật.
RUN apk add --no-cache libc6-compat

# Chép package.json + package-lock.json TRƯỚC rồi mới chép mã nguồn, cùng lý do
# với `go mod download` ở backend: tầng cài dependency được cache lại khi chỉ có
# code thay đổi, sửa một dòng TSX không phải tải lại toàn bộ node_modules.
COPY package.json package-lock.json ./

# `npm ci` chứ KHÔNG phải `npm install`: ci cài đúng cây phụ thuộc đã ghim trong
# package-lock.json và báo lỗi nếu lock lệch với package.json. `npm install` thì
# âm thầm nâng phiên bản và ghi đè lock — nghĩa là image production có thể mang
# một phiên bản thư viện chưa ai kiểm thử.
RUN npm ci

COPY . .

# Tắt telemetry: build trong container không nên phát sinh lời gọi mạng ra ngoài
# ngoài những gì ta chủ ý.
ENV NEXT_TELEMETRY_DISABLED=1

# ⚠️ Bước này CÓ phụ thuộc mạng: app/layout.tsx dùng next/font/google, nên
# `next build` tải file font từ fonts.gstatic.com. Máy build không ra được
# Google Fonts thì build hỏng. Đã kiểm bằng `docker build --no-cache` ở Task 6.
# Muốn cắt hẳn phụ thuộc này thì chuyển sang next/font/local với .woff2 commit
# vào repo — xem docs/superpowers/specs/2026-09-11-p0-4-frontend-design.md.
RUN npm run build

FROM node:24-alpine AS runtime
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# ⚠️ HAI BIẾN NÀY LÀ BẮT BUỘC, KHÔNG PHẢI TRANG TRÍ.
#
# server.js do `output: 'standalone'` sinh ra mặc định nghe trên `localhost`.
# Trong container, `localhost` là loopback CỦA CHÍNH CONTAINER — cổng có ánh xạ
# ra host cũng không ai vào được, `curl localhost:3000` từ ngoài bị từ chối kết
# nối trong khi log bên trong vẫn in "Ready in ...". Phải là 0.0.0.0.
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

# ⚠️ CÁI BẪY LỚN NHẤT CỦA `output: 'standalone'`.
#
# standalone KHÔNG tự chép `.next/static/` (và `public/`) vào thư mục output —
# nó chỉ gom server code cùng đúng những node_modules cần thiết. Quên hai dòng
# dưới thì image vẫn build được, container vẫn chạy, trang vẫn ra HTML có chữ —
# nhưng mọi file CSS/JS trong /_next/static/ trả về 404 và trang hiện ra trần
# trụi không style. Không có lỗi nào ở log để lần ra.
#
# Dự án hiện KHÔNG có thư mục public/ (favicon nằm ở app/favicon.ico và do Next
# tự phục vụ), nên không có dòng `COPY ... /src/public ./public`. Ngày nào thêm
# public/ thì phải thêm dòng đó, nếu không ảnh tĩnh sẽ 404 đúng theo kiểu trên.
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static

# `node` là user không phải root có sẵn trong image chính thức (uid 1000).
# Không cần tạo user mới.
USER node

EXPOSE 3000

# Dạng exec (JSON array), KHÔNG phải dạng shell — cùng lý do đã đo ở backend:
# `docker stop` gửi SIGTERM cho PID 1, và chỉ khi tiến trình Node thật sự là
# PID 1 thì nó mới nhận được tín hiệu. Viết dạng shell thì PID 1 là /bin/sh,
# sh không chuyển tiếp tín hiệu, container bị SIGKILL sau thời gian chờ và trả
# exit code 137. Xem docs/design/05-deployment.md mục 4.1.
#
# `node server.js` chứ KHÔNG phải `next start`: với output standalone thì
# `next start` từ chối chạy (Next.js báo lỗi và bảo dùng server.js). Ngoài ra
# image này còn không có `next` CLI — standalone chỉ gom runtime cần thiết.
ENTRYPOINT ["node", "server.js"]
