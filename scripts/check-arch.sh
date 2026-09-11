#!/usr/bin/env bash
# Kiểm tra chiều phụ thuộc của kiến trúc hexagonal bằng máy, không bằng tự giác.
# Quy tắc đầy đủ ở README mục 3.
#
# Dự án không có unit test, nên script này cùng golangci-lint là toàn bộ lưới an
# toàn tự động. Đừng làm yếu nó đi.
#
# ⚠️ KHÔNG dùng mẫu kiểu ./internal/*/domain/... cho go list. Bash chỉ mở rộng `*`
# khi TOÀN BỘ mẫu khớp đường dẫn có thật, mà không có thư mục nào tên `...`, nên
# nó truyền nguyên chuỗi cho go list, go list lỗi, và lỗi bị nuốt — script luôn
# báo OK dù có vi phạm. Cách đúng: liệt kê ./internal/... rồi lọc bằng grep.
set -euo pipefail

cd "$(dirname "$0")/../apps/api"
fail=0

all_pkgs="$(go list ./internal/... 2>/dev/null || true)"

# 1. domain chỉ được import stdlib và ba package trong danh sách trắng.
#
# Đây là ALLOWLIST, không phải denylist. Denylist chỉ chặn được những thứ ta
# nghĩ ra trước; allowlist chặn mọi thứ chưa được cho phép — đúng như README
# mục 3.1 hứa ("muốn thêm gì nữa phải sửa tài liệu này trước").
ALLOWED_DOMAIN_DEPS='^(base-ecommerce/api/internal/platform/errs|github\.com/google/uuid|github\.com/shopspring/decimal)$'

for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/domain(/|$)' || true); do
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

# 2. app chỉ được import domain, errs và hai package tính toán thuần.
#
# ALLOWLIST giống rule 1. Bản denylist cũ chỉ xét import TRỰC TIẾP, nên pgx,
# redis và chi đều lọt, và platform/httpx kéo net/http vào cũng lọt.
ALLOWED_APP_DEPS='^(base-ecommerce/api/internal/catalog/domain|base-ecommerce/api/internal/platform/errs|github\.com/google/uuid|github\.com/shopspring/decimal)$'

for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/app(/|$)' || true); do
  offenders="$(go list -deps "$pkg" 2>/dev/null \
    | grep -E '^(base-ecommerce/|[^/]+\.[^/]+/)' \
    | grep -v "^$pkg\$" \
    | grep -Ev "$ALLOWED_APP_DEPS" || true)"
  if [ -n "$offenders" ]; then
    echo "LỖI KIẾN TRÚC: $pkg import package ngoài danh sách trắng"
    printf '%s\n' "$offenders" | sed 's/^/    /'
    fail=1
  fi
done

# 3. adapter phải nhận DBTX qua Manager.DB(ctx), không được tự giữ pool.
#
# Kiểm import TRỰC TIẾP thay vì grep mã nguồn: grep vừa bắn nhầm vào comment,
# vừa bị qua mặt bởi `import pp "..."` rồi dùng pp.Pool. Dùng .Imports chứ không
# phải -deps vì adapter hoàn toàn có quyền chạm pgxpool gián tiếp qua
# platform/postgres.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/adapter(/|$)' || true); do
  if go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null \
     | grep -q '^github\.com/jackc/pgx/v5/pgxpool$'; then
    echo "LỖI KIẾN TRÚC: $pkg import pgxpool trực tiếp — phải nhận DBTX qua Manager.DB(ctx)"
    fail=1
  fi
done

# 4. Hạ tầng dùng chung không được biết tới module nghiệp vụ nào.
#
# internal/platform/** và internal/outbox là thứ MỌI module dùng. Chỉ cần một
# lần "tiện tay" import catalog/domain để lấy một hằng số là module orders ở P4
# sẽ kéo theo cả catalog vào đồ thị phụ thuộc của nó — và đó là lúc modular
# monolith biến thành big ball of mud.
#
# Dùng -deps chứ không phải .Imports: vi phạm gián tiếp (hạ tầng A import hạ
# tầng B, B import catalog) cũng nguy hiểm y hệt và khó thấy hơn nhiều.
#
# Danh sách module nghiệp vụ để ở một chỗ, thêm module mới thì thêm vào đây.
business_modules='catalog'

for pkg in $(printf '%s
' "$all_pkgs" | grep -E '/internal/(platform/|outbox)' || true); do
  for m in $business_modules; do
    if go list -deps "$pkg" 2>/dev/null | grep -q "^base-ecommerce/api/internal/$m\(/\|$\)"; then
      echo "LỖI KIẾN TRÚC: $pkg (hạ tầng dùng chung) phụ thuộc vào module nghiệp vụ $m"
      echo "    → hạ tầng không được biết module nào tồn tại. Việc dịch từ kiểu"
      echo "      nghiệp vụ sang kiểu hạ tầng là của adapter thuộc chính module đó."
      fail=1
    fi
  done
done

if [ "$fail" -eq 0 ]; then
  echo "Kiểm tra kiến trúc: OK"
fi
exit "$fail"
