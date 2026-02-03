// Package service 提供多用户模块的业务逻辑层
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/repository"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

// UserService 用户服务接口
// 验证: 需求 5.1-5.7
type UserService interface {
	// 用户 CRUD
	CreateUser(ctx context.Context, req *CreateUserRequest) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, id string, req *UpdateUserRequest) (*models.User, error)
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error)

	// 状态管理
	SetUserStatus(ctx context.Context, id string, status models.UserStatus) error
	SetUserRole(ctx context.Context, id string, role models.UserRole) error
	SetUserBalance(ctx context.Context, id string, balance decimal.Decimal) error

	// 密码管理
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	SetPassword(ctx context.Context, userID string, password string) error
	VerifyPassword(ctx context.Context, userID string, password string) (bool, error)

	// 验证
	CheckUsernameAvailable(ctx context.Context, username string) (bool, error)
	CheckEmailAvailable(ctx context.Context, email string) (bool, error)
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username       string
	Email          string
	Password       string
	DisplayName    string
	Role           models.UserRole
	Status         models.UserStatus
	GroupID        string
	Balance        decimal.Decimal
	RegisterSource models.RegisterSource
	RegisterIP     string
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email       *string
	DisplayName *string
	AvatarURL   *string
	GroupID     *string
}

// ListUsersRequest 用户列表请求
type ListUsersRequest struct {
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
	Status   string
	Role     string
	GroupID  string
	Search   string
}

// ListUsersResponse 用户列表响应
type ListUsersResponse struct {
	Total    int64
	Page     int
	PageSize int
	Items    []*models.User
}

// userService UserService 的实现
type userService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建新的用户服务
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

// CreateUser 创建用户
func (s *userService) CreateUser(ctx context.Context, req *CreateUserRequest) (*models.User, error) {
	// 检查用户名是否已存在
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// 检查邮箱是否已存在
	if req.Email != "" {
		exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
	}

	// 加密密码
	var passwordHash string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			return nil, err
		}
		passwordHash = string(hash)
	}

	// 设置默认值
	role := req.Role
	if role == "" {
		role = models.RoleUser
	}
	status := req.Status
	if status == "" {
		status = models.StatusActive
	}
	registerSource := req.RegisterSource
	if registerSource == "" {
		registerSource = models.RegisterSourceLocal
	}

	// 创建用户
	user := &models.User{
		ID:             uuid.New().String(),
		Username:       req.Username,
		Email:          req.Email,
		PasswordHash:   passwordHash,
		DisplayName:    req.DisplayName,
		Role:           role,
		Status:         status,
		Balance:        req.Balance,
		RegisterSource: registerSource,
		RegisterIP:     req.RegisterIP,
	}

	if req.GroupID != "" {
		user.GroupID = &req.GroupID
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUser 获取用户
func (s *userService) GetUser(ctx context.Context, id string) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// GetUserByUsername 根据用户名获取用户
func (s *userService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// GetUserByEmail 根据邮箱获取用户
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateUser 更新用户
func (s *userService) UpdateUser(ctx context.Context, id string, req *UpdateUserRequest) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// 如果更新邮箱，检查是否已存在
	if req.Email != nil && *req.Email != user.Email {
		exists, err := s.userRepo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
		user.Email = *req.Email
	}

	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.GroupID != nil {
		if *req.GroupID == "" {
			user.GroupID = nil
		} else {
			user.GroupID = req.GroupID
		}
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser 删除用户
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	err := s.userRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// ListUsers 获取用户列表
func (s *userService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	opts := &repository.ListOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
		Filters:  make(map[string]interface{}),
	}

	if req.Status != "" {
		opts.Filters["status"] = req.Status
	}
	if req.Role != "" {
		opts.Filters["role"] = req.Role
	}
	if req.GroupID != "" {
		opts.Filters["group_id"] = req.GroupID
	}
	if req.Search != "" {
		opts.Filters["username_like"] = req.Search
	}

	users, total, err := s.userRepo.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &ListUsersResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    users,
	}, nil
}

// SetUserStatus 设置用户状态
func (s *userService) SetUserStatus(ctx context.Context, id string, status models.UserStatus) error {
	err := s.userRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// SetUserRole 设置用户角色
func (s *userService) SetUserRole(ctx context.Context, id string, role models.UserRole) error {
	err := s.userRepo.UpdateRole(ctx, id, role)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// SetUserBalance 设置用户余额
func (s *userService) SetUserBalance(ctx context.Context, id string, balance decimal.Decimal) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	user.Balance = balance
	return s.userRepo.Update(ctx, user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrWrongPassword
	}

	// 验证新密码长度
	if len(newPassword) < 8 {
		return ErrInvalidPassword
	}

	// 加密新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	return s.userRepo.Update(ctx, user)
}

// SetPassword 设置密码（不验证旧密码）
func (s *userService) SetPassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 验证密码长度
	if len(password) < 8 {
		return ErrInvalidPassword
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	return s.userRepo.Update(ctx, user)
}

// VerifyPassword 验证密码
func (s *userService) VerifyPassword(ctx context.Context, userID string, password string) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return false, ErrUserNotFound
		}
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil, nil
}

// CheckUsernameAvailable 检查用户名是否可用
func (s *userService) CheckUsernameAvailable(ctx context.Context, username string) (bool, error) {
	exists, err := s.userRepo.ExistsByUsername(ctx, username)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckEmailAvailable 检查邮箱是否可用
func (s *userService) CheckEmailAvailable(ctx context.Context, email string) (bool, error) {
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
