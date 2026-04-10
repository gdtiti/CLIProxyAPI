// Package service 提供多用户模块的业务逻辑层
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/repository"
	"github.com/shopspring/decimal"
)

// GroupService 用户组服务接口
// 验证: 需求 10.1-10.6, 11.1-11.5
type GroupService interface {
	// 用户组 CRUD
	CreateGroup(ctx context.Context, req *CreateGroupRequest) (*models.UserGroup, error)
	GetGroup(ctx context.Context, id string) (*models.UserGroup, error)
	UpdateGroup(ctx context.Context, id string, req *UpdateGroupRequest) (*models.UserGroup, error)
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context, req *ListGroupsRequest) (*ListGroupsResponse, error)

	// 成员管理
	AddUserToGroup(ctx context.Context, groupID, userID string) error
	RemoveUserFromGroup(ctx context.Context, groupID, userID string) error
	ListGroupMembers(ctx context.Context, groupID string) ([]*models.User, error)

	// 验证
	CheckGroupNameAvailable(ctx context.Context, name string) (bool, error)
}

// CreateGroupRequest 创建用户组请求
type CreateGroupRequest struct {
	Name           string
	Description    string
	ParentID       string
	BalancePool    decimal.Decimal
	SharedBalance  bool
	RateMultiplier decimal.Decimal
	Priority       int
}

// UpdateGroupRequest 更新用户组请求
type UpdateGroupRequest struct {
	Name           *string
	Description    *string
	BalancePool    *decimal.Decimal
	SharedBalance  *bool
	RateMultiplier *decimal.Decimal
	Priority       *int
}

// ListGroupsRequest 用户组列表请求
type ListGroupsRequest struct {
	Page          int
	PageSize      int
	SortBy        string
	SortDesc      bool
	ParentID      *string // nil 表示不筛选，空字符串表示只查根组
	SharedBalance *bool
	Search        string
}

// ListGroupsResponse 用户组列表响应
type ListGroupsResponse struct {
	Total    int64
	Page     int
	PageSize int
	Items    []*models.UserGroup
}

// groupService GroupService 的实现
type groupService struct {
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
}

// NewGroupService 创建新的用户组服务
func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository) GroupService {
	return &groupService{
		groupRepo: groupRepo,
		userRepo:  userRepo,
	}
}

// CreateGroup 创建用户组
func (s *groupService) CreateGroup(ctx context.Context, req *CreateGroupRequest) (*models.UserGroup, error) {
	// 检查名称是否已存在
	exists, err := s.groupRepo.ExistsByName(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrGroupAlreadyExists
	}

	// 如果指定了父组，检查父组是否存在
	if req.ParentID != "" {
		_, err := s.groupRepo.GetByID(ctx, req.ParentID)
		if err != nil {
			if errors.Is(err, repository.ErrGroupNotFound) {
				return nil, ErrGroupNotFound
			}
			return nil, err
		}
	}

	// 设置默认费率倍数
	rateMultiplier := req.RateMultiplier
	if rateMultiplier.IsZero() {
		rateMultiplier = decimal.NewFromInt(1)
	}

	// 创建用户组
	group := &models.UserGroup{
		ID:             uuid.New().String(),
		Name:           req.Name,
		Description:    req.Description,
		BalancePool:    req.BalancePool,
		SharedBalance:  req.SharedBalance,
		RateMultiplier: rateMultiplier,
		Priority:       req.Priority,
	}

	if req.ParentID != "" {
		group.ParentID = &req.ParentID
	}

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

// GetGroup 获取用户组
func (s *groupService) GetGroup(ctx context.Context, id string) (*models.UserGroup, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return group, nil
}

// UpdateGroup 更新用户组
func (s *groupService) UpdateGroup(ctx context.Context, id string, req *UpdateGroupRequest) (*models.UserGroup, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}

	// 如果更新名称，检查是否已存在
	if req.Name != nil && *req.Name != group.Name {
		exists, err := s.groupRepo.ExistsByName(ctx, *req.Name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrGroupAlreadyExists
		}
		group.Name = *req.Name
	}

	if req.Description != nil {
		group.Description = *req.Description
	}
	if req.BalancePool != nil {
		group.BalancePool = *req.BalancePool
	}
	if req.SharedBalance != nil {
		group.SharedBalance = *req.SharedBalance
	}
	if req.RateMultiplier != nil {
		group.RateMultiplier = *req.RateMultiplier
	}
	if req.Priority != nil {
		group.Priority = *req.Priority
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

// DeleteGroup 删除用户组
func (s *groupService) DeleteGroup(ctx context.Context, id string) error {
	err := s.groupRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return ErrGroupNotFound
		}
		if errors.Is(err, repository.ErrGroupHasMembers) {
			return ErrGroupHasMembers
		}
		if errors.Is(err, repository.ErrGroupHasChildren) {
			return ErrGroupHasChildren
		}
		return err
	}
	return nil
}

// ListGroups 获取用户组列表
func (s *groupService) ListGroups(ctx context.Context, req *ListGroupsRequest) (*ListGroupsResponse, error) {
	opts := &repository.ListOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
		Filters:  make(map[string]interface{}),
	}

	if req.ParentID != nil {
		opts.Filters["parent_id"] = *req.ParentID
	}
	if req.SharedBalance != nil {
		opts.Filters["shared_balance"] = *req.SharedBalance
	}
	if req.Search != "" {
		opts.Filters["name_like"] = req.Search
	}

	groups, total, err := s.groupRepo.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &ListGroupsResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    groups,
	}, nil
}

// AddUserToGroup 将用户添加到用户组
func (s *groupService) AddUserToGroup(ctx context.Context, groupID, userID string) error {
	// 检查用户组是否存在
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	// 获取用户
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 更新用户的 GroupID
	user.GroupID = &groupID
	return s.userRepo.Update(ctx, user)
}

// RemoveUserFromGroup 将用户从用户组移除
func (s *groupService) RemoveUserFromGroup(ctx context.Context, groupID, userID string) error {
	// 获取用户
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 检查用户是否在该组
	if user.GroupID == nil || *user.GroupID != groupID {
		return nil // 用户不在该组，直接返回
	}

	// 移除用户的 GroupID
	user.GroupID = nil
	return s.userRepo.Update(ctx, user)
}

// ListGroupMembers 获取用户组成员
func (s *groupService) ListGroupMembers(ctx context.Context, groupID string) ([]*models.User, error) {
	// 检查用户组是否存在
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}

	return s.groupRepo.GetMembers(ctx, groupID)
}

// CheckGroupNameAvailable 检查用户组名是否可用
func (s *groupService) CheckGroupNameAvailable(ctx context.Context, name string) (bool, error) {
	exists, err := s.groupRepo.ExistsByName(ctx, name)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
