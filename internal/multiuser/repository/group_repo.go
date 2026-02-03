// Package repository 提供多用户模块的数据访问层
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"gorm.io/gorm"
)

// GroupRepository 用户组数据访问接口
// 验证: 需求 10.1, 10.2, 10.3, 10.4, 10.5
type GroupRepository interface {
	Create(ctx context.Context, group *models.UserGroup) error
	GetByID(ctx context.Context, id string) (*models.UserGroup, error)
	GetByName(ctx context.Context, name string) (*models.UserGroup, error)
	Update(ctx context.Context, group *models.UserGroup) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts *ListOptions) ([]*models.UserGroup, int64, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	GetMembers(ctx context.Context, groupID string) ([]*models.User, error)
	GetChildren(ctx context.Context, parentID string) ([]*models.UserGroup, error)
	HasMembers(ctx context.Context, groupID string) (bool, error)
	HasChildren(ctx context.Context, groupID string) (bool, error)
}

// groupRepository GroupRepository 的 GORM 实现
type groupRepository struct {
	db *gorm.DB
}

// NewGroupRepository 创建新的用户组仓库
func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{db: db}
}

// Create 创建用户组
func (r *groupRepository) Create(ctx context.Context, group *models.UserGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

// GetByID 根据 ID 获取用户组
func (r *groupRepository) GetByID(ctx context.Context, id string) (*models.UserGroup, error) {
	var group models.UserGroup
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&group).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

// GetByName 根据名称获取用户组
func (r *groupRepository) GetByName(ctx context.Context, name string) (*models.UserGroup, error) {
	var group models.UserGroup
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&group).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

// Update 更新用户组
func (r *groupRepository) Update(ctx context.Context, group *models.UserGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

// Delete 删除用户组
func (r *groupRepository) Delete(ctx context.Context, id string) error {
	// 检查是否有成员
	hasMember, err := r.HasMembers(ctx, id)
	if err != nil {
		return err
	}
	if hasMember {
		return ErrGroupHasMembers
	}

	// 检查是否有子组
	hasChild, err := r.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if hasChild {
		return ErrGroupHasChildren
	}

	result := r.db.WithContext(ctx).Delete(&models.UserGroup{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// List 分页查询用户组列表
func (r *groupRepository) List(ctx context.Context, opts *ListOptions) ([]*models.UserGroup, int64, error) {
	var groups []*models.UserGroup
	var total int64

	query := r.db.WithContext(ctx).Model(&models.UserGroup{})

	// 应用筛选条件
	if opts != nil && opts.Filters != nil {
		for key, value := range opts.Filters {
			switch key {
			case "parent_id":
				if value == nil || value == "" {
					query = query.Where("parent_id IS NULL")
				} else {
					query = query.Where("parent_id = ?", value)
				}
			case "shared_balance":
				query = query.Where("shared_balance = ?", value)
			case "name_like":
				query = query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", value))
			}
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 应用排序
	if opts != nil && opts.SortBy != "" {
		order := opts.SortBy
		if opts.SortDesc {
			order += " DESC"
		}
		query = query.Order(order)
	} else {
		query = query.Order("priority DESC, created_at DESC")
	}

	// 应用分页
	if opts != nil && opts.Page > 0 && opts.PageSize > 0 {
		offset := (opts.Page - 1) * opts.PageSize
		query = query.Offset(offset).Limit(opts.PageSize)
	}

	// 执行查询
	if err := query.Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

// ExistsByName 检查用户组名是否存在
func (r *groupRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.UserGroup{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMembers 获取用户组成员
func (r *groupRepository) GetMembers(ctx context.Context, groupID string) ([]*models.User, error) {
	var users []*models.User
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GetChildren 获取子用户组
func (r *groupRepository) GetChildren(ctx context.Context, parentID string) ([]*models.UserGroup, error) {
	var groups []*models.UserGroup
	err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// HasMembers 检查用户组是否有成员
func (r *groupRepository) HasMembers(ctx context.Context, groupID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("group_id = ?", groupID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasChildren 检查用户组是否有子组
func (r *groupRepository) HasChildren(ctx context.Context, groupID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.UserGroup{}).Where("parent_id = ?", groupID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
