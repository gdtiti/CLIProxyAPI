package models

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestUserGroup_TableName(t *testing.T) {
	group := UserGroup{}
	assert.Equal(t, "user_groups", group.TableName())
}

func TestUserGroup_HasParent(t *testing.T) {
	parentID := "parent-123"
	emptyParentID := ""

	tests := []struct {
		name     string
		parentID *string
		expected bool
	}{
		{"有父组", &parentID, true},
		{"无父组 - nil", nil, false},
		{"无父组 - 空字符串", &emptyParentID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := UserGroup{ParentID: tt.parentID}
			assert.Equal(t, tt.expected, group.HasParent())
		})
	}
}

func TestUserGroup_IsRoot(t *testing.T) {
	parentID := "parent-123"

	tests := []struct {
		name     string
		parentID *string
		expected bool
	}{
		{"根组 - nil", nil, true},
		{"非根组", &parentID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := UserGroup{ParentID: tt.parentID}
			assert.Equal(t, tt.expected, group.IsRoot())
		})
	}
}

func TestUserGroup_IsSharedBalance(t *testing.T) {
	tests := []struct {
		name          string
		sharedBalance bool
		expected      bool
	}{
		{"启用共享余额", true, true},
		{"未启用共享余额", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := UserGroup{SharedBalance: tt.sharedBalance}
			assert.Equal(t, tt.expected, group.IsSharedBalance())
		})
	}
}

func TestUserGroup_GetEffectiveRateMultiplier(t *testing.T) {
	tests := []struct {
		name           string
		rateMultiplier decimal.Decimal
		expected       decimal.Decimal
	}{
		{"设置了费率倍数", decimal.NewFromFloat(1.5), decimal.NewFromFloat(1.5)},
		{"未设置费率倍数", decimal.Zero, decimal.NewFromInt(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := UserGroup{RateMultiplier: tt.rateMultiplier}
			assert.True(t, tt.expected.Equal(group.GetEffectiveRateMultiplier()))
		})
	}
}

func TestUserGroup_Metadata(t *testing.T) {
	group := UserGroup{}

	// 测试 SetMetadata
	group.SetMetadata("key1", "value1")
	assert.Equal(t, "value1", group.GetMetadata("key1"))

	// 测试 GetMetadata 不存在的键
	assert.Nil(t, group.GetMetadata("nonexistent"))

	// 测试 nil Metadata
	group2 := UserGroup{}
	assert.Nil(t, group2.GetMetadata("key"))
}

func TestUserGroup_GetUserCount(t *testing.T) {
	group := UserGroup{
		Users: []User{{ID: "1"}, {ID: "2"}, {ID: "3"}},
	}
	assert.Equal(t, 3, group.GetUserCount())

	emptyGroup := UserGroup{}
	assert.Equal(t, 0, emptyGroup.GetUserCount())
}

func TestUserGroup_GetChildCount(t *testing.T) {
	group := UserGroup{
		Children: []UserGroup{{ID: "1"}, {ID: "2"}},
	}
	assert.Equal(t, 2, group.GetChildCount())

	emptyGroup := UserGroup{}
	assert.Equal(t, 0, emptyGroup.GetChildCount())
}

func TestUserGroup_Clone(t *testing.T) {
	group := &UserGroup{
		ID:   "group-123",
		Name: "Test Group",
	}

	clone := group.Clone()
	assert.Equal(t, group.ID, clone.ID)
	assert.Equal(t, group.Name, clone.Name)

	// 修改克隆不影响原始
	clone.Name = "Modified"
	assert.NotEqual(t, group.Name, clone.Name)
}

func TestUserGroup_Clone_Nil(t *testing.T) {
	var group *UserGroup
	clone := group.Clone()
	assert.Nil(t, clone)
}

func TestUserGroup_BalanceOperations(t *testing.T) {
	group := UserGroup{
		BalancePool: decimal.NewFromFloat(100.00),
	}

	// 测试 AddBalance
	group.AddBalance(decimal.NewFromFloat(50.00))
	assert.True(t, decimal.NewFromFloat(150.00).Equal(group.BalancePool))

	// 测试 HasSufficientBalance
	assert.True(t, group.HasSufficientBalance(decimal.NewFromFloat(100.00)))
	assert.False(t, group.HasSufficientBalance(decimal.NewFromFloat(200.00)))

	// 测试 SubtractBalance - 成功
	success := group.SubtractBalance(decimal.NewFromFloat(50.00))
	assert.True(t, success)
	assert.True(t, decimal.NewFromFloat(100.00).Equal(group.BalancePool))

	// 测试 SubtractBalance - 失败（余额不足）
	success = group.SubtractBalance(decimal.NewFromFloat(200.00))
	assert.False(t, success)
	assert.True(t, decimal.NewFromFloat(100.00).Equal(group.BalancePool)) // 余额不变
}
