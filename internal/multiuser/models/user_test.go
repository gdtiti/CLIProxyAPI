package models

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestUser_TableName(t *testing.T) {
	user := User{}
	assert.Equal(t, "users", user.TableName())
}

func TestUser_HasPassword(t *testing.T) {
	tests := []struct {
		name         string
		passwordHash string
		expected     bool
	}{
		{"有密码", "hashed_password", true},
		{"无密码", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{PasswordHash: tt.passwordHash}
			assert.Equal(t, tt.expected, user.HasPassword())
		})
	}
}

func TestUser_CanLogin(t *testing.T) {
	tests := []struct {
		name     string
		status   UserStatus
		expected bool
	}{
		{"active 可以登录", StatusActive, true},
		{"suspended 不能登录", StatusSuspended, false},
		{"disabled 不能登录", StatusDisabled, false},
		{"pending 不能登录", StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Status: tt.status}
			assert.Equal(t, tt.expected, user.CanLogin())
		})
	}
}

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		role     UserRole
		expected bool
	}{
		{"admin 是管理员", RoleAdmin, true},
		{"operator 不是管理员", RoleOperator, false},
		{"user 不是管理员", RoleUser, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Role: tt.role}
			assert.Equal(t, tt.expected, user.IsAdmin())
		})
	}
}

func TestUser_IsOperator(t *testing.T) {
	tests := []struct {
		name     string
		role     UserRole
		expected bool
	}{
		{"operator 是操作员", RoleOperator, true},
		{"admin 不是操作员", RoleAdmin, false},
		{"user 不是操作员", RoleUser, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Role: tt.role}
			assert.Equal(t, tt.expected, user.IsOperator())
		})
	}
}

func TestUser_HasGroup(t *testing.T) {
	groupID := "group-123"
	emptyGroupID := ""

	tests := []struct {
		name     string
		groupID  *string
		expected bool
	}{
		{"有用户组", &groupID, true},
		{"无用户组 - nil", nil, false},
		{"无用户组 - 空字符串", &emptyGroupID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{GroupID: tt.groupID}
			assert.Equal(t, tt.expected, user.HasGroup())
		})
	}
}

func TestUser_GetAvailableBalance(t *testing.T) {
	user := User{
		Balance:     decimal.NewFromFloat(100.50),
		CreditLimit: decimal.NewFromFloat(50.25),
	}
	expected := decimal.NewFromFloat(150.75)
	assert.True(t, expected.Equal(user.GetAvailableBalance()))
}

func TestUser_IsOAuthUser(t *testing.T) {
	tests := []struct {
		name           string
		registerSource RegisterSource
		expected       bool
	}{
		{"local 不是 OAuth", RegisterSourceLocal, false},
		{"google 是 OAuth", RegisterSourceGoogle, true},
		{"wechat 是 OAuth", RegisterSourceWechat, true},
		{"github 是 OAuth", RegisterSourceGitHub, true},
		{"invite 不是 OAuth", RegisterSourceInvite, false},
		{"admin 不是 OAuth", RegisterSourceAdmin, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{RegisterSource: tt.registerSource}
			assert.Equal(t, tt.expected, user.IsOAuthUser())
		})
	}
}

func TestUser_Metadata(t *testing.T) {
	user := User{}

	// 测试 SetMetadata
	user.SetMetadata("key1", "value1")
	assert.Equal(t, "value1", user.GetMetadata("key1"))

	// 测试 GetMetadata 不存在的键
	assert.Nil(t, user.GetMetadata("nonexistent"))

	// 测试 nil Metadata
	user2 := User{}
	assert.Nil(t, user2.GetMetadata("key"))
}

func TestUser_UpdateLastLogin(t *testing.T) {
	user := User{}
	assert.Nil(t, user.LastLoginAt)

	user.UpdateLastLogin()
	assert.NotNil(t, user.LastLoginAt)
	assert.True(t, time.Since(*user.LastLoginAt) < time.Second)
}

func TestUser_Sanitize(t *testing.T) {
	user := User{
		PasswordHash: "secret_hash",
	}
	user.Sanitize()
	assert.Empty(t, user.PasswordHash)
}

func TestUser_Clone(t *testing.T) {
	user := &User{
		ID:       "user-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	clone := user.Clone()
	assert.Equal(t, user.ID, clone.ID)
	assert.Equal(t, user.Username, clone.Username)

	// 修改克隆不影响原始
	clone.Username = "modified"
	assert.NotEqual(t, user.Username, clone.Username)
}

func TestUser_Clone_Nil(t *testing.T) {
	var user *User
	clone := user.Clone()
	assert.Nil(t, clone)
}
