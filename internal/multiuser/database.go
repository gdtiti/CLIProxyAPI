// Package multiuser 提供 CLI Proxy API 的多用户管理功能
package multiuser

import (
	"context"
	"fmt"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/config"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database 数据库连接管理器
type Database struct {
	db     *gorm.DB
	config *config.DatabaseConfig
}

// NewDatabase 创建新的数据库连接管理器
// 验证: 需求 1.1, 1.2, 1.5
func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	if cfg == nil {
		return nil, fmt.Errorf("数据库配置不能为空")
	}

	if cfg.DSN == "" {
		return nil, fmt.Errorf("数据库连接字符串不能为空")
	}

	return &Database{
		config: cfg,
	}, nil
}

// Connect 建立数据库连接
// 验证: 需求 1.1, 1.4
func (d *Database) Connect(ctx context.Context) error {
	// 配置 GORM 日志
	gormLogger := logger.New(
		logrus.StandardLogger(),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// 打开数据库连接
	db, err := gorm.Open(postgres.Open(d.config.DSN), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层 sql.DB 以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接池失败: %w", err)
	}

	// 配置连接池参数
	if d.config.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(d.config.MaxOpenConns)
	}
	if d.config.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(d.config.MaxIdleConns)
	}
	if d.config.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(d.config.ConnMaxLifetime)
	}

	// 测试连接
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 如果指定了 schema，设置搜索路径
	if d.config.Schema != "" {
		if err := db.Exec(fmt.Sprintf("SET search_path TO %s, public", d.config.Schema)).Error; err != nil {
			logrus.Warnf("设置数据库 schema 失败: %v", err)
		}
	}

	d.db = db
	logrus.Info("多用户模块数据库连接成功")
	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	if d.db == nil {
		return nil
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("关闭数据库连接失败: %w", err)
	}

	logrus.Info("多用户模块数据库连接已关闭")
	return nil
}

// DB 获取 GORM 数据库实例
func (d *Database) DB() *gorm.DB {
	return d.db
}

// IsConnected 检查数据库是否已连接
func (d *Database) IsConnected() bool {
	if d.db == nil {
		return false
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		return false
	}

	return sqlDB.Ping() == nil
}

// Ping 测试数据库连接
func (d *Database) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("数据库未连接")
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	return sqlDB.PingContext(ctx)
}

// Stats 获取数据库连接池统计信息
func (d *Database) Stats() (*DBStats, error) {
	if d.db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	stats := sqlDB.Stats()
	return &DBStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}, nil
}

// DBStats 数据库连接池统计信息
type DBStats struct {
	MaxOpenConnections int           `json:"max_open_connections"`
	OpenConnections    int           `json:"open_connections"`
	InUse              int           `json:"in_use"`
	Idle               int           `json:"idle"`
	WaitCount          int64         `json:"wait_count"`
	WaitDuration       time.Duration `json:"wait_duration"`
	MaxIdleClosed      int64         `json:"max_idle_closed"`
	MaxLifetimeClosed  int64         `json:"max_lifetime_closed"`
}

// WithContext 返回带有上下文的数据库实例
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	if d.db == nil {
		return nil
	}
	return d.db.WithContext(ctx)
}

// Transaction 执行数据库事务
func (d *Database) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if d.db == nil {
		return fmt.Errorf("数据库未连接")
	}
	return d.db.WithContext(ctx).Transaction(fn)
}
