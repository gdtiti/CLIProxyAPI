// Package service 提供多用户模块的业务逻辑层
package service

import "errors"

// 服务层错误定义
var (
	// 用户相关错误
	ErrUserNotFound       = errors.New("用户不存在")
	ErrUserAlreadyExists  = errors.New("用户名或邮箱已存在")
	ErrUsernameExists     = errors.New("用户名已存在")
	ErrEmailExists        = errors.New("邮箱已存在")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserSuspended      = errors.New("用户已被暂停")
	ErrUserDisabled       = errors.New("用户已被禁用")
	ErrUserPending        = errors.New("用户待审核")
	ErrInvalidPassword    = errors.New("密码不符合要求")
	ErrWrongPassword      = errors.New("当前密码错误")

	// 用户组相关错误
	ErrGroupNotFound      = errors.New("用户组不存在")
	ErrGroupAlreadyExists = errors.New("用户组名已存在")
	ErrGroupHasMembers    = errors.New("用户组有成员，无法删除")
	ErrGroupHasChildren   = errors.New("用户组有子组，无法删除")
	ErrCircularReference  = errors.New("不能创建循环引用的用户组层级")

	// 认证相关错误
	ErrTokenInvalid       = errors.New("Token 无效")
	ErrTokenExpired       = errors.New("Token 已过期")
	ErrTokenBlacklisted   = errors.New("Token 已失效")
	ErrRefreshTokenInvalid = errors.New("Refresh Token 无效")
	ErrRegistrationDisabled = errors.New("注册功能已关闭")

	// OAuth 相关错误
	ErrOAuthAccountNotFound = errors.New("OAuth 账号不存在")
	ErrOAuthAccountExists   = errors.New("OAuth 账号已绑定")

	// 通用错误
	ErrInvalidInput = errors.New("输入参数无效")
	ErrInternal     = errors.New("内部服务器错误")
)
