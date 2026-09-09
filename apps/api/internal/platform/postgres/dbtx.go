package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX là tập lệnh chung của *pgxpool.Pool và pgx.Tx.
//
// Repository nhận DBTX chứ không nhận *pgxpool.Pool, nhờ vậy cùng một repository
// chạy được cả trong lẫn ngoài transaction mà không phải viết hai bản.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
