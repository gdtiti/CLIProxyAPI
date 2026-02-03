package multiuser

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDatabase(t *testing.T) {
	t.Run("有效配置", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			DSN:             "postgres://user:pass@localhost:5432/db",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
		}

		db, err := NewDatabase(cfg)
		require.NoError(t, err)
		assert.NotNil(t, db)
		assert.Equal(t, cfg, db.config)
	})

	t.Run("nil 配置", func(t *testing.T) {
		db, err := NewDatabase(nil)
		assert.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "配置不能为空")
	})

	t.Run("空 DSN", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			DSN: "",
		}

		db, err := NewDatabase(cfg)
		assert.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "连接字符串不能为空")
	})
}

func TestDatabase_IsConnected(t *testing.T) {
	t.Run("未连接时返回 false", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			DSN: "postgres://user:pass@localhost:5432/db",
		}

		db, err := NewDatabase(cfg)
		require.NoError(t, err)

		// 未调用 Connect，应该返回 false
		assert.False(t, db.IsConnected())
	})
}

func TestDatabase_DB(t *testing.T) {
	t.Run("未连接时返回 nil", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			DSN: "postgres://user:pass@localhost:5432/db",
		}

		db, err := NewDatabase(cfg)
		require.NoError(t, err)

		// 未调用 Connect，DB() 应该返回 nil
		assert.Nil(t, db.DB())
	})
}

func TestDatabase_Close(t *testing.T) {
	t.Run("未连接时关闭不报错", func(t *testing.T) {
		cfg := &config.DatabaseConfig{
			DSN: "postgres://user:pass@localhost:5432/db",
		}

		db, err := NewDatabase(cfg)
		require.NoError(t, err)

		// 未连接时关闭应该不报错
		err = db.Close()
		assert.NoError(t, err)
	})
}

// 注意：以下测试需要实际的数据库连接，在 CI 环境中可能需要跳过
// 可以通过环境变量 MULTIUSER_TEST_DSN 来配置测试数据库

/*
func TestDatabase_Connect_Integration(t *testing.T) {
	dsn := os.Getenv("MULTIUSER_TEST_DSN")
	if dsn == "" {
		t.Skip("跳过集成测试：未设置 MULTIUSER_TEST_DSN 环境变量")
	}

	cfg := &config.DatabaseConfig{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 1 * time.Minute,
	}

	db, err := NewDatabase(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = db.Connect(ctx)
	require.NoError(t, err)
	defer db.Close()

	// 验证连接成功
	assert.True(t, db.IsConnected())
	assert.NotNil(t, db.DB())

	// 测试 Ping
	err = db.Ping(ctx)
	assert.NoError(t, err)

	// 测试 Stats
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 5, stats.MaxOpenConnections)
}
*/
