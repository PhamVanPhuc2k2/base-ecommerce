// Lệnh checkcodes in ra mọi mã lỗi mà code Go có thể trả về cho client, mỗi mã
// một dòng. scripts/check-openapi-codes.sh đối chiếu danh sách này với enum
// `code` trong api/openapi.yaml.
//
// Vì sao phải phân tích cú pháp thay vì grep: bản đầu tiên của phép kiểm này
// dùng `grep -oE`, và nó hỏng theo ba cách đều im lặng —
//
//  1. grep đọc theo từng dòng, nên `errs.New(` bị xuống dòng là mất dấu. Đây
//     không phải cách viết kỳ quặc: gofmt TỰ xuống dòng khi tên biến đủ dài.
//  2. Mã đặt trong hằng (`errs.New(kind, codeStockOut, ...)`) không có literal
//     để bắt.
//  3. Tệ nhất: với `"LEGACY_"+"ENDPOINT_GONE"` grep báo mã tên là `LEGACY_`.
//     Dev tin nó, thêm đúng cái tên sai đó vào spec, và CI xanh trở lại trong
//     khi hợp đồng sai gấp đôi.
//
// Nguyên tắc của lệnh này: chỗ nào không phân tích được thì BÁO LỖI, không bỏ
// qua. Một phép kiểm im lặng bỏ sót còn tệ hơn không có phép kiểm, vì nó tạo ra
// niềm tin sai.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	codes, problems, err := collect(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "checkcodes:", err)
		os.Exit(2)
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "checkcodes: có mã lỗi không phân tích được — hãy dùng chuỗi viết thẳng hoặc hằng string:")
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "    "+p)
		}
		os.Exit(2)
	}
	for _, c := range codes {
		fmt.Println(c)
	}
}

// collect duyệt mọi file .go dưới root và trả về danh sách mã lỗi đã sắp xếp,
// cùng danh sách những chỗ không phân tích được.
func collect(root string) (codes []string, problems []string, err error) {
	fset := token.NewFileSet()

	// Lượt 1: gom mọi hằng string của cả module, khóa theo tên gói + tên hằng.
	// Hằng thường nằm ở file khác với chỗ dùng, nên phải gom xong mới giải được.
	consts := map[string]string{}
	files := map[string][]*ast.File{} // tên gói -> các file

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Bỏ qua thư mục sinh tự động và thư mục nháp.
			switch d.Name() {
			case "vendor", "testdata", "scratch":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return fmt.Errorf("không đọc được %s: %w", path, perr)
		}
		pkg := f.Name.Name
		files[pkg] = append(files[pkg], f)
		collectConsts(pkg, f, consts)
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}

	// Lượt 2: tìm mọi chỗ sinh mã lỗi.
	seen := map[string]bool{}
	for pkg, fs := range files {
		for _, f := range fs {
			// params giữ tên tham số của hàm đang duyệt. Cần nó để phân biệt
			// hai trường hợp trông giống hệt nhau trong AST:
			//
			//   errs.New(KindConflict, "DUPLICATE_SKU", ...)  ← khai mã, phải bắt
			//   func New(kind Kind, code string) *Error {     ← hàm dựng, mã đến
			//       return &Error{Code: code}                   từ chỗ GỌI nó
			//
			// Không loại tham số ra thì chính errs.New và errs.Wrap bị báo là
			// "không phân tích được", và phép kiểm không bao giờ chạy nổi.
			var params map[string]bool

			ast.Inspect(f, func(n ast.Node) bool {
				if fn, ok := n.(*ast.FuncDecl); ok {
					params = paramNames(fn)
					return true
				}
				switch node := n.(type) {
				case *ast.CallExpr:
					idx, ok := codeArgIndex(node.Fun)
					if !ok || idx >= len(node.Args) {
						return true
					}
					v, ok := resolve(node.Args[idx], pkg, consts)
					if !ok {
						if !isParam(node.Args[idx], params) {
							pos := fset.Position(node.Args[idx].Pos())
							problems = append(problems, fmt.Sprintf("%s: đối số mã lỗi không phải hằng string", pos))
						}
						return true
					}
					seen[v] = true

				case *ast.CompositeLit:
					// errs.Error{Code: "..."} hoặc Error{Code: "..."} bên trong
					// chính gói errs — Validation() dựng mã kiểu này, và grep
					// trước đây hoàn toàn không thấy nó.
					if !isErrsErrorType(node.Type) {
						return true
					}
					for _, el := range node.Elts {
						kv, ok := el.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, ok := kv.Key.(*ast.Ident)
						if !ok || key.Name != "Code" {
							continue
						}
						v, ok := resolve(kv.Value, pkg, consts)
						if !ok {
							if !isParam(kv.Value, params) {
								pos := fset.Position(kv.Value.Pos())
								problems = append(problems, fmt.Sprintf("%s: trường Code không phải hằng string", pos))
							}
							continue
						}
						seen[v] = true
					}
				}
				return true
			})
		}
	}

	for c := range seen {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	sort.Strings(problems)
	return codes, problems, nil
}

// paramNames trả về tên mọi tham số của một hàm.
func paramNames(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	if fn.Type == nil || fn.Type.Params == nil {
		return out
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			out[name.Name] = true
		}
	}
	return out
}

// isParam cho biết biểu thức có phải chỉ là tên một tham số của hàm bao ngoài.
func isParam(e ast.Expr, params map[string]bool) bool {
	id, ok := e.(*ast.Ident)
	return ok && params[id.Name]
}

// codeArgIndex cho biết đối số thứ mấy là mã lỗi, với những hàm sinh lỗi đã biết.
func codeArgIndex(fun ast.Expr) (int, bool) {
	sel, ok := fun.(*ast.SelectorExpr)
	if ok {
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "errs" {
			return 0, false
		}
		return argIndexByName(sel.Sel.Name)
	}
	// Gọi không qua tên gói: chỉ xảy ra bên trong chính gói errs.
	id, ok := fun.(*ast.Ident)
	if !ok {
		return 0, false
	}
	return argIndexByName(id.Name)
}

func argIndexByName(name string) (int, bool) {
	switch name {
	case "New": // New(kind, code, message)
		return 1, true
	case "Wrap": // Wrap(cause, kind, code, message)
		return 2, true
	}
	return 0, false
}

func isErrsErrorType(t ast.Expr) bool {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name == "Error"
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		return ok && pkg.Name == "errs" && x.Sel.Name == "Error"
	}
	return false
}

func collectConsts(pkg string, f *ast.File, out map[string]string) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if v, ok := literal(vs.Values[i]); ok {
					out[pkg+"."+name.Name] = v
				}
			}
		}
	}
}

// resolve quy một biểu thức về giá trị string, xử lý được literal, hằng cùng
// gói, và phép cộng chuỗi hằng.
func resolve(e ast.Expr, pkg string, consts map[string]string) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		return literal(x)
	case *ast.Ident:
		v, ok := consts[pkg+"."+x.Name]
		return v, ok
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		v, ok := consts[id.Name+"."+x.Sel.Name]
		return v, ok
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		l, ok := resolve(x.X, pkg, consts)
		if !ok {
			return "", false
		}
		r, ok := resolve(x.Y, pkg, consts)
		if !ok {
			return "", false
		}
		return l + r, true
	case *ast.ParenExpr:
		return resolve(x.X, pkg, consts)
	}
	return "", false
}

func literal(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return v, true
}
