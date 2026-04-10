// Package repository 提供多用户模块的数据访问层
package repository

import "errors"

// 仓库层错误定义
var (
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("用户不存在")

	// ErrGroupNotFound 用户组不存在
	ErrGroupNotFound = errors.New("用户组不存在")

	// ErrOAuthAccountNotFound OAuth 账号不存在
	ErrOAuthAccountNotFound = errors.New("OAuth 账号不存在")

	// ErrDuplicateUsername 用户名已存在
	ErrDuplicateUsername = errors.New("用户名已存在")

	// ErrDuplicateEmail 邮箱已存在
	ErrDuplicateEmail = errors.New("邮箱已存在")

	// ErrDuplicateGroupName 用户组名已存在
	ErrDuplicateGroupName = errors.New("用户组名已存在")

	// ErrDuplicateOAuthAccount OAuth 账号已绑定
	ErrDuplicateOAuthAccount = errors.New("OAuth 账号已绑定")

	// ErrGroupHasMembers 用户组有成员，无法删除
	ErrGroupHasMembers = errors.New("用户组有成员，无法删除")

	// ErrGroupHasChildren 用户组有子组，无法删除
	ErrGroupHasChildren = errors.New("用户组有子组，无法删除")
)
