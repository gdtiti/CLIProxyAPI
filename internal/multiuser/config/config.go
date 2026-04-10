// Package config 提供多用户模块的配置管理功能
package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// MultiUserConfig 多用户模块主配置结构
// 包含数据库、认证、注册和管理员配置
type MultiUserConfig struct {
	// Enabled 是否启用多用户模块
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Database 数据库连接配置
	Database DatabaseConfig `yaml:"database" json:"database"`

	// Auth 认证相关配置
	Auth AuthConfig `yaml:"auth" json:"auth"`

	// Registration 用户注册配置
	Registration RegistrationConfig `yaml:"registration" json:"registration"`

	// Admin 默认管理员配置
	Admin AdminConfig `yaml:"admin" json:"admin"`
}

// DatabaseConfig 数据库连接配置
// 支持 PostgreSQL 数据库连接和连接池配置
type DatabaseConfig struct {
	// DSN 数据库连接字符串
	// 格式: postgres://user:password@host:port/dbname?sslmode=disable
	DSN string `yaml:"dsn" json:"dsn"`

	// Schema 数据库 schema 名称，用于隔离多用户模块的表
	Schema string `yaml:"schema" json:"schema"`

	// MaxOpenConns 最大打开连接数
	MaxOpenConns int `yaml:"max-open-conns" json:"max_open_conns"`

	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `yaml:"max-idle-conns" json:"max_idle_conns"`

	// ConnMaxLifetime 连接最大生命周期
	ConnMaxLifetime time.Duration `yaml:"conn-max-lifetime" json:"conn_max_lifetime"`
}

// AuthConfig 认证配置
// 包含 JWT Token 相关配置
type AuthConfig struct {
	// JWTSecret JWT 签名密钥
	// 建议使用至少 32 字节的随机字符串
	JWTSecret string `yaml:"jwt-secret" json:"jwt_secret"`

	// AccessTokenTTL Access Token 有效期
	// 默认 24 小时
	AccessTokenTTL time.Duration `yaml:"access-token-ttl" json:"access_token_ttl"`

	// RefreshTokenTTL Refresh Token 有效期
	// 默认 7 天（168 小时）
	RefreshTokenTTL time.Duration `yaml:"refresh-token-ttl" json:"refresh_token_ttl"`
}

// RegistrationConfig 用户注册配置
// 控制用户注册行为和默认值
type RegistrationConfig struct {
	// AllowPublicRegistration 是否允许公开注册
	// 如果为 false，只有管理员可以创建用户
	AllowPublicRegistration bool `yaml:"allow-public-registration" json:"allow_public_registration"`

	// RequireEmailVerification 是否需要邮箱验证
	RequireEmailVerification bool `yaml:"require-email-verification" json:"require_email_verification"`

	// RequireAdminApproval 是否需要管理员审核
	// 如果为 true，新注册用户状态为 pending
	RequireAdminApproval bool `yaml:"require-admin-approval" json:"require_admin_approval"`

	// DefaultGroupID 新用户默认分配的用户组 ID
	// 留空表示不分配用户组
	DefaultGroupID string `yaml:"default-group-id" json:"default_group_id"`

	// DefaultBalance 新用户默认余额
	// 使用字符串格式以支持精确的小数表示
	DefaultBalance string `yaml:"default-balance" json:"default_balance"`

	// DefaultRole 新用户默认角色
	// 可选值: user, operator
	// 注意: 不允许默认设置为 admin
	DefaultRole string `yaml:"default-role" json:"default_role"`
}

// AdminConfig 默认管理员配置
// 用于系统初始化时创建默认管理员账号
type AdminConfig struct {
	// Username 管理员用户名
	Username string `yaml:"username" json:"username"`

	// Password 管理员密码
	// 如果留空，系统将自动生成随机密码
	Password string `yaml:"password" json:"password"`

	// Email 管理员邮箱
	Email string `yaml:"email" json:"email"`
}


// 默认配置常量
const (
	// DefaultMaxOpenConns 默认最大打开连接数
	DefaultMaxOpenConns = 25

	// DefaultMaxIdleConns 默认最大空闲连接数
	DefaultMaxIdleConns = 10

	// DefaultConnMaxLifetime 默认连接最大生命周期
	DefaultConnMaxLifetime = 5 * time.Minute

	// DefaultAccessTokenTTL 默认 Access Token 有效期（24小时）
	DefaultAccessTokenTTL = 24 * time.Hour

	// DefaultRefreshTokenTTL 默认 Refresh Token 有效期（7天）
	DefaultRefreshTokenTTL = 7 * 24 * time.Hour

	// DefaultBalance 默认用户余额
	DefaultBalance = "1.00"

	// DefaultRole 默认用户角色
	DefaultRole = "user"

	// DefaultAdminUsername 默认管理员用户名
	DefaultAdminUsername = "admin"

	// DefaultAdminEmail 默认管理员邮箱
	DefaultAdminEmail = "admin@example.com"

	// DefaultSchema 默认数据库 schema
	DefaultSchema = "multiuser"

	// MinJWTSecretLength JWT 密钥最小长度
	MinJWTSecretLength = 32

	// MinPasswordLength 密码最小长度
	MinPasswordLength = 8
)

// 有效的用户角色
var validRoles = map[string]bool{
	"admin":    true,
	"operator": true,
	"user":     true,
}

// 允许作为默认角色的角色（不包括 admin）
var allowedDefaultRoles = map[string]bool{
	"operator": true,
	"user":     true,
}

// NewDefaultConfig 创建带有默认值的配置
func NewDefaultConfig() *MultiUserConfig {
	return &MultiUserConfig{
		Enabled: false,
		Database: DatabaseConfig{
			DSN:             "",
			Schema:          DefaultSchema,
			MaxOpenConns:    DefaultMaxOpenConns,
			MaxIdleConns:    DefaultMaxIdleConns,
			ConnMaxLifetime: DefaultConnMaxLifetime,
		},
		Auth: AuthConfig{
			JWTSecret:       "",
			AccessTokenTTL:  DefaultAccessTokenTTL,
			RefreshTokenTTL: DefaultRefreshTokenTTL,
		},
		Registration: RegistrationConfig{
			AllowPublicRegistration:  false,
			RequireEmailVerification: false,
			RequireAdminApproval:     false,
			DefaultGroupID:           "",
			DefaultBalance:           DefaultBalance,
			DefaultRole:              DefaultRole,
		},
		Admin: AdminConfig{
			Username: DefaultAdminUsername,
			Password: "",
			Email:    DefaultAdminEmail,
		},
	}
}

// SetDefaults 为配置设置默认值
// 只会设置零值字段，不会覆盖已有配置
func (c *MultiUserConfig) SetDefaults() {
	c.Database.SetDefaults()
	c.Auth.SetDefaults()
	c.Registration.SetDefaults()
	c.Admin.SetDefaults()
}

// SetDefaults 为数据库配置设置默认值
func (c *DatabaseConfig) SetDefaults() {
	if c.Schema == "" {
		c.Schema = DefaultSchema
	}
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
}

// SetDefaults 为认证配置设置默认值
func (c *AuthConfig) SetDefaults() {
	if c.AccessTokenTTL <= 0 {
		c.AccessTokenTTL = DefaultAccessTokenTTL
	}
	if c.RefreshTokenTTL <= 0 {
		c.RefreshTokenTTL = DefaultRefreshTokenTTL
	}
}

// SetDefaults 为注册配置设置默认值
func (c *RegistrationConfig) SetDefaults() {
	if c.DefaultBalance == "" {
		c.DefaultBalance = DefaultBalance
	}
	if c.DefaultRole == "" {
		c.DefaultRole = DefaultRole
	}
}

// SetDefaults 为管理员配置设置默认值
func (c *AdminConfig) SetDefaults() {
	if c.Username == "" {
		c.Username = DefaultAdminUsername
	}
	if c.Email == "" {
		c.Email = DefaultAdminEmail
	}
}

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string // 字段名
	Message string // 错误信息
}

// Error 实现 error 接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证失败 [%s]: %s", e.Field, e.Message)
}

// ValidationErrors 多个验证错误的集合
type ValidationErrors []*ValidationError

// Error 实现 error 接口
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("配置验证失败，共 %d 个错误: %s", len(e), e[0].Message)
}

// HasErrors 检查是否有验证错误
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validate 验证整个配置
// 返回所有验证错误，如果配置有效则返回 nil
func (c *MultiUserConfig) Validate() error {
	// 如果模块未启用，跳过验证
	if !c.Enabled {
		return nil
	}

	var errs ValidationErrors

	// 验证数据库配置
	if dbErrs := c.Database.Validate(); dbErrs != nil {
		if ve, ok := dbErrs.(ValidationErrors); ok {
			errs = append(errs, ve...)
		} else {
			errs = append(errs, &ValidationError{Field: "database", Message: dbErrs.Error()})
		}
	}

	// 验证认证配置
	if authErrs := c.Auth.Validate(); authErrs != nil {
		if ve, ok := authErrs.(ValidationErrors); ok {
			errs = append(errs, ve...)
		} else {
			errs = append(errs, &ValidationError{Field: "auth", Message: authErrs.Error()})
		}
	}

	// 验证注册配置
	if regErrs := c.Registration.Validate(); regErrs != nil {
		if ve, ok := regErrs.(ValidationErrors); ok {
			errs = append(errs, ve...)
		} else {
			errs = append(errs, &ValidationError{Field: "registration", Message: regErrs.Error()})
		}
	}

	// 验证管理员配置
	if adminErrs := c.Admin.Validate(); adminErrs != nil {
		if ve, ok := adminErrs.(ValidationErrors); ok {
			errs = append(errs, ve...)
		} else {
			errs = append(errs, &ValidationError{Field: "admin", Message: adminErrs.Error()})
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate 验证数据库配置
func (c *DatabaseConfig) Validate() error {
	var errs ValidationErrors

	// DSN 必须配置
	if c.DSN == "" {
		errs = append(errs, &ValidationError{
			Field:   "database.dsn",
			Message: "数据库连接字符串不能为空",
		})
	}

	// 连接池配置验证
	if c.MaxOpenConns < 0 {
		errs = append(errs, &ValidationError{
			Field:   "database.max-open-conns",
			Message: "最大打开连接数不能为负数",
		})
	}

	if c.MaxIdleConns < 0 {
		errs = append(errs, &ValidationError{
			Field:   "database.max-idle-conns",
			Message: "最大空闲连接数不能为负数",
		})
	}

	if c.MaxIdleConns > c.MaxOpenConns && c.MaxOpenConns > 0 {
		errs = append(errs, &ValidationError{
			Field:   "database.max-idle-conns",
			Message: "最大空闲连接数不能大于最大打开连接数",
		})
	}

	if c.ConnMaxLifetime < 0 {
		errs = append(errs, &ValidationError{
			Field:   "database.conn-max-lifetime",
			Message: "连接最大生命周期不能为负数",
		})
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate 验证认证配置
func (c *AuthConfig) Validate() error {
	var errs ValidationErrors

	// JWT 密钥必须配置且长度足够
	if c.JWTSecret == "" {
		errs = append(errs, &ValidationError{
			Field:   "auth.jwt-secret",
			Message: "JWT 密钥不能为空",
		})
	} else if len(c.JWTSecret) < MinJWTSecretLength {
		errs = append(errs, &ValidationError{
			Field:   "auth.jwt-secret",
			Message: fmt.Sprintf("JWT 密钥长度不能少于 %d 字符", MinJWTSecretLength),
		})
	}

	// Token 有效期验证
	if c.AccessTokenTTL <= 0 {
		errs = append(errs, &ValidationError{
			Field:   "auth.access-token-ttl",
			Message: "Access Token 有效期必须大于 0",
		})
	}

	if c.RefreshTokenTTL <= 0 {
		errs = append(errs, &ValidationError{
			Field:   "auth.refresh-token-ttl",
			Message: "Refresh Token 有效期必须大于 0",
		})
	}

	// Refresh Token 有效期应该大于 Access Token 有效期
	if c.RefreshTokenTTL > 0 && c.AccessTokenTTL > 0 && c.RefreshTokenTTL < c.AccessTokenTTL {
		errs = append(errs, &ValidationError{
			Field:   "auth.refresh-token-ttl",
			Message: "Refresh Token 有效期应该大于或等于 Access Token 有效期",
		})
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate 验证注册配置
func (c *RegistrationConfig) Validate() error {
	var errs ValidationErrors

	// 验证默认余额格式
	if c.DefaultBalance != "" {
		if _, err := decimal.NewFromString(c.DefaultBalance); err != nil {
			errs = append(errs, &ValidationError{
				Field:   "registration.default-balance",
				Message: "默认余额格式无效，必须是有效的数字",
			})
		}
	}

	// 验证默认角色
	if c.DefaultRole != "" {
		if !validRoles[c.DefaultRole] {
			errs = append(errs, &ValidationError{
				Field:   "registration.default-role",
				Message: fmt.Sprintf("无效的默认角色: %s，可选值: user, operator", c.DefaultRole),
			})
		} else if !allowedDefaultRoles[c.DefaultRole] {
			errs = append(errs, &ValidationError{
				Field:   "registration.default-role",
				Message: "默认角色不能设置为 admin",
			})
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// Validate 验证管理员配置
func (c *AdminConfig) Validate() error {
	var errs ValidationErrors

	// 用户名验证
	if c.Username == "" {
		errs = append(errs, &ValidationError{
			Field:   "admin.username",
			Message: "管理员用户名不能为空",
		})
	} else if len(c.Username) < 3 || len(c.Username) > 64 {
		errs = append(errs, &ValidationError{
			Field:   "admin.username",
			Message: "管理员用户名长度必须在 3-64 字符之间",
		})
	}

	// 密码验证（如果提供了密码）
	if c.Password != "" && len(c.Password) < MinPasswordLength {
		errs = append(errs, &ValidationError{
			Field:   "admin.password",
			Message: fmt.Sprintf("管理员密码长度不能少于 %d 字符", MinPasswordLength),
		})
	}

	// 邮箱验证
	if c.Email == "" {
		errs = append(errs, &ValidationError{
			Field:   "admin.email",
			Message: "管理员邮箱不能为空",
		})
	} else if !isValidEmail(c.Email) {
		errs = append(errs, &ValidationError{
			Field:   "admin.email",
			Message: "管理员邮箱格式无效",
		})
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// isValidEmail 简单的邮箱格式验证
func isValidEmail(email string) bool {
	// 简单验证：包含 @ 且 @ 前后都有字符
	if len(email) < 3 {
		return false
	}
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			if atIndex != -1 {
				return false // 多个 @
			}
			atIndex = i
		}
	}
	if atIndex <= 0 || atIndex >= len(email)-1 {
		return false
	}
	// 检查 @ 后面是否有点
	afterAt := email[atIndex+1:]
	hasDot := false
	for _, c := range afterAt {
		if c == '.' {
			hasDot = true
			break
		}
	}
	return hasDot
}

// GetDefaultBalanceDecimal 获取默认余额的 Decimal 值
func (c *RegistrationConfig) GetDefaultBalanceDecimal() (decimal.Decimal, error) {
	if c.DefaultBalance == "" {
		return decimal.NewFromString(DefaultBalance)
	}
	return decimal.NewFromString(c.DefaultBalance)
}

// IsEnabled 检查多用户模块是否启用
func (c *MultiUserConfig) IsEnabled() bool {
	return c.Enabled
}

// Clone 创建配置的深拷贝
func (c *MultiUserConfig) Clone() *MultiUserConfig {
	if c == nil {
		return nil
	}
	return &MultiUserConfig{
		Enabled:      c.Enabled,
		Database:     c.Database,
		Auth:         c.Auth,
		Registration: c.Registration,
		Admin:        c.Admin,
	}
}

// MergeWith 将另一个配置合并到当前配置
// 只合并非零值字段
func (c *MultiUserConfig) MergeWith(other *MultiUserConfig) {
	if other == nil {
		return
	}

	// 合并数据库配置
	if other.Database.DSN != "" {
		c.Database.DSN = other.Database.DSN
	}
	if other.Database.Schema != "" {
		c.Database.Schema = other.Database.Schema
	}
	if other.Database.MaxOpenConns > 0 {
		c.Database.MaxOpenConns = other.Database.MaxOpenConns
	}
	if other.Database.MaxIdleConns > 0 {
		c.Database.MaxIdleConns = other.Database.MaxIdleConns
	}
	if other.Database.ConnMaxLifetime > 0 {
		c.Database.ConnMaxLifetime = other.Database.ConnMaxLifetime
	}

	// 合并认证配置
	if other.Auth.JWTSecret != "" {
		c.Auth.JWTSecret = other.Auth.JWTSecret
	}
	if other.Auth.AccessTokenTTL > 0 {
		c.Auth.AccessTokenTTL = other.Auth.AccessTokenTTL
	}
	if other.Auth.RefreshTokenTTL > 0 {
		c.Auth.RefreshTokenTTL = other.Auth.RefreshTokenTTL
	}

	// 合并注册配置
	if other.Registration.DefaultGroupID != "" {
		c.Registration.DefaultGroupID = other.Registration.DefaultGroupID
	}
	if other.Registration.DefaultBalance != "" {
		c.Registration.DefaultBalance = other.Registration.DefaultBalance
	}
	if other.Registration.DefaultRole != "" {
		c.Registration.DefaultRole = other.Registration.DefaultRole
	}

	// 合并管理员配置
	if other.Admin.Username != "" {
		c.Admin.Username = other.Admin.Username
	}
	if other.Admin.Password != "" {
		c.Admin.Password = other.Admin.Password
	}
	if other.Admin.Email != "" {
		c.Admin.Email = other.Admin.Email
	}
}

// ErrConfigDisabled 模块未启用错误
var ErrConfigDisabled = errors.New("多用户模块未启用")

// ErrInvalidConfig 配置无效错误
var ErrInvalidConfig = errors.New("配置无效")
