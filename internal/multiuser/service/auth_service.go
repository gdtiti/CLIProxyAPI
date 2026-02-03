// Package service 提供多用户模块的业务逻辑层
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/repository"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

// AuthService 认证服务接口
// 验证: 需求 6.1-6.7, 7.1-7.7
type AuthService interface {
	// 本地认证
	Register(ctx context.Context, req *RegisterRequest) (*models.User, error)
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, token string) error
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)

	// Token 管理
	GenerateTokenPair(user *models.User) (*TokenPair, error)
	ValidateToken(token string) (*TokenClaims, error)
	InvalidateToken(token string) error
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
	RegisterIP  string
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string // 可以是用户名或邮箱
	Password string
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	TokenType    string
	User         *models.User
}

// TokenPair Token 对
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// TokenClaims JWT Token 声明
type TokenClaims struct {
	UserID   string          `json:"user_id"`
	Username string          `json:"username"`
	Role     models.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// authService AuthService 的实现
type authService struct {
	userRepo       repository.UserRepository
	config         *config.MultiUserConfig
	tokenBlacklist map[string]time.Time // Token 黑名单
	blacklistMu    sync.RWMutex
}

// NewAuthService 创建新的认证服务
func NewAuthService(userRepo repository.UserRepository, cfg *config.MultiUserConfig) AuthService {
	return &authService{
		userRepo:       userRepo,
		config:         cfg,
		tokenBlacklist: make(map[string]time.Time),
	}
}

// Register 用户注册
func (s *authService) Register(ctx context.Context, req *RegisterRequest) (*models.User, error) {
	// 检查是否允许公开注册
	if !s.config.Registration.AllowPublicRegistration {
		return nil, ErrRegistrationDisabled
	}

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

	// 验证密码长度
	if len(req.Password) < 8 {
		return nil, ErrInvalidPassword
	}

	// 加密密码
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	// 获取默认余额
	defaultBalance, _ := decimal.NewFromString(s.config.Registration.DefaultBalance)

	// 确定用户状态
	status := models.StatusActive
	if s.config.Registration.RequireAdminApproval {
		status = models.StatusPending
	}

	// 创建用户
	user := &models.User{
		ID:             uuid.New().String(),
		Username:       req.Username,
		Email:          req.Email,
		PasswordHash:   string(hash),
		DisplayName:    req.DisplayName,
		Role:           models.UserRole(s.config.Registration.DefaultRole),
		Status:         status,
		Balance:        defaultBalance,
		RegisterSource: models.RegisterSourceLocal,
		RegisterIP:     req.RegisterIP,
	}

	// 设置默认用户组
	if s.config.Registration.DefaultGroupID != "" {
		user.GroupID = &s.config.Registration.DefaultGroupID
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login 用户登录
func (s *authService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// 尝试通过用户名查找用户
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// 尝试通过邮箱查找
			user, err = s.userRepo.GetByEmail(ctx, req.Username)
			if err != nil {
				return nil, ErrInvalidCredentials
			}
		} else {
			return nil, err
		}
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 检查用户状态
	switch user.Status {
	case models.StatusSuspended:
		return nil, ErrUserSuspended
	case models.StatusDisabled:
		return nil, ErrUserDisabled
	case models.StatusPending:
		return nil, ErrUserPending
	}

	// 生成 Token
	tokenPair, err := s.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// 更新最后登录时间
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

// Logout 用户登出
func (s *authService) Logout(ctx context.Context, token string) error {
	return s.InvalidateToken(token)
}

// RefreshToken 刷新 Token
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// 验证 Refresh Token
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return nil, ErrRefreshTokenInvalid
	}

	// 获取用户
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrRefreshTokenInvalid
	}

	// 检查用户状态
	if !user.CanLogin() {
		return nil, ErrUserDisabled
	}

	// 使旧 Token 失效
	_ = s.InvalidateToken(refreshToken)

	// 生成新的 Token 对
	return s.GenerateTokenPair(user)
}

// GenerateTokenPair 生成 Token 对
func (s *authService) GenerateTokenPair(user *models.User) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(s.config.Auth.AccessTokenTTL)
	refreshExpiry := now.Add(s.config.Auth.RefreshTokenTTL)

	// 生成 Access Token
	accessClaims := &TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.Auth.JWTSecret))
	if err != nil {
		return nil, err
	}

	// 生成 Refresh Token
	refreshClaims := &TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.config.Auth.JWTSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(s.config.Auth.AccessTokenTTL.Seconds()),
	}, nil
}

// ValidateToken 验证 Token
func (s *authService) ValidateToken(tokenString string) (*TokenClaims, error) {
	// 检查是否在黑名单中
	s.blacklistMu.RLock()
	if _, exists := s.tokenBlacklist[tokenString]; exists {
		s.blacklistMu.RUnlock()
		return nil, ErrTokenBlacklisted
	}
	s.blacklistMu.RUnlock()

	// 解析 Token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return []byte(s.config.Auth.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// InvalidateToken 使 Token 失效
func (s *authService) InvalidateToken(token string) error {
	// 解析 Token 获取过期时间
	claims, err := s.ValidateToken(token)
	if err != nil && !errors.Is(err, ErrTokenBlacklisted) {
		// 如果 Token 已经无效，直接返回
		return nil
	}

	// 添加到黑名单
	s.blacklistMu.Lock()
	if claims != nil && claims.ExpiresAt != nil {
		s.tokenBlacklist[token] = claims.ExpiresAt.Time
	} else {
		// 如果无法获取过期时间，设置一个默认的过期时间
		s.tokenBlacklist[token] = time.Now().Add(24 * time.Hour)
	}
	s.blacklistMu.Unlock()

	// 清理过期的黑名单条目
	go s.cleanupBlacklist()

	return nil
}

// cleanupBlacklist 清理过期的黑名单条目
func (s *authService) cleanupBlacklist() {
	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	now := time.Now()
	for token, expiry := range s.tokenBlacklist {
		if now.After(expiry) {
			delete(s.tokenBlacklist, token)
		}
	}
}
