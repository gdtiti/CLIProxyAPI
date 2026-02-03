package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOAuthAccount_TableName(t *testing.T) {
	account := OAuthAccount{}
	assert.Equal(t, "oauth_accounts", account.TableName())
}

func TestOAuthAccount_IsTokenExpired(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name           string
		tokenExpiresAt *time.Time
		expected       bool
	}{
		{"Token 已过期", &pastTime, true},
		{"Token 未过期", &futureTime, false},
		{"无过期时间", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := OAuthAccount{TokenExpiresAt: tt.tokenExpiresAt}
			assert.Equal(t, tt.expected, account.IsTokenExpired())
		})
	}
}

func TestOAuthAccount_HasValidToken(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour)
	futureTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name           string
		accessToken    string
		tokenExpiresAt *time.Time
		expected       bool
	}{
		{"有效 Token", "valid_token", &futureTime, true},
		{"Token 已过期", "valid_token", &pastTime, false},
		{"无 Token", "", &futureTime, false},
		{"无过期时间的有效 Token", "valid_token", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := OAuthAccount{
				AccessToken:    tt.accessToken,
				TokenExpiresAt: tt.tokenExpiresAt,
			}
			assert.Equal(t, tt.expected, account.HasValidToken())
		})
	}
}

func TestOAuthAccount_HasRefreshToken(t *testing.T) {
	tests := []struct {
		name         string
		refreshToken string
		expected     bool
	}{
		{"有 Refresh Token", "refresh_token", true},
		{"无 Refresh Token", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := OAuthAccount{RefreshToken: tt.refreshToken}
			assert.Equal(t, tt.expected, account.HasRefreshToken())
		})
	}
}

func TestOAuthAccount_UpdateLastUsed(t *testing.T) {
	account := OAuthAccount{}
	assert.Nil(t, account.LastUsedAt)

	account.UpdateLastUsed()
	assert.NotNil(t, account.LastUsedAt)
	assert.True(t, time.Since(*account.LastUsedAt) < time.Second)
}

func TestOAuthAccount_UpdateTokens(t *testing.T) {
	account := OAuthAccount{}
	expiresAt := time.Now().Add(1 * time.Hour)

	account.UpdateTokens("new_access", "new_refresh", &expiresAt)
	assert.Equal(t, "new_access", account.AccessToken)
	assert.Equal(t, "new_refresh", account.RefreshToken)
	assert.Equal(t, &expiresAt, account.TokenExpiresAt)

	// 测试空 refresh token 不覆盖
	account.UpdateTokens("newer_access", "", nil)
	assert.Equal(t, "newer_access", account.AccessToken)
	assert.Equal(t, "new_refresh", account.RefreshToken) // 保持不变
}

func TestOAuthAccount_Scopes(t *testing.T) {
	account := OAuthAccount{
		Scopes: StringArray{"read", "write"},
	}

	// 测试 HasScope
	assert.True(t, account.HasScope("read"))
	assert.True(t, account.HasScope("write"))
	assert.False(t, account.HasScope("delete"))

	// 测试 AddScope
	account.AddScope("delete")
	assert.True(t, account.HasScope("delete"))
}

func TestOAuthAccount_RawData(t *testing.T) {
	account := OAuthAccount{}

	// 测试 SetRawData
	account.SetRawData("key1", "value1")
	assert.Equal(t, "value1", account.GetRawData("key1"))

	// 测试 GetRawData 不存在的键
	assert.Nil(t, account.GetRawData("nonexistent"))

	// 测试 nil RawData
	account2 := OAuthAccount{}
	assert.Nil(t, account2.GetRawData("key"))
}

func TestOAuthAccount_Sanitize(t *testing.T) {
	account := OAuthAccount{
		AccessToken:  "secret_access",
		RefreshToken: "secret_refresh",
		RawData:      JSON{"key": "value"},
	}

	account.Sanitize()
	assert.Empty(t, account.AccessToken)
	assert.Empty(t, account.RefreshToken)
	assert.Nil(t, account.RawData)
}

func TestOAuthAccount_Clone(t *testing.T) {
	account := &OAuthAccount{
		ID:       "account-123",
		Provider: OAuthProviderGoogle,
	}

	clone := account.Clone()
	assert.Equal(t, account.ID, clone.ID)
	assert.Equal(t, account.Provider, clone.Provider)

	// 修改克隆不影响原始
	clone.Provider = OAuthProviderGitHub
	assert.NotEqual(t, account.Provider, clone.Provider)
}

func TestOAuthAccount_Clone_Nil(t *testing.T) {
	var account *OAuthAccount
	clone := account.Clone()
	assert.Nil(t, clone)
}

func TestOAuthAccount_GetProviderDisplayName(t *testing.T) {
	tests := []struct {
		provider OAuthProvider
		expected string
	}{
		{OAuthProviderGoogle, "Google"},
		{OAuthProviderWechat, "微信"},
		{OAuthProviderGitHub, "GitHub"},
		{OAuthProviderApple, "Apple"},
		{OAuthProviderMicrosoft, "Microsoft"},
		{OAuthProvider("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			account := OAuthAccount{Provider: tt.provider}
			assert.Equal(t, tt.expected, account.GetProviderDisplayName())
		})
	}
}
