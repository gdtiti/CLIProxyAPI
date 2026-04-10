// Package models 定义多用户模块的数据模型
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthAccount 第三方登录账号模型
// 关联用户与外部身份提供商
// 验证: 需求 3.1, 3.3, 3.4
type OAuthAccount struct {
	// ID 账号唯一标识，使用 UUID
	ID string `gorm:"primaryKey;type:varchar(36)" json:"id"`

	// UserID 关联的用户 ID
	UserID string `gorm:"type:varchar(36);index;not null" json:"user_id"`

	// Provider OAuth 供应商：google、wechat、github、apple、microsoft
	Provider OAuthProvider `gorm:"type:varchar(32);index;not null" json:"provider"`

	// ProviderUserID 供应商用户 ID
	ProviderUserID string `gorm:"type:varchar(128);index;not null" json:"provider_user_id"`

	// Email 第三方账号邮箱
	Email string `gorm:"type:varchar(255)" json:"email"`

	// DisplayName 第三方账号显示名称
	DisplayName string `gorm:"type:varchar(128)" json:"display_name"`

	// AvatarURL 第三方账号头像 URL
	AvatarURL string `gorm:"type:varchar(512)" json:"avatar_url"`

	// AccessToken OAuth Access Token
	// JSON 序列化时忽略此字段
	AccessToken string `gorm:"type:text" json:"-"`

	// RefreshToken OAuth Refresh Token
	// JSON 序列化时忽略此字段
	RefreshToken string `gorm:"type:text" json:"-"`

	// TokenExpiresAt Token 过期时间
	TokenExpiresAt *time.Time `json:"token_expires_at"`

	// Scopes OAuth 授权范围
	Scopes StringArray `gorm:"type:text[]" json:"scopes"`

	// RawData 原始数据，存储供应商返回的完整信息
	// JSON 序列化时忽略此字段
	RawData JSON `gorm:"type:jsonb" json:"-"`

	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// LastUsedAt 最后使用时间
	LastUsedAt *time.Time `json:"last_used_at"`

	// ========== 关联关系 ==========

	// User 关联的用户
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 指定表名
func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}

// BeforeCreate GORM 钩子，在创建前生成 UUID
func (o *OAuthAccount) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}


// ========== 辅助方法 ==========

// IsTokenExpired 检查 Token 是否已过期
func (o *OAuthAccount) IsTokenExpired() bool {
	if o.TokenExpiresAt == nil {
		return false // 没有过期时间，认为不过期
	}
	return time.Now().After(*o.TokenExpiresAt)
}

// HasValidToken 检查是否有有效的 Token
func (o *OAuthAccount) HasValidToken() bool {
	return o.AccessToken != "" && !o.IsTokenExpired()
}

// HasRefreshToken 检查是否有 Refresh Token
func (o *OAuthAccount) HasRefreshToken() bool {
	return o.RefreshToken != ""
}

// UpdateLastUsed 更新最后使用时间
func (o *OAuthAccount) UpdateLastUsed() {
	now := time.Now()
	o.LastUsedAt = &now
}

// UpdateTokens 更新 Token 信息
func (o *OAuthAccount) UpdateTokens(accessToken, refreshToken string, expiresAt *time.Time) {
	o.AccessToken = accessToken
	if refreshToken != "" {
		o.RefreshToken = refreshToken
	}
	o.TokenExpiresAt = expiresAt
}

// HasScope 检查是否有指定的授权范围
func (o *OAuthAccount) HasScope(scope string) bool {
	return o.Scopes.Contains(scope)
}

// AddScope 添加授权范围
func (o *OAuthAccount) AddScope(scope string) {
	o.Scopes.Add(scope)
}

// SetRawData 设置原始数据中的键值对
func (o *OAuthAccount) SetRawData(key string, value interface{}) {
	if o.RawData == nil {
		o.RawData = make(JSON)
	}
	o.RawData.Set(key, value)
}

// GetRawData 获取原始数据中的值
func (o *OAuthAccount) GetRawData(key string) interface{} {
	if o.RawData == nil {
		return nil
	}
	return o.RawData.Get(key)
}

// Sanitize 清理敏感信息，用于返回给客户端
func (o *OAuthAccount) Sanitize() {
	o.AccessToken = ""
	o.RefreshToken = ""
	o.RawData = nil
}

// Clone 创建 OAuth 账号的浅拷贝
func (o *OAuthAccount) Clone() *OAuthAccount {
	if o == nil {
		return nil
	}
	clone := *o
	return &clone
}

// GetProviderDisplayName 获取供应商的显示名称
func (o *OAuthAccount) GetProviderDisplayName() string {
	switch o.Provider {
	case OAuthProviderGoogle:
		return "Google"
	case OAuthProviderWechat:
		return "微信"
	case OAuthProviderGitHub:
		return "GitHub"
	case OAuthProviderApple:
		return "Apple"
	case OAuthProviderMicrosoft:
		return "Microsoft"
	default:
		return string(o.Provider)
	}
}
