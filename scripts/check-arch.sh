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

# Danh sách package hạ tầng mà domain và app không được chạm tới.
FORBIDDEN='^(github\.com/go-chi/|github\.com/jackc/pgx|github\.com/redis/|github\.com/rabbitmq/|net/http)$'

all_pkgs="$(go list ./internal/... 2>/dev/null || true)"

# 1. domain không được chạm hạ tầng.
#    Danh sách trắng: stdlib, platform/errs, google/uuid, shopspring/decimal.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/domain(/|$)' || true); do
  hits="$(go list -deps "$pkg" 2>/dev/null | grep -E "$FORBIDDEN" || true)"
  if [ -n "$hits" ]; then
    echo "LỖI KIẾN TRÚC: $pkg import package hạ tầng"
    printf '%s\n' "$hits" | sed 's/^/    /'
    fail=1
  fi
done

# 2. app không được import net/http hay adapter.
for pkg in $(printf '%s\n' "$all_pkgs" | grep -E '/app(/|$)' || true); do
  hits="$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null \
          | grep -E '^net/http$|/adapter/' || true)"
  if [ -n "$hits" ]; then
    echo "LỖI KIẾN TRÚC: $pkg import net/http hoặc adapter"
    printf '%s\n' "$hits" | sed 's/^/    /'
    fail=1
  fi
done

# 3. repository phải dùng DBTX, không được giữ pool trực tiếp.
adapter_hits="$(grep -rn --include='*.go' 'pgxpool\.Pool' internal/ 2>/dev/null \
                | grep '/adapter/' || true)"
if [ -n "$adapter_hits" ]; then
  echo "LỖI KIẾN TRÚC: adapter giữ *pgxpool.Pool — phải nhận DBTX qua Manager.DB(ctx)"
  printf '%s\n' "$adapter_hits" | sed 's/^/    /'
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "Kiểm tra kiến trúc: OK"
fi
exit "$fail"
