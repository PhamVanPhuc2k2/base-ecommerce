#!/usr/bin/env bash
# Đối chiếu cây thư mục ở README mục 4 với đĩa thật. Logic nằm ở check-tree.py.
#
# check-arch.sh canh chiều phụ thuộc, check-openapi-codes.sh canh mã lỗi — chưa
# có gì canh việc tài liệu mô tả đúng dự án. Và nó đã lệch thật.
set -euo pipefail

cd "$(dirname "$0")/.."

# Không dùng `command -v python3`: trên Windows nó tìm thấy shim rỗng của
# Microsoft Store, chạy vào là hỏng. Phép thử đáng tin duy nhất là chạy thật.
PY_BIN=""
for c in python3 python py; do
  if "$c" -c "import sys" >/dev/null 2>&1; then PY_BIN="$c"; break; fi
done
if [ -z "$PY_BIN" ]; then
  echo "LỖI: không tìm thấy Python để chạy scripts/check-tree.py"
  exit 2
fi

# PYTHONIOENCODING: console Windows mặc định là cp1258, in ⬜ hay tiếng Việt vào
# đó sẽ ném UnicodeEncodeError và script chết vì lý do chẳng liên quan gì tới
# nội dung đang kiểm.
PYTHONIOENCODING=utf-8 "$PY_BIN" scripts/check-tree.py
