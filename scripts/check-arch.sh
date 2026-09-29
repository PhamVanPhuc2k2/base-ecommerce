#!/usr/bin/env bash
# Kiểm tra chiều phụ thuộc của Clean Architecture bằng máy, không bằng tự giác.
# Quy tắc đầy đủ ở README mục 3.
#
# Dự án không có unit test, nên script này cùng golangci-lint là toàn bộ lưới an
# toàn tự động. Đừng làm yếu nó đi.
#
# ⚠️ KHÔNG dùng mẫu kiểu ./internal/*/domain/... cho go list. Bash chỉ mở rộng `*`
# khi TOÀN BỘ mẫu khớp đường dẫn có thật, mà không có thư mục nào tên `...`, nên
# nó truyền nguyên chuỗi cho go list, go list lỗi, và lỗi bị nuốt — script luôn
# báo OK dù có vi phạm. Cách đúng: liệt kê ./internal/... ./pkg/... rồi lọc.
set -euo pipefail

cd "$(dirname "$0")/../apps/api"
fail=0

all_pkgs="$(go list ./internal/... ./pkg/... 2>/dev/null || true)"

# 1. domain chỉ được import stdlib và ba package trong danh sách trắng.
#
# Đây là ALLOWLIST, không phải denylist. Denylist chỉ chặn được những thứ ta
# nghĩ ra trước; allowlist chặn mọi thứ chưa được cho phép — đúng như README
# mục 3.1 hứa ("muốn thêm gì nữa phải sửa tài liệu này trước").
ALLOWED_DOMAIN_DEPS='^(base-ecommerce/api/pkg/errs|github\.com/google/uuid|github\.com/shopspring/decimal)$'

for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/domain(/|$)' || true); do
  # Lọc lấy package trong dự án và package bên thứ ba; stdlib có thành phần
  # đầu không chứa dấu chấm nên bị loại. Bỏ chính nó ra khỏi danh sách.
  offenders="$(go list -deps "$pkg" 2>/dev/null \
    | grep -E '^(base-ecommerce/|[^/]+\.[^/]+/)' \
    | grep -v "^$pkg\$" \
    | grep -Ev "$ALLOWED_DOMAIN_DEPS" || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg import package ngoài danh sách trắng"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done

# 2. usecase chỉ được import domain, errs và hai package tính toán thuần.
#
# ALLOWLIST giống rule 1. Bản denylist cũ chỉ xét import TRỰC TIẾP, nên pgx,
# redis và chi đều lọt, và pkg/httpx kéo net/http vào cũng lọt.
ALLOWED_USECASE_DEPS='^(base-ecommerce/api/internal/domain(/.*)?|base-ecommerce/api/pkg/errs|github\.com/google/uuid|github\.com/shopspring/decimal)$'

for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/usecase(/|$)' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null \
    | grep -E '^(base-ecommerce/|[^/]+\.[^/]+/)' \
    | grep -v "^$pkg\$" \
    | grep -Ev "$ALLOWED_USECASE_DEPS" || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg import package ngoài danh sách trắng"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done

# 3. repository phải nhận DBTX qua Manager.DB(ctx), không được tự giữ pool.
#
# Kiểm import TRỰC TIẾP thay vì grep mã nguồn: grep vừa bắn nhầm vào comment,
# vừa bị qua mặt bởi `import pp "..."` rồi dùng pp.Pool. Dùng .Imports chứ không
# phải -deps vì repository hoàn toàn có quyền chạm pgxpool gián tiếp qua
# pkg/postgres.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/repository(/|$)' || true); do
  if go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null \
     | grep -q '^github\.com/jackc/pgx/v5/pgxpool$'; then
    echo "LỖI KIẾN TRÚC: $pkg import pgxpool trực tiếp — phải nhận DBTX qua Manager.DB(ctx)"
    fail=1
  fi
done

# 4. pkg/ không được biết tới BẤT CỨ thứ gì trong internal/.
#
# pkg/ là hạ tầng dùng chung (config, errs, httpx, postgres, redis, rabbitmq...).
# Chỉ cần một lần "tiện tay" import internal/domain để lấy một hằng số là mọi
# nghiệp vụ dùng pkg/ đó kéo theo cả catalog vào đồ thị phụ thuộc — và đó là
# lúc hệ thống biến thành big ball of mud.
#
# Dùng -deps chứ không phải .Imports: vi phạm gián tiếp (pkg A import pkg B, B
# import internal/domain) cũng nguy hiểm y hệt và khó thấy hơn nhiều.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '^base-ecommerce/api/pkg/' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null | grep '^base-ecommerce/api/internal/' || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg (hạ tầng dùng chung) phụ thuộc vào internal/"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    echo "    → pkg/ không được biết nghiệp vụ nào tồn tại. Việc dịch từ kiểu"
    echo "      nghiệp vụ sang kiểu hạ tầng là của tầng repository."
    fail=1
  fi
done

# 5. repository/outbox là hạ tầng outbox dùng chung — cùng luật với pkg/.
#
# Nó nằm trong internal/repository vì đọc ghi database, nhưng mọi nghiệp vụ đều
# dùng nó nên nó không được biết domain hay usecase. Dịch domain.Event sang
# outbox.Record là việc của repository/outboxpub.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/repository/outbox(/|$)' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null \
    | grep -E '^base-ecommerce/api/internal/(domain|usecase|delivery)(/|$)' || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg (outbox dùng chung) phụ thuộc vào nghiệp vụ"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done

# 6. delivery và repository không được biết nhau.
#
# Handler gọi usecase, usecase gọi repository qua interface. Handler gọi thẳng
# repository là bỏ qua toàn bộ quy tắc nghiệp vụ, transaction và việc ghi sự
# kiện nằm trong usecase — request vẫn chạy, chỉ là chạy sai. Chiều ngược lại
# (repository import delivery) kéo net/http vào tầng dữ liệu.
#
# Chỉ cmd/ (composition root) được biết cả hai.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/delivery(/|$)' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null | grep -E '^base-ecommerce/api/internal/repository(/|$)' || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg (delivery) gọi thẳng repository — phải đi qua usecase"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/internal/repository(/|$)' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null | grep -E '^base-ecommerce/api/internal/delivery(/|$)' || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg (repository) phụ thuộc vào delivery"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done

if [ "$fail" -eq 0 ]; then
  echo "Kiểm tra kiến trúc: OK"
fi
exit "$fail"
