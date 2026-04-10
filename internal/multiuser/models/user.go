// Package models 定义多用户模块的数据模型
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// User 用户模型
// 包含用户身份认证和授权信息
// 验证: 需求 2.1, 2.5, 2.6
type User struct {
	// ID 用户唯一标识，使用 UUID
	ID string `gorm:"primaryKey;type:varchar(36)" json:"id"`

	// Username 用户名，唯一且不能为空
	Username string `gorm:"uniqueIndex;type:varchar(64);not null" json:"username"`

	// Email 用户邮箱，唯一
	Email string `gorm:"uniqueIndex;type:varchar(255)" json:"email"`

	// PasswordHash 密码哈希值，使用 bcrypt 加密
	// JSON 序列化时忽略此字段
	PasswordHash string `gorm:"type:varchar(255)" json:"-"`

	// DisplayName 显示名称
	DisplayName string `gorm:"type:varchar(128)" json:"display_name"`

	// AvatarURL 头像 URL
	AvatarURL string `gorm:"type:varchar(512)" json:"avatar_url"`

	// Role 用户角色：admin、operator、user
	Role UserRole `gorm:"type:varchar(32);default:'user'" json:"role"`

	// Status 用户状态：active、suspended、disabled、pending
	Status UserStatus `gorm:"type:varchar(32);default:'active'" json:"status"`

	// Balance 用户余额
	Balance decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"balance"`

	// CreditLimit 信用额度
	CreditLimit decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"credit_limit"`

	// GroupID 所属用户组 ID
	GroupID *string `gorm:"type:varchar(36);index" json:"group_id"`

	// RegisterSource 注册来源：local、google、wechat、github、invite、admin
	RegisterSource RegisterSource `gorm:"type:varchar(32);default:'local'" json:"register_source"`

	// RegisterIP 注册时的 IP 地址
	RegisterIP string `gorm:"type:varchar(64)" json:"register_ip"`

	// Metadata 用户元数据，存储额外信息
	Metadata JSON `gorm:"type:jsonb" json:"metadata"`

	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// LastLoginAt 最后登录时间
	LastLoginAt *time.Time `json:"last_login_at"`

	// ========== 关联关系 ==========

	// Group 所属用户组
	Group *UserGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`

	// OAuthAccounts 关联的第三方登录账号
	OAuthAccounts []OAuthAccount `gorm:"foreignKey:UserID" json:"oauth_accounts,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// BeforeCreate GORM 钩子，在创建前生成 UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}


// ========== 辅助方法 ==========

// HasPassword 检查用户是否设置了密码
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// CanLogin 检查用户是否可以登录
// 只有状态为 active 的用户才能登录
func (u *User) CanLogin() bool {
	return u.Status.CanLogin()
}

// IsAdmin 检查用户是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsOperator 检查用户是否为操作员
func (u *User) IsOperator() bool {
	return u.Role == RoleOperator
}

// HasGroup 检查用户是否属于某个用户组
func (u *User) HasGroup() bool {
	return u.GroupID != nil && *u.GroupID != ""
}

// GetAvailableBalance 获取可用余额（余额 + 信用额度）
func (u *User) GetAvailableBalance() decimal.Decimal {
	return u.Balance.Add(u.CreditLimit)
}

// IsOAuthUser 检查用户是否通过 OAuth 注册
func (u *User) IsOAuthUser() bool {
	return u.RegisterSource.IsOAuth()
}

// SetMetadata 设置元数据中的键值对
func (u *User) SetMetadata(key string, value interface{}) {
	if u.Metadata == nil {
		u.Metadata = make(JSON)
	}
	u.Metadata.Set(key, value)
}

// GetMetadata 获取元数据中的值
func (u *User) GetMetadata(key string) interface{} {
	if u.Metadata == nil {
		return nil
	}
	return u.Metadata.Get(key)
}

// UpdateLastLogin 更新最后登录时间
func (u *User) UpdateLastLogin() {
	now := time.Now()
	u.LastLoginAt = &now
}

// Sanitize 清理敏感信息，用于返回给客户端
func (u *User) Sanitize() {
	u.PasswordHash = ""
}

// Clone 创建用户的浅拷贝
func (u *User) Clone() *User {
	if u == nil {
		return nil
	}
	clone := *u
	return &clone
}
