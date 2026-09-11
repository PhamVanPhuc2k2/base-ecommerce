#!/usr/bin/env bash
# Đối chiếu enum mã lỗi trong api/openapi.yaml với bảng tra thông điệp tiếng
# Việt ở apps/web/lib/errors.ts.
#
# Vì sao cần script này: check-openapi-codes.sh canh "code Go ↔ openapi.yaml",
# nhưng không ai canh nốt đoạn cuối của sợi dây — "openapi.yaml ↔ chữ khách đọc".
# Thiếu một mã trong bảng tra thì không có gì hỏng lúc build, không có gì đỏ lúc
# chạy: khách chỉ nhận câu chung chung "Đã có lỗi xảy ra" đúng vào lúc có sự cố,
# tức là đúng lúc thông điệp cụ thể còn có ích. Đó là loại lỗi không ai phát
# hiện ra cho tới khi khách gọi điện phàn nàn.
#
# Chiều ngược lại cũng phải đỏ: khóa có trong bảng tra mà spec không khai nghĩa
# là bảng đang nói về một mã đã bị xóa hoặc chưa từng tồn tại — dead code mang
# hình dạng tài liệu, và nó khiến người đọc tin nhầm rằng mã đó còn dùng.
set -euo pipefail

cd "$(dirname "$0")/.."
fail=0

TS_FILE="apps/web/lib/errors.ts"

# Hai mã dưới đây do FRONTEND tự sinh, backend không bao giờ trả về nên chúng
# không nằm trong openapi.yaml. Loại trừ TƯỜNG MINH THEO TÊN, không dùng quy
# tắc mơ hồ kiểu "bỏ qua mã không khớp": một quy tắc mơ hồ sẽ âm thầm tha luôn
# cả những khóa gõ sai chính tả mà đáng lẽ phải báo đỏ.
FRONTEND_ONLY=$'UNKNOWN\nNETWORK_ERROR'

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

# Phía TypeScript dùng cách (a): cắt lấy khối `errorMessages` rồi mới bắt khóa
# bằng regex, thay vì grep thô cả file.
#
# Vì sao không grep thô: file có doc comment nhắc tên mã (ví dụ DUPLICATE_SKU
# trong đoạn giải thích cách viết câu). Grep cả file sẽ đếm cả những cái tên
# chỉ được NHẮC TỚI mà chưa hề có thông điệp — script báo xanh trong khi khách
# vẫn thấy câu chung chung.
#
# Vì sao không chọn cách (b) chạy thật bằng `node --experimental-strip-types`:
# cách đó đúng hơn về nguyên tắc, nhưng buộc job CI này phải cài Node đúng
# phiên bản chỉ để đọc một danh sách hằng. Đổi lại, cách (a) mỏng manh trước
# việc đổi định dạng file — nên phần dưới BÁO LỖI nếu không cắt được khối,
# thay vì lặng lẽ trả danh sách rỗng rồi kết luận "OK".
ts_codes="$("$PY_BIN" - "$TS_FILE" <<'PY' | tr -d '\r' | sort -u
import io, re, sys

path = sys.argv[1]
src = io.open(path, encoding='utf-8').read()

# Cắt từ dòng khai báo errorMessages tới dấu } đóng ở cột 0. Bảng tra là object
# phẳng nên dấu đóng ở cột 0 là biên đáng tin.
m = re.search(r'^export const errorMessages: Record<string, string> = \{$(.*?)^\}$',
              src, re.S | re.M)
if not m:
    sys.stderr.write(
        'khong cat duoc khoi errorMessages trong %s\n'
        '  -> hoac bien da bi doi ten/doi kieu, hoac dinh dang da khac.\n'
        '     Sua lai file cho dung khuon, hoac cap nhat regex trong script nay.\n' % path)
    sys.exit(2)

keys = re.findall(r'^\s*([A-Z_]+):', m.group(1), re.M)
if not keys:
    sys.stderr.write('khoi errorMessages trong %s khong co khoa nao\n' % path)
    sys.exit(2)

dup = {k for k in keys if keys.count(k) > 1}
if dup:
    # Object literal của JS cho phép trùng khóa và lặng lẽ lấy cái sau. comm
    # cũng không thấy gì vì đã sort -u. Phải bắt ở đây.
    sys.stderr.write('khoa bi khai trung trong %s: %s\n' % (path, ', '.join(sorted(dup))))
    sys.exit(2)

print('\n'.join(keys))
PY
)"

# Bỏ hai mã riêng của frontend ra khỏi phía TS trước khi so.
ts_codes="$(comm -23 <(printf '%s\n' "$ts_codes") <(printf '%s\n' "$FRONTEND_ONLY" | sort -u))"

# Ngược lại: hai mã đó BẮT BUỘC phải có mặt, vì lib/api/server.ts ném chúng ra
# lúc chạy. Thiếu thì messageFor() rơi về câu mặc định — đúng cái script này
# sinh ra để ngăn.
while IFS= read -r c; do
  if ! grep -qE "^\s*$c:" "$TS_FILE"; then
    echo "LỖI: thiếu mã riêng của frontend '$c' trong $TS_FILE"
    echo "    → lib/api/server.ts ném mã này khi fetch hỏng; không có thông điệp là khách thấy câu mặc định"
    fail=1
  fi
done <<< "$FRONTEND_ONLY"

missing="$(comm -23 <(printf '%s\n' "$yaml_codes") <(printf '%s\n' "$ts_codes"))"
extra="$(comm -13 <(printf '%s\n' "$yaml_codes") <(printf '%s\n' "$ts_codes"))"

if [ -n "$missing" ]; then
  echo "LỖI HỢP ĐỒNG: mã lỗi khai trong api/openapi.yaml nhưng THIẾU trong $TS_FILE"
  printf '%s\n' "$missing" | sed 's/^/    /'
  echo "    → khách sẽ thấy câu chung chung 'Đã có lỗi xảy ra' đúng lúc cần thông điệp cụ thể"
  fail=1
fi

if [ -n "$extra" ]; then
  echo "LỖI HỢP ĐỒNG: khóa có trong $TS_FILE nhưng api/openapi.yaml KHÔNG khai"
  printf '%s\n' "$extra" | sed 's/^/    /'
  echo "    → hoặc gõ sai tên mã, hoặc mã đã bị xóa khỏi hợp đồng. Cả hai đều là chữ chết"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "Đối chiếu thông điệp lỗi: OK ($(printf '%s\n' "$yaml_codes" | wc -l | tr -d ' ') mã + 2 mã riêng của frontend)"
fi
exit "$fail"
