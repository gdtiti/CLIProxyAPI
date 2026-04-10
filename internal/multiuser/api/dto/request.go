// Package dto 定义多用户模块的数据传输对象
package dto

import "github.com/shopspring/decimal"

// ============================================================================
// 用户相关请求 DTO
// ============================================================================

// CreateUserRequest 创建用户请求
// @Description 管理员创建用户的请求参数
type CreateUserRequest struct {
	// 用户名，3-64 字符，仅允许字母数字和下划线
	Username string `json:"username" binding:"required,min=3,max=64"`
	// 邮箱地址
	Email string `json:"email" binding:"required,email"`
	// 密码，最少 8 位字符
	Password string `json:"password" binding:"required,min=8"`
	// 显示名称
	DisplayName string `json:"display_name" binding:"max=128"`
	// 用户角色：admin、operator、user
	Role string `json:"role" binding:"omitempty,oneof=admin operator user"`
	// 用户状态：active、suspended、disabled、pending
	Status string `json:"status" binding:"omitempty,oneof=active suspended disabled pending"`
	// 所属用户组 ID
	GroupID string `json:"group_id" binding:"omitempty"`
	// 初始余额
	Balance string `json:"balance" binding:"omitempty"`
}

// UpdateUserRequest 更新用户请求
// @Description 更新用户信息的请求参数
type UpdateUserRequest struct {
	// 邮箱地址
	Email *string `json:"email" binding:"omitempty,email"`
	// 显示名称
	DisplayName *string `json:"display_name" binding:"omitempty,max=128"`
	// 头像 URL
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url"`
	// 所属用户组 ID，空字符串表示移除用户组
	GroupID *string `json:"group_id" binding:"omitempty"`
}

// LoginRequest 登录请求
// @Description 用户登录的请求参数
type LoginRequest struct {
	// 用户名或邮箱
	Username string `json:"username" binding:"required"`
	// 密码
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 注册请求
// @Description 用户注册的请求参数
type RegisterRequest struct {
	// 用户名，3-64 字符，仅允许字母数字和下划线
	Username string `json:"username" binding:"required,min=3,max=64"`
	// 邮箱地址
	Email string `json:"email" binding:"required,email"`
	// 密码，最少 8 位字符
	Password string `json:"password" binding:"required,min=8"`
	// 显示名称
	DisplayName string `json:"display_name" binding:"max=128"`
}

// ChangePasswordRequest 修改密码请求
// @Description 用户修改密码的请求参数
type ChangePasswordRequest struct {
	// 当前密码
	OldPassword string `json:"old_password" binding:"required"`
	// 新密码，最少 8 位字符
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// SetUserStatusRequest 设置用户状态请求
// @Description 管理员设置用户状态的请求参数
type SetUserStatusRequest struct {
	// 用户状态：active、suspended、disabled、pending
	Status string `json:"status" binding:"required,oneof=active suspended disabled pending"`
}

// SetUserRoleRequest 设置用户角色请求
// @Description 管理员设置用户角色的请求参数
type SetUserRoleRequest struct {
	// 用户角色：admin、operator、user
	Role string `json:"role" binding:"required,oneof=admin operator user"`
}

// SetUserBalanceRequest 设置用户余额请求
// @Description 管理员设置用户余额的请求参数
type SetUserBalanceRequest struct {
	// 余额值
	Balance decimal.Decimal `json:"balance" binding:"required" swaggertype:"string" example:"100.00"`
}

// RefreshTokenRequest 刷新 Token 请求
// @Description 刷新访问令牌的请求参数
type RefreshTokenRequest struct {
	// 刷新令牌
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ============================================================================
// 用户组相关请求 DTO
// ============================================================================

// CreateGroupRequest 创建用户组请求
// @Description 管理员创建用户组的请求参数
type CreateGroupRequest struct {
	// 用户组名称，1-64 字符
	Name string `json:"name" binding:"required,min=1,max=64"`
	// 用户组描述
	Description string `json:"description" binding:"omitempty"`
	// 父用户组 ID
	ParentID string `json:"parent_id" binding:"omitempty"`
	// 余额池
	BalancePool string `json:"balance_pool" binding:"omitempty"`
	// 是否共享余额
	SharedBalance bool `json:"shared_balance"`
	// 费率倍数
	RateMultiplier string `json:"rate_multiplier" binding:"omitempty"`
	// 优先级
	Priority int `json:"priority" binding:"omitempty"`
}

// UpdateGroupRequest 更新用户组请求
// @Description 更新用户组信息的请求参数
type UpdateGroupRequest struct {
	// 用户组名称，1-64 字符
	Name *string `json:"name" binding:"omitempty,min=1,max=64"`
	// 用户组描述
	Description *string `json:"description" binding:"omitempty"`
	// 余额池
	BalancePool *string `json:"balance_pool" binding:"omitempty"`
	// 是否共享余额
	SharedBalance *bool `json:"shared_balance" binding:"omitempty"`
	// 费率倍数
	RateMultiplier *string `json:"rate_multiplier" binding:"omitempty"`
	// 优先级
	Priority *int `json:"priority" binding:"omitempty"`
}

// AddGroupMemberRequest 添加用户组成员请求
// @Description 添加用户到用户组的请求参数
type AddGroupMemberRequest struct {
	// 用户 ID
	UserID string `json:"user_id" binding:"required"`
}

// ============================================================================
// 列表查询请求 DTO
// ============================================================================

// ListUsersRequest 用户列表请求
// @Description 分页查询用户列表的请求参数
type ListUsersRequest struct {
	// 页码，从 1 开始
	Page int `form:"page" binding:"omitempty,min=1"`
	// 每页数量，1-100
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
	// 排序字段
	SortBy string `form:"sort_by" binding:"omitempty"`
	// 是否降序排序
	SortDesc bool `form:"sort_desc" binding:"omitempty"`
	// 按状态筛选
	Status string `form:"status" binding:"omitempty,oneof=active suspended disabled pending"`
	// 按角色筛选
	Role string `form:"role" binding:"omitempty,oneof=admin operator user"`
	// 按用户组 ID 筛选
	GroupID string `form:"group_id" binding:"omitempty"`
	// 搜索关键词（用户名模糊匹配）
	Search string `form:"search" binding:"omitempty"`
}

// ListGroupsRequest 用户组列表请求
// @Description 分页查询用户组列表的请求参数
type ListGroupsRequest struct {
	// 页码，从 1 开始
	Page int `form:"page" binding:"omitempty,min=1"`
	// 每页数量，1-100
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
	// 排序字段
	SortBy string `form:"sort_by" binding:"omitempty"`
	// 是否降序排序
	SortDesc bool `form:"sort_desc" binding:"omitempty"`
	// 按父用户组 ID 筛选，空字符串表示只查根组
	ParentID *string `form:"parent_id" binding:"omitempty"`
	// 按共享余额筛选
	SharedBalance *bool `form:"shared_balance" binding:"omitempty"`
	// 搜索关键词（用户组名模糊匹配）
	Search string `form:"search" binding:"omitempty"`
}

// ============================================================================
// 可用性检查请求 DTO
// ============================================================================

// CheckUsernameRequest 检查用户名可用性请求
// @Description 检查用户名是否可用的请求参数
type CheckUsernameRequest struct {
	// 用户名
	Username string `form:"username" binding:"required,min=3,max=64"`
}

// CheckEmailRequest 检查邮箱可用性请求
// @Description 检查邮箱是否可用的请求参数
type CheckEmailRequest struct {
	// 邮箱地址
	Email string `form:"email" binding:"required,email"`
}

// ============================================================================
// 请求默认值设置
// ============================================================================

// SetDefaults 设置 ListUsersRequest 的默认值
func (r *ListUsersRequest) SetDefaults() {
	if r.Page <= 0 {
		r.Page = 1
	}
	if r.PageSize <= 0 {
		r.PageSize = 20
	}
	if r.PageSize > 100 {
		r.PageSize = 100
	}
}

// SetDefaults 设置 ListGroupsRequest 的默认值
func (r *ListGroupsRequest) SetDefaults() {
	if r.Page <= 0 {
		r.Page = 1
	}
	if r.PageSize <= 0 {
		r.PageSize = 20
	}
	if r.PageSize > 100 {
		r.PageSize = 100
	}
}
