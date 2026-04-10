package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDefaultConfig 测试默认配置创建
func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	assert.NotNil(t, cfg)
	assert.False(t, cfg.Enabled)

	// 验证数据库默认配置
	assert.Equal(t, "", cfg.Database.DSN)
	assert.Equal(t, DefaultSchema, cfg.Database.Schema)
	assert.Equal(t, DefaultMaxOpenConns, cfg.Database.MaxOpenConns)
	assert.Equal(t, DefaultMaxIdleConns, cfg.Database.MaxIdleConns)
	assert.Equal(t, DefaultConnMaxLifetime, cfg.Database.ConnMaxLifetime)

	// 验证认证默认配置
	assert.Equal(t, "", cfg.Auth.JWTSecret)
	assert.Equal(t, DefaultAccessTokenTTL, cfg.Auth.AccessTokenTTL)
	assert.Equal(t, DefaultRefreshTokenTTL, cfg.Auth.RefreshTokenTTL)

	// 验证注册默认配置
	assert.False(t, cfg.Registration.AllowPublicRegistration)
	assert.False(t, cfg.Registration.RequireEmailVerification)
	assert.False(t, cfg.Registration.RequireAdminApproval)
	assert.Equal(t, "", cfg.Registration.DefaultGroupID)
	assert.Equal(t, DefaultBalance, cfg.Registration.DefaultBalance)
	assert.Equal(t, DefaultRole, cfg.Registration.DefaultRole)

	// 验证管理员默认配置
	assert.Equal(t, DefaultAdminUsername, cfg.Admin.Username)
	assert.Equal(t, "", cfg.Admin.Password)
	assert.Equal(t, DefaultAdminEmail, cfg.Admin.Email)
}

// TestSetDefaults 测试设置默认值
func TestSetDefaults(t *testing.T) {
	cfg := &MultiUserConfig{}
	cfg.SetDefaults()

	assert.Equal(t, DefaultSchema, cfg.Database.Schema)
	assert.Equal(t, DefaultMaxOpenConns, cfg.Database.MaxOpenConns)
	assert.Equal(t, DefaultMaxIdleConns, cfg.Database.MaxIdleConns)
	assert.Equal(t, DefaultConnMaxLifetime, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, DefaultAccessTokenTTL, cfg.Auth.AccessTokenTTL)
	assert.Equal(t, DefaultRefreshTokenTTL, cfg.Auth.RefreshTokenTTL)
	assert.Equal(t, DefaultBalance, cfg.Registration.DefaultBalance)
	assert.Equal(t, DefaultRole, cfg.Registration.DefaultRole)
	assert.Equal(t, DefaultAdminUsername, cfg.Admin.Username)
	assert.Equal(t, DefaultAdminEmail, cfg.Admin.Email)
}

// TestSetDefaults_DoesNotOverrideExisting 测试设置默认值不覆盖已有值
func TestSetDefaults_DoesNotOverrideExisting(t *testing.T) {
	cfg := &MultiUserConfig{
		Database: DatabaseConfig{
			Schema:       "custom_schema",
			MaxOpenConns: 50,
		},
		Auth: AuthConfig{
			AccessTokenTTL: 48 * time.Hour,
		},
		Registration: RegistrationConfig{
			DefaultBalance: "100.00",
			DefaultRole:    "operator",
		},
		Admin: AdminConfig{
			Username: "superadmin",
			Email:    "super@example.com",
		},
	}
	cfg.SetDefaults()

	// 已有值不应被覆盖
	assert.Equal(t, "custom_schema", cfg.Database.Schema)
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 48*time.Hour, cfg.Auth.AccessTokenTTL)
	assert.Equal(t, "100.00", cfg.Registration.DefaultBalance)
	assert.Equal(t, "operator", cfg.Registration.DefaultRole)
	assert.Equal(t, "superadmin", cfg.Admin.Username)
	assert.Equal(t, "super@example.com", cfg.Admin.Email)

	// 零值应被设置为默认值
	assert.Equal(t, DefaultMaxIdleConns, cfg.Database.MaxIdleConns)
	assert.Equal(t, DefaultRefreshTokenTTL, cfg.Auth.RefreshTokenTTL)
}

// TestValidate_DisabledModule 测试禁用模块时跳过验证
func TestValidate_DisabledModule(t *testing.T) {
	cfg := &MultiUserConfig{
		Enabled: false,
		// 其他配置为空，正常情况下会验证失败
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

// TestValidate_ValidConfig 测试有效配置验证通过
func TestValidate_ValidConfig(t *testing.T) {
	cfg := &MultiUserConfig{
		Enabled: true,
		Database: DatabaseConfig{
			DSN:             "postgres://user:pass@localhost:5432/db",
			Schema:          "multiuser",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Auth: AuthConfig{
			JWTSecret:       "this-is-a-very-long-secret-key-for-jwt-signing",
			AccessTokenTTL:  24 * time.Hour,
			RefreshTokenTTL: 168 * time.Hour,
		},
		Registration: RegistrationConfig{
			AllowPublicRegistration: true,
			DefaultBalance:          "10.00",
			DefaultRole:             "user",
		},
		Admin: AdminConfig{
			Username: "admin",
			Password: "securepassword123",
			Email:    "admin@example.com",
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

// TestValidate_DatabaseConfig 测试数据库配置验证
func TestValidate_DatabaseConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    DatabaseConfig
		expectErr bool
		errField  string
	}{
		{
			name: "有效配置",
			config: DatabaseConfig{
				DSN:             "postgres://user:pass@localhost:5432/db",
				MaxOpenConns:    25,
				MaxIdleConns:    10,
				ConnMaxLifetime: 5 * time.Minute,
			},
			expectErr: false,
		},
		{
			name: "DSN 为空",
			config: DatabaseConfig{
				DSN: "",
			},
			expectErr: true,
			errField:  "database.dsn",
		},
		{
			name: "MaxOpenConns 为负数",
			config: DatabaseConfig{
				DSN:          "postgres://user:pass@localhost:5432/db",
				MaxOpenConns: -1,
			},
			expectErr: true,
			errField:  "database.max-open-conns",
		},
		{
			name: "MaxIdleConns 大于 MaxOpenConns",
			config: DatabaseConfig{
				DSN:          "postgres://user:pass@localhost:5432/db",
				MaxOpenConns: 10,
				MaxIdleConns: 20,
			},
			expectErr: true,
			errField:  "database.max-idle-conns",
		},
		{
			name: "ConnMaxLifetime 为负数",
			config: DatabaseConfig{
				DSN:             "postgres://user:pass@localhost:5432/db",
				ConnMaxLifetime: -1 * time.Minute,
			},
			expectErr: true,
			errField:  "database.conn-max-lifetime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				if ve, ok := err.(ValidationErrors); ok {
					found := false
					for _, e := range ve {
						if e.Field == tt.errField {
							found = true
							break
						}
					}
					assert.True(t, found, "期望字段 %s 验证失败", tt.errField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_AuthConfig 测试认证配置验证
func TestValidate_AuthConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    AuthConfig
		expectErr bool
		errField  string
	}{
		{
			name: "有效配置",
			config: AuthConfig{
				JWTSecret:       "this-is-a-very-long-secret-key-for-jwt",
				AccessTokenTTL:  24 * time.Hour,
				RefreshTokenTTL: 168 * time.Hour,
			},
			expectErr: false,
		},
		{
			name: "JWTSecret 为空",
			config: AuthConfig{
				JWTSecret:       "",
				AccessTokenTTL:  24 * time.Hour,
				RefreshTokenTTL: 168 * time.Hour,
			},
			expectErr: true,
			errField:  "auth.jwt-secret",
		},
		{
			name: "JWTSecret 太短",
			config: AuthConfig{
				JWTSecret:       "short",
				AccessTokenTTL:  24 * time.Hour,
				RefreshTokenTTL: 168 * time.Hour,
			},
			expectErr: true,
			errField:  "auth.jwt-secret",
		},
		{
			name: "AccessTokenTTL 为零",
			config: AuthConfig{
				JWTSecret:       "this-is-a-very-long-secret-key-for-jwt",
				AccessTokenTTL:  0,
				RefreshTokenTTL: 168 * time.Hour,
			},
			expectErr: true,
			errField:  "auth.access-token-ttl",
		},
		{
			name: "RefreshTokenTTL 小于 AccessTokenTTL",
			config: AuthConfig{
				JWTSecret:       "this-is-a-very-long-secret-key-for-jwt",
				AccessTokenTTL:  24 * time.Hour,
				RefreshTokenTTL: 1 * time.Hour,
			},
			expectErr: true,
			errField:  "auth.refresh-token-ttl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				if ve, ok := err.(ValidationErrors); ok {
					found := false
					for _, e := range ve {
						if e.Field == tt.errField {
							found = true
							break
						}
					}
					assert.True(t, found, "期望字段 %s 验证失败", tt.errField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_RegistrationConfig 测试注册配置验证
func TestValidate_RegistrationConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    RegistrationConfig
		expectErr bool
		errField  string
	}{
		{
			name: "有效配置",
			config: RegistrationConfig{
				DefaultBalance: "10.00",
				DefaultRole:    "user",
			},
			expectErr: false,
		},
		{
			name: "有效配置 - operator 角色",
			config: RegistrationConfig{
				DefaultBalance: "10.00",
				DefaultRole:    "operator",
			},
			expectErr: false,
		},
		{
			name: "无效的默认余额格式",
			config: RegistrationConfig{
				DefaultBalance: "invalid",
				DefaultRole:    "user",
			},
			expectErr: true,
			errField:  "registration.default-balance",
		},
		{
			name: "无效的默认角色",
			config: RegistrationConfig{
				DefaultBalance: "10.00",
				DefaultRole:    "invalid_role",
			},
			expectErr: true,
			errField:  "registration.default-role",
		},
		{
			name: "默认角色不能为 admin",
			config: RegistrationConfig{
				DefaultBalance: "10.00",
				DefaultRole:    "admin",
			},
			expectErr: true,
			errField:  "registration.default-role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				if ve, ok := err.(ValidationErrors); ok {
					found := false
					for _, e := range ve {
						if e.Field == tt.errField {
							found = true
							break
						}
					}
					assert.True(t, found, "期望字段 %s 验证失败", tt.errField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_AdminConfig 测试管理员配置验证
func TestValidate_AdminConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    AdminConfig
		expectErr bool
		errField  string
	}{
		{
			name: "有效配置",
			config: AdminConfig{
				Username: "admin",
				Password: "securepassword123",
				Email:    "admin@example.com",
			},
			expectErr: false,
		},
		{
			name: "有效配置 - 密码为空（将自动生成）",
			config: AdminConfig{
				Username: "admin",
				Password: "",
				Email:    "admin@example.com",
			},
			expectErr: false,
		},
		{
			name: "用户名为空",
			config: AdminConfig{
				Username: "",
				Email:    "admin@example.com",
			},
			expectErr: true,
			errField:  "admin.username",
		},
		{
			name: "用户名太短",
			config: AdminConfig{
				Username: "ab",
				Email:    "admin@example.com",
			},
			expectErr: true,
			errField:  "admin.username",
		},
		{
			name: "密码太短",
			config: AdminConfig{
				Username: "admin",
				Password: "short",
				Email:    "admin@example.com",
			},
			expectErr: true,
			errField:  "admin.password",
		},
		{
			name: "邮箱为空",
			config: AdminConfig{
				Username: "admin",
				Email:    "",
			},
			expectErr: true,
			errField:  "admin.email",
		},
		{
			name: "邮箱格式无效 - 无 @",
			config: AdminConfig{
				Username: "admin",
				Email:    "invalid-email",
			},
			expectErr: true,
			errField:  "admin.email",
		},
		{
			name: "邮箱格式无效 - 无域名",
			config: AdminConfig{
				Username: "admin",
				Email:    "admin@",
			},
			expectErr: true,
			errField:  "admin.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				if ve, ok := err.(ValidationErrors); ok {
					found := false
					for _, e := range ve {
						if e.Field == tt.errField {
							found = true
							break
						}
					}
					assert.True(t, found, "期望字段 %s 验证失败", tt.errField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestIsValidEmail 测试邮箱验证函数
func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"admin@example.com", true},
		{"user.name@domain.org", true},
		{"test@sub.domain.com", true},
		{"", false},
		{"invalid", false},
		{"@example.com", false},
		{"admin@", false},
		{"admin@domain", false},
		{"admin@@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isValidEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetDefaultBalanceDecimal 测试获取默认余额
func TestGetDefaultBalanceDecimal(t *testing.T) {
	tests := []struct {
		name           string
		defaultBalance string
		expectErr      bool
		expectedValue  string
	}{
		{
			name:           "有效余额",
			defaultBalance: "10.50",
			expectErr:      false,
			expectedValue:  "10.5",
		},
		{
			name:           "空余额使用默认值",
			defaultBalance: "",
			expectErr:      false,
			expectedValue:  "1",
		},
		{
			name:           "无效余额",
			defaultBalance: "invalid",
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &RegistrationConfig{DefaultBalance: tt.defaultBalance}
			result, err := cfg.GetDefaultBalanceDecimal()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result.String())
			}
		})
	}
}

// TestClone 测试配置克隆
func TestClone(t *testing.T) {
	original := &MultiUserConfig{
		Enabled: true,
		Database: DatabaseConfig{
			DSN:    "postgres://localhost/db",
			Schema: "test",
		},
		Auth: AuthConfig{
			JWTSecret: "secret",
		},
	}

	cloned := original.Clone()

	// 验证克隆值相等
	assert.Equal(t, original.Enabled, cloned.Enabled)
	assert.Equal(t, original.Database.DSN, cloned.Database.DSN)
	assert.Equal(t, original.Auth.JWTSecret, cloned.Auth.JWTSecret)

	// 验证修改克隆不影响原始
	cloned.Database.DSN = "modified"
	assert.NotEqual(t, original.Database.DSN, cloned.Database.DSN)
}

// TestClone_Nil 测试 nil 配置克隆
func TestClone_Nil(t *testing.T) {
	var cfg *MultiUserConfig
	cloned := cfg.Clone()
	assert.Nil(t, cloned)
}

// TestMergeWith 测试配置合并
func TestMergeWith(t *testing.T) {
	base := NewDefaultConfig()
	base.Database.DSN = "original-dsn"

	other := &MultiUserConfig{
		Database: DatabaseConfig{
			Schema:       "custom_schema",
			MaxOpenConns: 100,
		},
		Auth: AuthConfig{
			JWTSecret: "new-secret",
		},
	}

	base.MergeWith(other)

	// 验证合并后的值
	assert.Equal(t, "original-dsn", base.Database.DSN) // 保持原值（other 为空）
	assert.Equal(t, "custom_schema", base.Database.Schema)
	assert.Equal(t, 100, base.Database.MaxOpenConns)
	assert.Equal(t, "new-secret", base.Auth.JWTSecret)
}

// TestMergeWith_Nil 测试与 nil 合并
func TestMergeWith_Nil(t *testing.T) {
	cfg := NewDefaultConfig()
	originalDSN := cfg.Database.DSN

	cfg.MergeWith(nil)

	// 验证配置未改变
	assert.Equal(t, originalDSN, cfg.Database.DSN)
}

// TestValidationError_Error 测试验证错误消息
func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Message: "测试错误消息",
	}

	assert.Contains(t, err.Error(), "test.field")
	assert.Contains(t, err.Error(), "测试错误消息")
}

// TestValidationErrors_Error 测试多个验证错误消息
func TestValidationErrors_Error(t *testing.T) {
	errs := ValidationErrors{
		{Field: "field1", Message: "错误1"},
		{Field: "field2", Message: "错误2"},
	}

	errMsg := errs.Error()
	assert.Contains(t, errMsg, "2 个错误")
}

// TestValidationErrors_HasErrors 测试检查是否有错误
func TestValidationErrors_HasErrors(t *testing.T) {
	var emptyErrs ValidationErrors
	assert.False(t, emptyErrs.HasErrors())

	errs := ValidationErrors{{Field: "test", Message: "error"}}
	assert.True(t, errs.HasErrors())
}

// TestIsEnabled 测试模块启用检查
func TestIsEnabled(t *testing.T) {
	cfg := &MultiUserConfig{Enabled: false}
	assert.False(t, cfg.IsEnabled())

	cfg.Enabled = true
	assert.True(t, cfg.IsEnabled())
}
