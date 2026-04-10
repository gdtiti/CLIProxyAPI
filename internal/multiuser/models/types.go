// Package models 定义多用户模块的数据模型
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// ============================================================================
// 用户角色枚举
// ============================================================================

// UserRole 用户角色枚举类型
// 验证: 需求 2.2
type UserRole string

const (
	// RoleAdmin 管理员角色，拥有系统所有权限
	RoleAdmin UserRole = "admin"
	// RoleOperator 操作员角色，拥有部分管理权限
	RoleOperator UserRole = "operator"
	// RoleUser 普通用户角色，拥有基本使用权限
	RoleUser UserRole = "user"
)

// validUserRoles 有效的用户角色列表
var validUserRoles = map[UserRole]bool{
	RoleAdmin:    true,
	RoleOperator: true,
	RoleUser:     true,
}

// IsValid 验证用户角色是否有效
func (r UserRole) IsValid() bool {
	return validUserRoles[r]
}

// String 返回用户角色的字符串表示
func (r UserRole) String() string {
	return string(r)
}

// AllUserRoles 返回所有有效的用户角色
func AllUserRoles() []UserRole {
	return []UserRole{RoleAdmin, RoleOperator, RoleUser}
}

// ParseUserRole 从字符串解析用户角色
func ParseUserRole(s string) (UserRole, error) {
	role := UserRole(s)
	if !role.IsValid() {
		return "", fmt.Errorf("无效的用户角色: %s", s)
	}
	return role, nil
}

// ============================================================================
// 用户状态枚举
// ============================================================================

// UserStatus 用户状态枚举类型
// 验证: 需求 2.3
type UserStatus string

const (
	// StatusActive 活跃状态，用户可以正常使用系统
	StatusActive UserStatus = "active"
	// StatusSuspended 暂停状态，用户暂时无法使用系统
	StatusSuspended UserStatus = "suspended"
	// StatusDisabled 禁用状态，用户被永久禁止使用系统
	StatusDisabled UserStatus = "disabled"
	// StatusPending 待审核状态，用户等待管理员审核
	StatusPending UserStatus = "pending"
)

// validUserStatuses 有效的用户状态列表
var validUserStatuses = map[UserStatus]bool{
	StatusActive:    true,
	StatusSuspended: true,
	StatusDisabled:  true,
	StatusPending:   true,
}

// IsValid 验证用户状态是否有效
func (s UserStatus) IsValid() bool {
	return validUserStatuses[s]
}

// String 返回用户状态的字符串表示
func (s UserStatus) String() string {
	return string(s)
}

// AllUserStatuses 返回所有有效的用户状态
func AllUserStatuses() []UserStatus {
	return []UserStatus{StatusActive, StatusSuspended, StatusDisabled, StatusPending}
}

// ParseUserStatus 从字符串解析用户状态
func ParseUserStatus(s string) (UserStatus, error) {
	status := UserStatus(s)
	if !status.IsValid() {
		return "", fmt.Errorf("无效的用户状态: %s", s)
	}
	return status, nil
}

// CanLogin 判断该状态的用户是否可以登录
func (s UserStatus) CanLogin() bool {
	return s == StatusActive
}

// ============================================================================
// 注册来源枚举
// ============================================================================

// RegisterSource 注册来源枚举类型
// 验证: 需求 2.4
type RegisterSource string

const (
	// RegisterSourceLocal 本地注册
	RegisterSourceLocal RegisterSource = "local"
	// RegisterSourceGoogle Google 第三方登录注册
	RegisterSourceGoogle RegisterSource = "google"
	// RegisterSourceWechat 微信第三方登录注册
	RegisterSourceWechat RegisterSource = "wechat"
	// RegisterSourceGitHub GitHub 第三方登录注册
	RegisterSourceGitHub RegisterSource = "github"
	// RegisterSourceInvite 邀请注册
	RegisterSourceInvite RegisterSource = "invite"
	// RegisterSourceAdmin 管理员创建
	RegisterSourceAdmin RegisterSource = "admin"
)

// validRegisterSources 有效的注册来源列表
var validRegisterSources = map[RegisterSource]bool{
	RegisterSourceLocal:  true,
	RegisterSourceGoogle: true,
	RegisterSourceWechat: true,
	RegisterSourceGitHub: true,
	RegisterSourceInvite: true,
	RegisterSourceAdmin:  true,
}

// IsValid 验证注册来源是否有效
func (r RegisterSource) IsValid() bool {
	return validRegisterSources[r]
}

// String 返回注册来源的字符串表示
func (r RegisterSource) String() string {
	return string(r)
}

// AllRegisterSources 返回所有有效的注册来源
func AllRegisterSources() []RegisterSource {
	return []RegisterSource{
		RegisterSourceLocal,
		RegisterSourceGoogle,
		RegisterSourceWechat,
		RegisterSourceGitHub,
		RegisterSourceInvite,
		RegisterSourceAdmin,
	}
}

// ParseRegisterSource 从字符串解析注册来源
func ParseRegisterSource(s string) (RegisterSource, error) {
	source := RegisterSource(s)
	if !source.IsValid() {
		return "", fmt.Errorf("无效的注册来源: %s", s)
	}
	return source, nil
}

// IsOAuth 判断是否为 OAuth 第三方登录来源
func (r RegisterSource) IsOAuth() bool {
	return r == RegisterSourceGoogle || r == RegisterSourceWechat || r == RegisterSourceGitHub
}

// ============================================================================
// OAuth 供应商枚举
// ============================================================================

// OAuthProvider OAuth 供应商枚举类型
// 验证: 需求 3.2
type OAuthProvider string

const (
	// OAuthProviderGoogle Google OAuth 供应商
	OAuthProviderGoogle OAuthProvider = "google"
	// OAuthProviderWechat 微信 OAuth 供应商
	OAuthProviderWechat OAuthProvider = "wechat"
	// OAuthProviderGitHub GitHub OAuth 供应商
	OAuthProviderGitHub OAuthProvider = "github"
	// OAuthProviderApple Apple OAuth 供应商
	OAuthProviderApple OAuthProvider = "apple"
	// OAuthProviderMicrosoft Microsoft OAuth 供应商
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
)

// validOAuthProviders 有效的 OAuth 供应商列表
var validOAuthProviders = map[OAuthProvider]bool{
	OAuthProviderGoogle:    true,
	OAuthProviderWechat:    true,
	OAuthProviderGitHub:    true,
	OAuthProviderApple:     true,
	OAuthProviderMicrosoft: true,
}

// IsValid 验证 OAuth 供应商是否有效
func (p OAuthProvider) IsValid() bool {
	return validOAuthProviders[p]
}

// String 返回 OAuth 供应商的字符串表示
func (p OAuthProvider) String() string {
	return string(p)
}

// AllOAuthProviders 返回所有有效的 OAuth 供应商
func AllOAuthProviders() []OAuthProvider {
	return []OAuthProvider{
		OAuthProviderGoogle,
		OAuthProviderWechat,
		OAuthProviderGitHub,
		OAuthProviderApple,
		OAuthProviderMicrosoft,
	}
}

// ParseOAuthProvider 从字符串解析 OAuth 供应商
func ParseOAuthProvider(s string) (OAuthProvider, error) {
	provider := OAuthProvider(s)
	if !provider.IsValid() {
		return "", fmt.Errorf("无效的 OAuth 供应商: %s", s)
	}
	return provider, nil
}

// ============================================================================
// GORM 自定义类型 - JSON
// ============================================================================

// JSON 自定义 JSON 类型，用于存储 PostgreSQL JSONB 数据
// 实现 GORM 的 Scanner 和 Valuer 接口
type JSON map[string]interface{}

// Value 实现 driver.Valuer 接口，将 JSON 转换为数据库值
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口，从数据库值扫描到 JSON
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("无法将值扫描到 JSON 类型")
	}

	result := make(JSON)
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("JSON 解析失败: %w", err)
	}
	*j = result
	return nil
}

// Get 获取 JSON 中指定键的值
func (j JSON) Get(key string) interface{} {
	if j == nil {
		return nil
	}
	return j[key]
}

// Set 设置 JSON 中指定键的值
func (j JSON) Set(key string, value interface{}) {
	if j != nil {
		j[key] = value
	}
}

// Delete 删除 JSON 中指定键
func (j JSON) Delete(key string) {
	if j != nil {
		delete(j, key)
	}
}

// Has 检查 JSON 中是否存在指定键
func (j JSON) Has(key string) bool {
	if j == nil {
		return false
	}
	_, exists := j[key]
	return exists
}

// MarshalJSON 实现 json.Marshaler 接口
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]interface{}(j))
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (j *JSON) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*j = nil
		return nil
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*j = result
	return nil
}

// ============================================================================
// GORM 自定义类型 - StringArray
// ============================================================================

// StringArray 自定义字符串数组类型，用于存储 PostgreSQL text[] 数组
// 实现 GORM 的 Scanner 和 Valuer 接口
type StringArray []string

// Value 实现 driver.Valuer 接口，将 StringArray 转换为数据库值
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.Array(a).Value()
}

// Scan 实现 sql.Scanner 接口，从数据库值扫描到 StringArray
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	// 使用 pq.StringArray 进行扫描
	var pgArray pq.StringArray
	if err := pgArray.Scan(value); err != nil {
		return fmt.Errorf("StringArray 扫描失败: %w", err)
	}
	*a = StringArray(pgArray)
	return nil
}

// Contains 检查数组是否包含指定元素
func (a StringArray) Contains(s string) bool {
	for _, item := range a {
		if item == s {
			return true
		}
	}
	return false
}

// Add 添加元素到数组（如果不存在）
func (a *StringArray) Add(s string) {
	if !a.Contains(s) {
		*a = append(*a, s)
	}
}

// Remove 从数组中移除指定元素
func (a *StringArray) Remove(s string) {
	result := make(StringArray, 0, len(*a))
	for _, item := range *a {
		if item != s {
			result = append(result, item)
		}
	}
	*a = result
}

// Len 返回数组长度
func (a StringArray) Len() int {
	return len(a)
}

// IsEmpty 检查数组是否为空
func (a StringArray) IsEmpty() bool {
	return len(a) == 0
}
