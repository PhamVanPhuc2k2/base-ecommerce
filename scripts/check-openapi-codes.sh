#!/usr/bin/env bash
# Đối chiếu danh sách mã lỗi trong api/openapi.yaml với mã lỗi thật trong code Go.
#
# Vì sao cần script này: job openapi-drift (sinh lại type TS rồi kiểm git diff)
# KHÔNG bắt được loại lệch nguy hiểm nhất. Nó chỉ so openapi.yaml với output do
# chính nó sinh ra, không bao giờ nhìn vào code Go. Một file YAML sai hoàn toàn
# về tập mã lỗi vẫn nhất quán với chính nó và vẫn qua được drift check.
#
# Mã lỗi là hợp đồng frontend dùng để map sang thông điệp tiếng Việt. Thiếu một
# mã ở đây nghĩa là frontend gặp mã lạ lúc chạy và không biết hiển thị gì.
set -euo pipefail

cd "$(dirname "$0")/.."
fail=0

# Mã trong code Go: errs.New(Kind, "CODE", ...) hoặc errs.Wrap(err, Kind, "CODE", ...)
go_codes="$(grep -rhoE 'errs\.(New|Wrap)\([^,]+, *(errs\.Kind[A-Za-z]+, *)?"[A-Z_]+"' apps/api \
  | grep -oE '"[A-Z_]+"' | tr -d '"' | sort -u)"

# Mã trong OpenAPI: các dòng "- CODE" nằm trong khối enum của trường code
yaml_codes="$(awk '/^ *code:/{inblock=1} inblock && /^ *- [A-Z_]+$/{print $2} inblock && /^ *request_id:/{inblock=0}' \
  api/openapi.yaml | sort -u)"

missing="$(comm -23 <(printf '%s\n' "$go_codes") <(printf '%s\n' "$yaml_codes"))"
extra="$(comm -13 <(printf '%s\n' "$go_codes") <(printf '%s\n' "$yaml_codes"))"

if [ -n "$missing" ]; then
  echo "LỖI HỢP ĐỒNG: mã lỗi có trong code Go nhưng THIẾU trong api/openapi.yaml"
  printf '%s\n' "$missing" | sed 's/^/    /'
  echo "    → frontend sẽ gặp mã lạ lúc chạy và không biết hiển thị gì"
  fail=1
fi

if [ -n "$extra" ]; then
  echo "LỖI HỢP ĐỒNG: mã lỗi khai trong api/openapi.yaml nhưng code Go KHÔNG sinh ra được"
  printf '%s\n' "$extra" | sed 's/^/    /'
  echo "    → hoặc mã đã bị xóa, hoặc khai thừa. Cả hai đều làm hợp đồng nói dối"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "Đối chiếu mã lỗi OpenAPI: OK ($(printf '%s\n' "$go_codes" | wc -l | tr -d ' ') mã)"
fi
exit "$fail"
