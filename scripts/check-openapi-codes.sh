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
#
# Cả hai phía đều phân tích cú pháp thật, KHÔNG grep:
#   - phía Go  : apps/api/cmd/checkcodes dùng go/ast — đọc được cả khi gofmt
#                xuống dòng, tra được hằng, và BÁO LỖI khi không phân tích được
#   - phía YAML: PyYAML đọc đúng components.schemas.Problem.properties.code.enum
#
# Bản đầu tiên của script này dùng grep cho cả hai phía và sai ở cả hai phía.
# Xem phần chú thích đầu cmd/checkcodes/main.go để biết ba lỗ hổng cụ thể.
set -euo pipefail

cd "$(dirname "$0")/.."
fail=0

# Không dùng `command -v python3`: trên Windows nó tìm thấy shim rỗng của
# Microsoft Store, chạy vào là hỏng. Phép thử đáng tin duy nhất là import thật.
PY_BIN=""
for c in python3 python py; do
  if "$c" -c "import yaml" >/dev/null 2>&1; then PY_BIN="$c"; break; fi
done
if [ -z "$PY_BIN" ]; then
  echo "LỖI: cần Python có PyYAML để đọc api/openapi.yaml (pip install pyyaml)"
  exit 2
fi

# tr -d: Python trên Windows in CRLF, comm sẽ coi "MÃ\r" khác "MÃ".
go_codes="$( (cd apps/api && go run ./cmd/checkcodes .) | tr -d '\r' | sort -u )"

yaml_codes="$("$PY_BIN" - <<'PY' | tr -d '\r' | sort -u
import io, sys, yaml
d = yaml.safe_load(io.open('api/openapi.yaml', encoding='utf-8'))
try:
    enum = d['components']['schemas']['Problem']['properties']['code']['enum']
except (KeyError, TypeError):
    sys.stderr.write('khong tim thay components.schemas.Problem.properties.code.enum\n')
    sys.exit(2)
print('\n'.join(enum))
PY
)"

if [ -z "$yaml_codes" ]; then
  echo "LỖI: không đọc được enum mã lỗi từ api/openapi.yaml"
  exit 2
fi

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
