package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker cài đặt health.Checker cho Postgres.
type HealthChecker struct{ pool *pgxpool.Pool }

func NewHealthChecker(pool *pgxpool.Pool) *HealthChecker { return &HealthChecker{pool: pool} }

func (c *HealthChecker) Name() string { return "postgres" }

func (c *HealthChecker) Check(ctx context.Context) error { return c.pool.Ping(ctx) }
