#!/usr/bin/env bash
# Kiểm tra chiều phụ thuộc của kiến trúc hexagonal bằng máy, không bằng tự giác.
# Quy tắc đầy đủ ở README mục 3.
#
# Dự án không có unit test, nên script này cùng golangci-lint là toàn bộ lưới an
# toàn tự động. Đừng làm yếu nó đi.
set -euo pipefail

cd "$(dirname "$0")/../apps/api"
fail=0

# 1. domain không được chạm hạ tầng.
#    Danh sách trắng: stdlib, platform/errs, google/uuid, shopspring/decimal.
for pkg in $(go list ./internal/*/domain/... 2>/dev/null || true); do
  if go list -deps "$pkg" | grep -Eq 'go-chi|jackc/pgx|redis|amqp|net/http$'; then
    echo "LỖI KIẾN TRÚC: $pkg import package hạ tầng"
    go list -deps "$pkg" | grep -E 'go-chi|jackc/pgx|redis|amqp|net/http$' | sed 's/^/    /'
    fail=1
  fi
done

# 2. app không được import net/http hay adapter.
for pkg in $(go list ./internal/*/app/... 2>/dev/null || true); do
  if go list -f '{{join .Imports "\n"}}' "$pkg" | grep -Eq 'net/http|/adapter/'; then
    echo "LỖI KIẾN TRÚC: $pkg import net/http hoặc adapter"
    fail=1
  fi
done

# 3. repository phải dùng DBTX, không được giữ pool trực tiếp.
if grep -rn 'pgxpool\.Pool' internal/*/adapter/ 2>/dev/null; then
  echo "LỖI KIẾN TRÚC: adapter giữ *pgxpool.Pool — phải nhận DBTX qua Manager.DB(ctx)"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "Kiểm tra kiến trúc: OK"
fi
exit "$fail"
