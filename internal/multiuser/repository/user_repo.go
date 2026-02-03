// Package repository 提供多用户模块的数据访问层
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"gorm.io/gorm"
)

// UserRepository 用户数据访问接口
// 验证: 需求 5.1, 5.2, 5.3, 5.4, 5.5, 5.6
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts *ListOptions) ([]*models.User, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateStatus(ctx context.Context, id string, status models.UserStatus) error
	UpdateRole(ctx context.Context, id string, role models.UserRole) error
	UpdateLastLogin(ctx context.Context, id string) error
}

// ListOptions 列表查询选项
type ListOptions struct {
	Page     int                    // 页码，从 1 开始
	PageSize int                    // 每页数量
	SortBy   string                 // 排序字段
	SortDesc bool                   // 是否降序
	Filters  map[string]interface{} // 筛选条件
}

// userRepository UserRepository 的 GORM 实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建新的用户仓库
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID 根据 ID 获取用户
func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Update 更新用户
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 删除用户
func (r *userRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// List 分页查询用户列表
func (r *userRepository) List(ctx context.Context, opts *ListOptions) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	query := r.db.WithContext(ctx).Model(&models.User{})

	// 应用筛选条件
	if opts != nil && opts.Filters != nil {
		for key, value := range opts.Filters {
			switch key {
			case "status":
				query = query.Where("status = ?", value)
			case "role":
				query = query.Where("role = ?", value)
			case "group_id":
				query = query.Where("group_id = ?", value)
			case "register_source":
				query = query.Where("register_source = ?", value)
			case "username_like":
				query = query.Where("username LIKE ?", fmt.Sprintf("%%%s%%", value))
			case "email_like":
				query = query.Where("email LIKE ?", fmt.Sprintf("%%%s%%", value))
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
		query = query.Order("created_at DESC")
	}

	// 应用分页
	if opts != nil && opts.Page > 0 && opts.PageSize > 0 {
		offset := (opts.Page - 1) * opts.PageSize
		query = query.Offset(offset).Limit(opts.PageSize)
	}

	// 执行查询
	if err := query.Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateStatus 更新用户状态
func (r *userRepository) UpdateStatus(ctx context.Context, id string, status models.UserStatus) error {
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateRole 更新用户角色
func (r *userRepository) UpdateRole(ctx context.Context, id string, role models.UserRole) error {
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateLastLogin 更新最后登录时间
func (r *userRepository) UpdateLastLogin(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("last_login_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}
