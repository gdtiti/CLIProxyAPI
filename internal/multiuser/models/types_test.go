package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// UserRole 测试
// ============================================================================

func TestUserRole_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		role     UserRole
		expected bool
	}{
		{"admin 角色有效", RoleAdmin, true},
		{"operator 角色有效", RoleOperator, true},
		{"user 角色有效", RoleUser, true},
		{"空字符串无效", UserRole(""), false},
		{"未知角色无效", UserRole("unknown"), false},
		{"大写角色无效", UserRole("ADMIN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.role.IsValid())
		})
	}
}

func TestUserRole_String(t *testing.T) {
	assert.Equal(t, "admin", RoleAdmin.String())
	assert.Equal(t, "operator", RoleOperator.String())
	assert.Equal(t, "user", RoleUser.String())
}

func TestAllUserRoles(t *testing.T) {
	roles := AllUserRoles()
	assert.Len(t, roles, 3)
	assert.Contains(t, roles, RoleAdmin)
	assert.Contains(t, roles, RoleOperator)
	assert.Contains(t, roles, RoleUser)
}

func TestParseUserRole(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    UserRole
		expectError bool
	}{
		{"解析 admin", "admin", RoleAdmin, false},
		{"解析 operator", "operator", RoleOperator, false},
		{"解析 user", "user", RoleUser, false},
		{"解析无效角色", "invalid", "", true},
		{"解析空字符串", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ParseUserRole(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, role)
			}
		})
	}
}

// ============================================================================
// UserStatus 测试
// ============================================================================

func TestUserStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   UserStatus
		expected bool
	}{
		{"active 状态有效", StatusActive, true},
		{"suspended 状态有效", StatusSuspended, true},
		{"disabled 状态有效", StatusDisabled, true},
		{"pending 状态有效", StatusPending, true},
		{"空字符串无效", UserStatus(""), false},
		{"未知状态无效", UserStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func TestUserStatus_String(t *testing.T) {
	assert.Equal(t, "active", StatusActive.String())
	assert.Equal(t, "suspended", StatusSuspended.String())
	assert.Equal(t, "disabled", StatusDisabled.String())
	assert.Equal(t, "pending", StatusPending.String())
}

func TestUserStatus_CanLogin(t *testing.T) {
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
			assert.Equal(t, tt.expected, tt.status.CanLogin())
		})
	}
}

func TestAllUserStatuses(t *testing.T) {
	statuses := AllUserStatuses()
	assert.Len(t, statuses, 4)
	assert.Contains(t, statuses, StatusActive)
	assert.Contains(t, statuses, StatusSuspended)
	assert.Contains(t, statuses, StatusDisabled)
	assert.Contains(t, statuses, StatusPending)
}

func TestParseUserStatus(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    UserStatus
		expectError bool
	}{
		{"解析 active", "active", StatusActive, false},
		{"解析 suspended", "suspended", StatusSuspended, false},
		{"解析 disabled", "disabled", StatusDisabled, false},
		{"解析 pending", "pending", StatusPending, false},
		{"解析无效状态", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := ParseUserStatus(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, status)
			}
		})
	}
}

// ============================================================================
// RegisterSource 测试
// ============================================================================

func TestRegisterSource_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		source   RegisterSource
		expected bool
	}{
		{"local 来源有效", RegisterSourceLocal, true},
		{"google 来源有效", RegisterSourceGoogle, true},
		{"wechat 来源有效", RegisterSourceWechat, true},
		{"github 来源有效", RegisterSourceGitHub, true},
		{"invite 来源有效", RegisterSourceInvite, true},
		{"admin 来源有效", RegisterSourceAdmin, true},
		{"空字符串无效", RegisterSource(""), false},
		{"未知来源无效", RegisterSource("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.IsValid())
		})
	}
}

func TestRegisterSource_String(t *testing.T) {
	assert.Equal(t, "local", RegisterSourceLocal.String())
	assert.Equal(t, "google", RegisterSourceGoogle.String())
	assert.Equal(t, "wechat", RegisterSourceWechat.String())
	assert.Equal(t, "github", RegisterSourceGitHub.String())
	assert.Equal(t, "invite", RegisterSourceInvite.String())
	assert.Equal(t, "admin", RegisterSourceAdmin.String())
}

func TestRegisterSource_IsOAuth(t *testing.T) {
	tests := []struct {
		name     string
		source   RegisterSource
		expected bool
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
			assert.Equal(t, tt.expected, tt.source.IsOAuth())
		})
	}
}

func TestAllRegisterSources(t *testing.T) {
	sources := AllRegisterSources()
	assert.Len(t, sources, 6)
	assert.Contains(t, sources, RegisterSourceLocal)
	assert.Contains(t, sources, RegisterSourceGoogle)
	assert.Contains(t, sources, RegisterSourceWechat)
	assert.Contains(t, sources, RegisterSourceGitHub)
	assert.Contains(t, sources, RegisterSourceInvite)
	assert.Contains(t, sources, RegisterSourceAdmin)
}

func TestParseRegisterSource(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    RegisterSource
		expectError bool
	}{
		{"解析 local", "local", RegisterSourceLocal, false},
		{"解析 google", "google", RegisterSourceGoogle, false},
		{"解析 github", "github", RegisterSourceGitHub, false},
		{"解析无效来源", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ParseRegisterSource(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, source)
			}
		})
	}
}

// ============================================================================
// OAuthProvider 测试
// ============================================================================

func TestOAuthProvider_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider OAuthProvider
		expected bool
	}{
		{"google 供应商有效", OAuthProviderGoogle, true},
		{"wechat 供应商有效", OAuthProviderWechat, true},
		{"github 供应商有效", OAuthProviderGitHub, true},
		{"apple 供应商有效", OAuthProviderApple, true},
		{"microsoft 供应商有效", OAuthProviderMicrosoft, true},
		{"空字符串无效", OAuthProvider(""), false},
		{"未知供应商无效", OAuthProvider("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.IsValid())
		})
	}
}

func TestOAuthProvider_String(t *testing.T) {
	assert.Equal(t, "google", OAuthProviderGoogle.String())
	assert.Equal(t, "wechat", OAuthProviderWechat.String())
	assert.Equal(t, "github", OAuthProviderGitHub.String())
	assert.Equal(t, "apple", OAuthProviderApple.String())
	assert.Equal(t, "microsoft", OAuthProviderMicrosoft.String())
}

func TestAllOAuthProviders(t *testing.T) {
	providers := AllOAuthProviders()
	assert.Len(t, providers, 5)
	assert.Contains(t, providers, OAuthProviderGoogle)
	assert.Contains(t, providers, OAuthProviderWechat)
	assert.Contains(t, providers, OAuthProviderGitHub)
	assert.Contains(t, providers, OAuthProviderApple)
	assert.Contains(t, providers, OAuthProviderMicrosoft)
}

func TestParseOAuthProvider(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    OAuthProvider
		expectError bool
	}{
		{"解析 google", "google", OAuthProviderGoogle, false},
		{"解析 wechat", "wechat", OAuthProviderWechat, false},
		{"解析 github", "github", OAuthProviderGitHub, false},
		{"解析 apple", "apple", OAuthProviderApple, false},
		{"解析 microsoft", "microsoft", OAuthProviderMicrosoft, false},
		{"解析无效供应商", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := ParseOAuthProvider(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, provider)
			}
		})
	}
}

// ============================================================================
// JSON 类型测试
// ============================================================================

func TestJSON_ValueAndScan(t *testing.T) {
	t.Run("正常值的序列化和反序列化", func(t *testing.T) {
		original := JSON{
			"name":  "test",
			"count": float64(42),
			"active": true,
		}

		// Value
		value, err := original.Value()
		require.NoError(t, err)
		require.NotNil(t, value)

		// Scan
		var scanned JSON
		err = scanned.Scan(value)
		require.NoError(t, err)
		assert.Equal(t, original["name"], scanned["name"])
		assert.Equal(t, original["count"], scanned["count"])
		assert.Equal(t, original["active"], scanned["active"])
	})

	t.Run("nil 值处理", func(t *testing.T) {
		var j JSON = nil
		value, err := j.Value()
		require.NoError(t, err)
		assert.Nil(t, value)

		var scanned JSON
		err = scanned.Scan(nil)
		require.NoError(t, err)
		assert.Nil(t, scanned)
	})

	t.Run("从字符串扫描", func(t *testing.T) {
		var j JSON
		err := j.Scan(`{"key": "value"}`)
		require.NoError(t, err)
		assert.Equal(t, "value", j["key"])
	})

	t.Run("无效类型扫描失败", func(t *testing.T) {
		var j JSON
		err := j.Scan(12345)
		assert.Error(t, err)
	})

	t.Run("无效 JSON 扫描失败", func(t *testing.T) {
		var j JSON
		err := j.Scan([]byte(`{invalid json}`))
		assert.Error(t, err)
	})
}

func TestJSON_Operations(t *testing.T) {
	t.Run("Get 操作", func(t *testing.T) {
		j := JSON{"key": "value"}
		assert.Equal(t, "value", j.Get("key"))
		assert.Nil(t, j.Get("nonexistent"))

		var nilJSON JSON
		assert.Nil(t, nilJSON.Get("key"))
	})

	t.Run("Set 操作", func(t *testing.T) {
		j := JSON{}
		j.Set("key", "value")
		assert.Equal(t, "value", j["key"])
	})

	t.Run("Delete 操作", func(t *testing.T) {
		j := JSON{"key": "value"}
		j.Delete("key")
		assert.Nil(t, j["key"])
	})

	t.Run("Has 操作", func(t *testing.T) {
		j := JSON{"key": "value"}
		assert.True(t, j.Has("key"))
		assert.False(t, j.Has("nonexistent"))

		var nilJSON JSON
		assert.False(t, nilJSON.Has("key"))
	})
}

func TestJSON_MarshalUnmarshal(t *testing.T) {
	t.Run("正常 JSON 序列化", func(t *testing.T) {
		j := JSON{"name": "test", "count": float64(42)}
		data, err := json.Marshal(j)
		require.NoError(t, err)

		var result JSON
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, j["name"], result["name"])
		assert.Equal(t, j["count"], result["count"])
	})

	t.Run("nil JSON 序列化", func(t *testing.T) {
		var j JSON = nil
		data, err := json.Marshal(j)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})

	t.Run("null 反序列化", func(t *testing.T) {
		var j JSON
		err := json.Unmarshal([]byte("null"), &j)
		require.NoError(t, err)
		assert.Nil(t, j)
	})
}

// ============================================================================
// StringArray 类型测试
// ============================================================================

func TestStringArray_Operations(t *testing.T) {
	t.Run("Contains 操作", func(t *testing.T) {
		arr := StringArray{"a", "b", "c"}
		assert.True(t, arr.Contains("a"))
		assert.True(t, arr.Contains("b"))
		assert.False(t, arr.Contains("d"))
	})

	t.Run("Add 操作", func(t *testing.T) {
		arr := StringArray{"a", "b"}
		arr.Add("c")
		assert.True(t, arr.Contains("c"))
		assert.Len(t, arr, 3)

		// 添加已存在的元素不应重复
		arr.Add("a")
		assert.Len(t, arr, 3)
	})

	t.Run("Remove 操作", func(t *testing.T) {
		arr := StringArray{"a", "b", "c"}
		arr.Remove("b")
		assert.False(t, arr.Contains("b"))
		assert.Len(t, arr, 2)

		// 移除不存在的元素不应报错
		arr.Remove("nonexistent")
		assert.Len(t, arr, 2)
	})

	t.Run("Len 操作", func(t *testing.T) {
		arr := StringArray{"a", "b", "c"}
		assert.Equal(t, 3, arr.Len())

		var emptyArr StringArray
		assert.Equal(t, 0, emptyArr.Len())
	})

	t.Run("IsEmpty 操作", func(t *testing.T) {
		arr := StringArray{"a"}
		assert.False(t, arr.IsEmpty())

		var emptyArr StringArray
		assert.True(t, emptyArr.IsEmpty())

		emptyArr2 := StringArray{}
		assert.True(t, emptyArr2.IsEmpty())
	})
}

func TestStringArray_ValueAndScan(t *testing.T) {
	t.Run("nil 值处理", func(t *testing.T) {
		var arr StringArray = nil
		value, err := arr.Value()
		require.NoError(t, err)
		assert.Nil(t, value)

		var scanned StringArray
		err = scanned.Scan(nil)
		require.NoError(t, err)
		assert.Nil(t, scanned)
	})

	t.Run("正常数组的 Value", func(t *testing.T) {
		arr := StringArray{"a", "b", "c"}
		value, err := arr.Value()
		require.NoError(t, err)
		assert.NotNil(t, value)
	})
}
