// Package dto 定义多用户模块的数据传输对象
package dto

import (
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
)

// ============================================================================
// 认证相关响应 DTO
// ============================================================================

// LoginResponse 登录响应
// @Description 用户登录成功后的响应
type LoginResponse struct {
	// 访问令牌
	AccessToken string `json:"access_token"`
	// 刷新令牌
	RefreshToken string `json:"refresh_token"`
	// 访问令牌过期时间（秒）
	ExpiresIn int64 `json:"expires_in"`
	// 令牌类型，固定为 Bearer
	TokenType string `json:"token_type"`
	// 用户信息
	User *UserInfo `json:"user"`
}

// TokenPair Token 对
// @Description 访问令牌和刷新令牌对
type TokenPair struct {
	// 访问令牌
	AccessToken string `json:"access_token"`
	// 刷新令牌
	RefreshToken string `json:"refresh_token"`
	// 访问令牌过期时间（秒）
	ExpiresIn int64 `json:"expires_in"`
}

// ============================================================================
// 用户信息响应 DTO
// ============================================================================

// UserInfo 用户信息（不包含敏感字段）
// @Description 用户基本信息，不包含密码等敏感字段
type UserInfo struct {
	// 用户 ID
	ID string `json:"id"`
	// 用户名
	Username string `json:"username"`
	// 邮箱地址
	Email string `json:"email"`
	// 显示名称
	DisplayName string `json:"display_name"`
	// 头像 URL
	AvatarURL string `json:"avatar_url"`
	// 用户角色
	Role string `json:"role"`
	// 用户状态
	Status string `json:"status"`
	// 余额
	Balance string `json:"balance"`
	// 信用额度
	CreditLimit string `json:"credit_limit"`
	// 所属用户组 ID
	GroupID string `json:"group_id,omitempty"`
	// 注册来源
	RegisterSource string `json:"register_source"`
	// 创建时间
	CreatedAt string `json:"created_at"`
	// 更新时间
	UpdatedAt string `json:"updated_at"`
	// 最后登录时间
	LastLoginAt string `json:"last_login_at,omitempty"`
}

// UserInfoFromModel 从 User 模型转换为 UserInfo DTO
func UserInfoFromModel(user *models.User) *UserInfo {
	if user == nil {
		return nil
	}

	info := &UserInfo{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		AvatarURL:      user.AvatarURL,
		Role:           string(user.Role),
		Status:         string(user.Status),
		Balance:        user.Balance.String(),
		CreditLimit:    user.CreditLimit.String(),
		RegisterSource: string(user.RegisterSource),
		CreatedAt:      user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      user.UpdatedAt.Format(time.RFC3339),
	}

	if user.GroupID != nil {
		info.GroupID = *user.GroupID
	}

	if user.LastLoginAt != nil {
		info.LastLoginAt = user.LastLoginAt.Format(time.RFC3339)
	}

	return info
}

// UserInfoListFromModels 从 User 模型列表转换为 UserInfo DTO 列表
func UserInfoListFromModels(users []*models.User) []*UserInfo {
	result := make([]*UserInfo, 0, len(users))
	for _, user := range users {
		result = append(result, UserInfoFromModel(user))
	}
	return result
}

// ============================================================================
// 用户组信息响应 DTO
// ============================================================================

// GroupInfo 用户组信息
// @Description 用户组基本信息
type GroupInfo struct {
	// 用户组 ID
	ID string `json:"id"`
	// 用户组名称
	Name string `json:"name"`
	// 用户组描述
	Description string `json:"description"`
	// 父用户组 ID
	ParentID string `json:"parent_id,omitempty"`
	// 余额池
	BalancePool string `json:"balance_pool"`
	// 是否共享余额
	SharedBalance bool `json:"shared_balance"`
	// 费率倍数
	RateMultiplier string `json:"rate_multiplier"`
	// 优先级
	Priority int `json:"priority"`
	// 创建时间
	CreatedAt string `json:"created_at"`
	// 更新时间
	UpdatedAt string `json:"updated_at"`
	// 成员数量（可选）
	MemberCount *int64 `json:"member_count,omitempty"`
}

// GroupInfoFromModel 从 UserGroup 模型转换为 GroupInfo DTO
func GroupInfoFromModel(group *models.UserGroup) *GroupInfo {
	if group == nil {
		return nil
	}

	info := &GroupInfo{
		ID:             group.ID,
		Name:           group.Name,
		Description:    group.Description,
		BalancePool:    group.BalancePool.String(),
		SharedBalance:  group.SharedBalance,
		RateMultiplier: group.RateMultiplier.String(),
		Priority:       group.Priority,
		CreatedAt:      group.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      group.UpdatedAt.Format(time.RFC3339),
	}

	if group.ParentID != nil {
		info.ParentID = *group.ParentID
	}

	return info
}

// GroupInfoListFromModels 从 UserGroup 模型列表转换为 GroupInfo DTO 列表
func GroupInfoListFromModels(groups []*models.UserGroup) []*GroupInfo {
	result := make([]*GroupInfo, 0, len(groups))
	for _, group := range groups {
		result = append(result, GroupInfoFromModel(group))
	}
	return result
}

// ============================================================================
// 列表响应 DTO
// ============================================================================

// ListUsersResponse 用户列表响应
// @Description 分页查询用户列表的响应
type ListUsersResponse struct {
	// 总记录数
	Total int64 `json:"total"`
	// 当前页码
	Page int `json:"page"`
	// 每页数量
	PageSize int `json:"page_size"`
	// 用户列表
	Items []*UserInfo `json:"items"`
}

// ListGroupsResponse 用户组列表响应
// @Description 分页查询用户组列表的响应
type ListGroupsResponse struct {
	// 总记录数
	Total int64 `json:"total"`
	// 当前页码
	Page int `json:"page"`
	// 每页数量
	PageSize int `json:"page_size"`
	// 用户组列表
	Items []*GroupInfo `json:"items"`
}

// GroupMembersResponse 用户组成员列表响应
// @Description 查询用户组成员的响应
type GroupMembersResponse struct {
	// 用户组 ID
	GroupID string `json:"group_id"`
	// 成员列表
	Members []*UserInfo `json:"members"`
}

// ============================================================================
// 错误响应 DTO
// ============================================================================

// ErrorResponse 错误响应
// @Description API 错误响应格式
type ErrorResponse struct {
	// 错误码
	Code string `json:"code"`
	// 错误信息
	Message string `json:"message"`
	// 详细信息（可选）
	Details interface{} `json:"details,omitempty"`
}

// ValidationErrorDetails 验证错误详情
// @Description 数据验证失败时的详细错误信息
type ValidationErrorDetails struct {
	// 字段名
	Field string `json:"field"`
	// 错误信息
	Message string `json:"message"`
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
	}
}

// NewErrorResponseWithDetails 创建带详情的错误响应
func NewErrorResponseWithDetails(code, message string, details interface{}) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewValidationErrorResponse 创建验证错误响应
func NewValidationErrorResponse(errors []ValidationErrorDetails) *ErrorResponse {
	return &ErrorResponse{
		Code:    "VALIDATION_ERROR",
		Message: "数据验证失败",
		Details: errors,
	}
}

// ============================================================================
// 成功响应 DTO
// ============================================================================

// SuccessResponse 成功响应
// @Description 操作成功的通用响应
type SuccessResponse struct {
	// 成功信息
	Message string `json:"message"`
	// 附加数据（可选）
	Data interface{} `json:"data,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(message string) *SuccessResponse {
	return &SuccessResponse{
		Message: message,
	}
}

// NewSuccessResponseWithData 创建带数据的成功响应
func NewSuccessResponseWithData(message string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Message: message,
		Data:    data,
	}
}

// ============================================================================
// 可用性检查响应 DTO
// ============================================================================

// AvailabilityResponse 可用性检查响应
// @Description 检查用户名或邮箱是否可用的响应
type AvailabilityResponse struct {
	// 是否可用
	Available bool `json:"available"`
	// 提示信息
	Message string `json:"message,omitempty"`
}

// NewAvailabilityResponse 创建可用性检查响应
func NewAvailabilityResponse(available bool, message string) *AvailabilityResponse {
	return &AvailabilityResponse{
		Available: available,
		Message:   message,
	}
}

// ============================================================================
// 预定义错误码常量
// ============================================================================

const (
	// 认证相关错误码
	ErrCodeAuthTokenMissing     = "AUTH_TOKEN_MISSING"
	ErrCodeAuthTokenInvalid     = "AUTH_TOKEN_INVALID"
	ErrCodeAuthTokenExpired     = "AUTH_TOKEN_EXPIRED"
	ErrCodeAuthInvalidCredentials = "AUTH_INVALID_CREDENTIALS"
	ErrCodeAuthUserSuspended    = "AUTH_USER_SUSPENDED"
	ErrCodeAuthUserDisabled     = "AUTH_USER_DISABLED"
	ErrCodeAuthUserPending      = "AUTH_USER_PENDING"

	// 权限相关错误码
	ErrCodePermissionDenied = "PERMISSION_DENIED"

	// 资源相关错误码
	ErrCodeUserNotFound  = "USER_NOT_FOUND"
	ErrCodeGroupNotFound = "GROUP_NOT_FOUND"

	// 冲突相关错误码
	ErrCodeUserAlreadyExists  = "USER_ALREADY_EXISTS"
	ErrCodeGroupAlreadyExists = "GROUP_ALREADY_EXISTS"

	// 验证相关错误码
	ErrCodeValidationError = "VALIDATION_ERROR"
	ErrCodeInvalidPassword = "INVALID_PASSWORD"

	// 业务相关错误码
	ErrCodeRegistrationDisabled = "REGISTRATION_DISABLED"
	ErrCodeGroupHasMembers      = "GROUP_HAS_MEMBERS"
	ErrCodeGroupHasChildren     = "GROUP_HAS_CHILDREN"

	// 系统相关错误码
	ErrCodeDatabaseError  = "DATABASE_ERROR"
	ErrCodeInternalError  = "INTERNAL_ERROR"
)

// ============================================================================
// 预定义错误消息常量
// ============================================================================

const (
	// 认证相关错误消息
	ErrMsgAuthTokenMissing     = "缺少认证 Token"
	ErrMsgAuthTokenInvalid     = "Token 无效或已过期"
	ErrMsgAuthTokenExpired     = "Token 已过期"
	ErrMsgAuthInvalidCredentials = "用户名或密码错误"
	ErrMsgAuthUserSuspended    = "用户已被暂停"
	ErrMsgAuthUserDisabled     = "用户已被禁用"
	ErrMsgAuthUserPending      = "用户待审核"

	// 权限相关错误消息
	ErrMsgPermissionDenied = "权限不足"

	// 资源相关错误消息
	ErrMsgUserNotFound  = "用户不存在"
	ErrMsgGroupNotFound = "用户组不存在"

	// 冲突相关错误消息
	ErrMsgUsernameExists = "用户名已存在"
	ErrMsgEmailExists    = "邮箱已存在"
	ErrMsgGroupExists    = "用户组名已存在"

	// 验证相关错误消息
	ErrMsgValidationError = "数据验证失败"
	ErrMsgInvalidPassword = "密码不符合要求"
	ErrMsgWrongPassword   = "当前密码错误"

	// 业务相关错误消息
	ErrMsgRegistrationDisabled = "注册功能已关闭"
	ErrMsgGroupHasMembers      = "用户组中还有成员，无法删除"
	ErrMsgGroupHasChildren     = "用户组还有子组，无法删除"

	// 系统相关错误消息
	ErrMsgDatabaseError = "数据库操作失败"
	ErrMsgInternalError = "内部服务器错误"
)
