# Đối chiếu cây thư mục ở README mục 4 với đĩa thật.
#
# Vì sao cần: check-arch.sh canh chiều phụ thuộc, check-openapi-codes.sh canh mã
# lỗi — nhưng KHÔNG có gì canh "tài liệu có mô tả đúng dự án không". Và nó đã
# lệch thật: README từng ghi platform/validate/ (không tồn tại), pgstore/
# repository.go (thật ra là ba file khác tên), và bỏ sót cả logpublisher/ lẫn
# cmd/checkcodes/.
#
# Luật:
#   - dòng KHÔNG có dấu ⬜  -> đường dẫn PHẢI tồn tại
#   - dòng CÓ dấu ⬜ Pxx    -> đường dẫn PHẢI CHƯA tồn tại; làm xong thì bỏ dấu đi
#
# Vế thứ hai quan trọng không kém vế thứ nhất: nó buộc tài liệu được cập nhật
# đúng lúc thứ đó được xây, thay vì để dấu ⬜ nằm lại vĩnh viễn.

import io
import os
import re
import sys

README = "README.md"
SECTION = "## 4. Cấu trúc thư mục"

BRANCH = re.compile(r"^(?P<indent>(?:[│ ] {3})*)(?:├──|└──) (?P<name>[^ #]+)")


def parse_tree(lines):
    """Trả về danh sách (đường_dẫn, chưa_làm) suy ra từ cây ASCII."""
    out = []
    stack = []  # stack[i] = tên thư mục ở độ sâu i
    for raw in lines:
        m = BRANCH.match(raw)
        if not m:
            continue
        depth = len(m.group("indent")) // 4
        name = m.group("name").rstrip("/")
        # Bỏ qua nhánh chỉ mang tính minh họa (có ký tự đại diện)
        if "*" in name or name in ("...",):
            continue
        del stack[depth:]
        stack.append(name)
        pending = "⬜" in raw
        out.append(("/".join(stack), pending))
    return out


def main():
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    text = io.open(os.path.join(root, README), encoding="utf-8").read()

    if SECTION not in text:
        print("LỖI: không tìm thấy '%s' trong README.md" % SECTION)
        return 2
    after = text.split(SECTION, 1)[1]
    if "```" not in after:
        print("LỖI: mục 4 không có khối cây thư mục")
        return 2
    block = after.split("```", 2)[1]

    entries = parse_tree(block.split("\n"))
    if len(entries) < 20:
        # Cây thật có ~60 dòng. Ít hơn nhiều nghĩa là regex không còn khớp định
        # dạng — phải đỏ, chứ không phải lặng lẽ kiểm 3 dòng rồi báo OK.
        print("LỖI: chỉ phân tích được %d mục từ cây — regex không khớp định dạng nữa" % len(entries))
        return 2

    missing, unexpected = [], []
    for path, pending in entries:
        # Bỏ tên thư mục gốc "base-ecommerce/" khỏi đầu đường dẫn
        rel = path.split("/", 1)[1] if path.startswith("base-ecommerce/") else path
        exists = os.path.exists(os.path.join(root, rel))
        if pending and exists:
            unexpected.append(rel)
        elif not pending and not exists:
            missing.append(rel)

    if missing:
        print("LỖI TÀI LIỆU: README mục 4 mô tả đường dẫn KHÔNG tồn tại")
        for m in missing:
            print("    " + m)
        print("    → sửa README, hoặc tạo thứ đó, hoặc đánh dấu ⬜ Pxx nếu chưa làm")
    if unexpected:
        print("LỖI TÀI LIỆU: đường dẫn đánh dấu ⬜ (chưa làm) nhưng ĐÃ tồn tại")
        for u in unexpected:
            print("    " + u)
        print("    → bỏ dấu ⬜ đi, thứ này đã được xây rồi")

    if missing or unexpected:
        return 1
    print("Đối chiếu cây thư mục: OK (%d mục)" % len(entries))
    return 0


if __name__ == "__main__":
    sys.exit(main())
