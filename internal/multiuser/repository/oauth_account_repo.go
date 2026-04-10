// Package repository 提供多用户模块的数据访问层
package repository

import (
	"context"
	"errors"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"gorm.io/gorm"
)

// OAuthAccountRepository OAuth 账号数据访问接口
// 验证: 需求 3.3, 3.4
type OAuthAccountRepository interface {
	Create(ctx context.Context, account *models.OAuthAccount) error
	GetByID(ctx context.Context, id string) (*models.OAuthAccount, error)
	GetByProviderAndUserID(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthAccount, error)
	Delete(ctx context.Context, id string) error
	ListByUserID(ctx context.Context, userID string) ([]*models.OAuthAccount, error)
	Update(ctx context.Context, account *models.OAuthAccount) error
	ExistsByProviderAndUserID(ctx context.Context, provider models.OAuthProvider, providerUserID string) (bool, error)
}

// oauthAccountRepository OAuthAccountRepository 的 GORM 实现
type oauthAccountRepository struct {
	db *gorm.DB
}

// NewOAuthAccountRepository 创建新的 OAuth 账号仓库
func NewOAuthAccountRepository(db *gorm.DB) OAuthAccountRepository {
	return &oauthAccountRepository{db: db}
}

// Create 创建 OAuth 账号
func (r *oauthAccountRepository) Create(ctx context.Context, account *models.OAuthAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

// GetByID 根据 ID 获取 OAuth 账号
func (r *oauthAccountRepository) GetByID(ctx context.Context, id string) (*models.OAuthAccount, error) {
	var account models.OAuthAccount
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthAccountNotFound
		}
		return nil, err
	}
	return &account, nil
}

// GetByProviderAndUserID 根据供应商和供应商用户 ID 获取 OAuth 账号
func (r *oauthAccountRepository) GetByProviderAndUserID(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthAccount, error) {
	var account models.OAuthAccount
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthAccountNotFound
		}
		return nil, err
	}
	return &account, nil
}

// Delete 删除 OAuth 账号
func (r *oauthAccountRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.OAuthAccount{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOAuthAccountNotFound
	}
	return nil
}

// ListByUserID 获取用户的所有 OAuth 账号
func (r *oauthAccountRepository) ListByUserID(ctx context.Context, userID string) ([]*models.OAuthAccount, error) {
	var accounts []*models.OAuthAccount
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// Update 更新 OAuth 账号
func (r *oauthAccountRepository) Update(ctx context.Context, account *models.OAuthAccount) error {
	return r.db.WithContext(ctx).Save(account).Error
}

// ExistsByProviderAndUserID 检查 OAuth 账号是否存在
func (r *oauthAccountRepository) ExistsByProviderAndUserID(ctx context.Context, provider models.OAuthProvider, providerUserID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.OAuthAccount{}).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
