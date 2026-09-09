package config_test

import (
	"testing"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestLoad_DayDuBienThiThanhCong(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/db?sslmode=disable")
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("DB_MAX_CONNS", "30")
	t.Setenv("HTTP_READ_TIMEOUT", "20s")

	c, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, "production", c.Env)
	require.Equal(t, ":9000", c.HTTP.Addr)
	require.EqualValues(t, 30, c.DB.MaxConns)
	require.Equal(t, 20*time.Second, c.HTTP.ReadTimeout)
}

func TestLoad_DungGiaTriMacDinhKhiThieuBienKhongBatBuoc(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")

	c, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, "development", c.Env)
	require.Equal(t, ":8080", c.HTTP.Addr)
	require.EqualValues(t, 20, c.DB.MaxConns)
	require.Equal(t, time.Hour, c.DB.MaxConnLifetime)
}

func TestLoad_ThieuBienBatBuocThiLoi(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_GomTatCaLoiTrongMotLan(t *testing.T) {
	// Sai nhiều biến thì phải báo hết trong một lần, không phải sửa từng cái một.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_MAX_CONNS", "khong-phai-so")
	t.Setenv("HTTP_READ_TIMEOUT", "khong-phai-thoi-gian")

	_, err := config.Load()

	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "DATABASE_URL")
	require.Contains(t, msg, "DB_MAX_CONNS")
	require.Contains(t, msg, "HTTP_READ_TIMEOUT")
}

func TestConfig_StringCheGiaTriNhayCam(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:matkhausieubimat@localhost:5432/db")

	c, err := config.Load()
	require.NoError(t, err)
	require.NotContains(t, c.String(), "matkhausieubimat")
}

func TestConfig_IsProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")

	c, err := config.Load()
	require.NoError(t, err)
	require.True(t, c.IsProduction())
}
