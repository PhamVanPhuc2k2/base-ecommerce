// Lệnh admintool: thao tác quản trị KHÔNG đi qua API.
//
//	go run ./cmd/admintool grant-role <email> <mã_vai_trò>
//
// Dùng để cấp super_admin cho người ĐẦU TIÊN — lúc đó chưa ai có quyền
// iam.roles.manage để gọi PUT /admin/users/{id}/roles. Cố ý không có endpoint
// "tạo admin đầu tiên": endpoint như vậy là lỗ hổng chờ ai đó gọi trước chủ nhà
// (đặc tả P2.2 mục 2.3). Lệnh này cần quyền vào database, tức đã là người vận
// hành hệ thống.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/config"
	"base-ecommerce/api/pkg/postgres"
	platformredis "base-ecommerce/api/pkg/redis"
)

func main() {
	if len(os.Args) != 4 || os.Args[1] != "grant-role" {
		fmt.Fprintln(os.Stderr, "dùng: admintool grant-role <email> <mã_vai_trò>")
		os.Exit(2)
	}
	if err := grantRole(os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, "thất bại:", err)
		os.Exit(1)
	}
}

func grantRole(rawEmail, roleCode string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("kết nối database: %w", err)
	}
	defer pool.Close()
	db := postgres.NewManager(pool)

	email, err := domain.NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	users := pgstore.NewUserRepository(db)
	roles := pgstore.NewRoleRepository(db)

	u, err := users.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	if u == nil {
		return fmt.Errorf("không có tài khoản %s — hãy đăng ký qua POST /auth/register trước", email)
	}
	roleID, err := roles.RoleIDByCode(ctx, roleCode)
	if err != nil {
		return fmt.Errorf("vai trò %q: %w", roleCode, err)
	}

	if err := db.Run(ctx, func(ctx context.Context) error {
		current, err := roles.UserRoleIDs(ctx, u.ID)
		if err != nil {
			return err
		}
		if slices.Contains(current, roleID) {
			return nil
		}
		// CỘNG thêm, không thay: giữ nguyên các vai trò người đó đang có.
		return roles.SetUserRoles(ctx, u.ID, append(current, roleID))
	}); err != nil {
		return err
	}

	// Xóa cache quyền, không thì người đó chờ tới 5 phút mới thấy quyền mới.
	// Redis không tới được thì chỉ cảnh báo: dữ liệu đã đúng trong database.
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	rdb := platformredis.NewClient(ctx, cfg.Redis, quiet)
	defer func() { _ = rdb.Close() }()
	platformredis.NewCache(rdb, quiet).Delete(ctx, usecase.KeyUserPerms(u.ID))

	fmt.Printf("đã cấp vai trò %s cho %s\n", roleCode, email)
	return nil
}
