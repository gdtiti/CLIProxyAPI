// Package migration 提供多用户模块的数据库迁移功能
package migration

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Migrator 数据库迁移器
type Migrator struct {
	db     *gorm.DB
	config *config.MultiUserConfig
}

// NewMigrator 创建新的迁移器
func NewMigrator(db *gorm.DB, cfg *config.MultiUserConfig) *Migrator {
	return &Migrator{
		db:     db,
		config: cfg,
	}
}

// AutoMigrate 自动迁移数据库表结构
// 验证: 需求 1.3
func (m *Migrator) AutoMigrate() error {
	logrus.Info("开始执行数据库迁移...")

	// 如果指定了 schema，先创建 schema
	if m.config.Database.Schema != "" {
		if err := m.createSchema(); err != nil {
			return fmt.Errorf("创建 schema 失败: %w", err)
		}
	}

	// 自动迁移所有模型
	if err := m.db.AutoMigrate(
		&models.User{},
		&models.UserGroup{},
		&models.OAuthAccount{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	// 创建额外的索引
	if err := m.createIndexes(); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	logrus.Info("数据库迁移完成")
	return nil
}

// createSchema 创建数据库 schema
func (m *Migrator) createSchema() error {
	schema := m.config.Database.Schema
	if schema == "" {
		return nil
	}

	// 创建 schema（如果不存在）
	sql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
	if err := m.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("创建 schema %s 失败: %w", schema, err)
	}

	// 设置搜索路径
	sql = fmt.Sprintf("SET search_path TO %s, public", schema)
	if err := m.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("设置 search_path 失败: %w", err)
	}

	logrus.Infof("数据库 schema %s 已创建", schema)
	return nil
}

// createIndexes 创建额外的数据库索引
func (m *Migrator) createIndexes() error {
	// OAuthAccount 唯一约束：同一供应商的同一用户只能绑定一次
	// 验证: 需求 3.3
	sql := `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_provider_user 
		ON oauth_accounts(provider, provider_user_id)
	`
	if err := m.db.Exec(sql).Error; err != nil {
		logrus.Warnf("创建 OAuth 唯一索引失败: %v", err)
	}

	return nil
}

// CreateDefaultAdmin 创建默认管理员账号
// 验证: 需求 1.4
func (m *Migrator) CreateDefaultAdmin() error {
	// 检查是否已存在管理员
	var count int64
	if err := m.db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return fmt.Errorf("检查管理员是否存在失败: %w", err)
	}

	if count > 0 {
		logrus.Info("管理员账号已存在，跳过创建")
		return nil
	}

	// 获取管理员配置
	adminCfg := m.config.Admin
	password := adminCfg.Password

	// 如果未配置密码，生成随机密码
	if password == "" {
		password = generateRandomPassword(16)
		logrus.Warnf("未配置管理员密码，已生成随机密码: %s", password)
		logrus.Warn("请妥善保存此密码，后续无法再次查看")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("加密密码失败: %w", err)
	}

	// 创建管理员用户
	admin := &models.User{
		ID:             uuid.New().String(),
		Username:       adminCfg.Username,
		Email:          adminCfg.Email,
		PasswordHash:   string(hashedPassword),
		DisplayName:    "系统管理员",
		Role:           models.RoleAdmin,
		Status:         models.StatusActive,
		RegisterSource: models.RegisterSourceAdmin,
	}

	if err := m.db.Create(admin).Error; err != nil {
		return fmt.Errorf("创建管理员账号失败: %w", err)
	}

	logrus.Infof("默认管理员账号已创建: %s", adminCfg.Username)
	return nil
}

// RunMigrations 执行所有迁移
func (m *Migrator) RunMigrations() error {
	// 1. 自动迁移表结构
	if err := m.AutoMigrate(); err != nil {
		return err
	}

	// 2. 创建默认管理员
	if err := m.CreateDefaultAdmin(); err != nil {
		return err
	}

	return nil
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用固定的默认密码
		return "ChangeMe123!"
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// DropAllTables 删除所有表（仅用于测试）
func (m *Migrator) DropAllTables() error {
	logrus.Warn("正在删除所有多用户模块表...")

	// 按照外键依赖顺序删除
	tables := []interface{}{
		&models.OAuthAccount{},
		&models.User{},
		&models.UserGroup{},
	}

	for _, table := range tables {
		if err := m.db.Migrator().DropTable(table); err != nil {
			logrus.Warnf("删除表失败: %v", err)
		}
	}

	logrus.Info("所有多用户模块表已删除")
	return nil
}

// ResetDatabase 重置数据库（删除并重新创建所有表）
// 仅用于开发和测试环境
func (m *Migrator) ResetDatabase() error {
	logrus.Warn("正在重置多用户模块数据库...")

	// 删除所有表
	if err := m.DropAllTables(); err != nil {
		return err
	}

	// 重新运行迁移
	return m.RunMigrations()
}
