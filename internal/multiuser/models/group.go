// Package models 定义多用户模块的数据模型
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// UserGroup 用户组模型
// 支持层级结构和共享配置
// 验证: 需求 4.1, 4.2, 4.3, 4.4, 4.5
type UserGroup struct {
	// ID 用户组唯一标识，使用 UUID
	ID string `gorm:"primaryKey;type:varchar(36)" json:"id"`

	// Name 用户组名称，唯一且不能为空
	Name string `gorm:"uniqueIndex;type:varchar(64);not null" json:"name"`

	// Description 用户组描述
	Description string `gorm:"type:text" json:"description"`

	// ParentID 父用户组 ID，用于实现层级结构
	ParentID *string `gorm:"type:varchar(36);index" json:"parent_id"`

	// BalancePool 组级余额池
	BalancePool decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"balance_pool"`

	// SharedBalance 是否启用共享余额模式
	SharedBalance bool `gorm:"default:false" json:"shared_balance"`

	// RateMultiplier 组级费率倍数
	RateMultiplier decimal.Decimal `gorm:"type:decimal(10,4);default:1.0" json:"rate_multiplier"`

	// Priority 优先级，数值越大优先级越高
	Priority int `gorm:"default:0" json:"priority"`

	// Metadata 用户组元数据，存储额外信息
	Metadata JSON `gorm:"type:jsonb" json:"metadata"`

	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// ========== 关联关系 ==========

	// Parent 父用户组
	Parent *UserGroup `gorm:"foreignKey:ParentID" json:"parent,omitempty"`

	// Children 子用户组列表
	Children []UserGroup `gorm:"foreignKey:ParentID" json:"children,omitempty"`

	// Users 组内用户列表
	Users []User `gorm:"foreignKey:GroupID" json:"users,omitempty"`
}

// TableName 指定表名
func (UserGroup) TableName() string {
	return "user_groups"
}

// BeforeCreate GORM 钩子，在创建前生成 UUID
func (g *UserGroup) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}


// ========== 辅助方法 ==========

// HasParent 检查用户组是否有父组
func (g *UserGroup) HasParent() bool {
	return g.ParentID != nil && *g.ParentID != ""
}

// IsRoot 检查用户组是否为根组（没有父组）
func (g *UserGroup) IsRoot() bool {
	return !g.HasParent()
}

// IsSharedBalance 检查是否启用共享余额模式
func (g *UserGroup) IsSharedBalance() bool {
	return g.SharedBalance
}

// GetEffectiveRateMultiplier 获取有效的费率倍数
// 如果未设置，返回默认值 1.0
func (g *UserGroup) GetEffectiveRateMultiplier() decimal.Decimal {
	if g.RateMultiplier.IsZero() {
		return decimal.NewFromInt(1)
	}
	return g.RateMultiplier
}

// SetMetadata 设置元数据中的键值对
func (g *UserGroup) SetMetadata(key string, value interface{}) {
	if g.Metadata == nil {
		g.Metadata = make(JSON)
	}
	g.Metadata.Set(key, value)
}

// GetMetadata 获取元数据中的值
func (g *UserGroup) GetMetadata(key string) interface{} {
	if g.Metadata == nil {
		return nil
	}
	return g.Metadata.Get(key)
}

// GetUserCount 获取组内用户数量
func (g *UserGroup) GetUserCount() int {
	return len(g.Users)
}

// GetChildCount 获取子组数量
func (g *UserGroup) GetChildCount() int {
	return len(g.Children)
}

// Clone 创建用户组的浅拷贝
func (g *UserGroup) Clone() *UserGroup {
	if g == nil {
		return nil
	}
	clone := *g
	return &clone
}

// AddBalance 增加组余额池
func (g *UserGroup) AddBalance(amount decimal.Decimal) {
	g.BalancePool = g.BalancePool.Add(amount)
}

// SubtractBalance 减少组余额池
// 返回是否成功（余额是否足够）
func (g *UserGroup) SubtractBalance(amount decimal.Decimal) bool {
	if g.BalancePool.LessThan(amount) {
		return false
	}
	g.BalancePool = g.BalancePool.Sub(amount)
	return true
}

// HasSufficientBalance 检查组余额池是否足够
func (g *UserGroup) HasSufficientBalance(amount decimal.Decimal) bool {
	return g.BalancePool.GreaterThanOrEqual(amount)
}
