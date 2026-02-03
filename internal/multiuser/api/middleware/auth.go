// Package middleware 提供多用户模块的 HTTP 中间件
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/api/dto"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/models"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/multiuser/service"
)

// Context 键常量
const (
	// ContextKeyUserID 用户 ID 在 Context 中的键
	ContextKeyUserID = "user_id"
	// ContextKeyUsername 用户名在 Context 中的键
	ContextKeyUsername = "username"
	// ContextKeyRole 用户角色在 Context 中的键
	ContextKeyRole = "role"
)

// JWTAuthMiddleware JWT 认证中间件
// 从 Authorization header 提取 Bearer Token 并验证
// 验证成功后将用户信息注入 Gin Context
// 验证: 需求 13.4, 13.5
func JWTAuthMiddleware(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求中提取 Token
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(401, dto.ErrorResponse{
				Code:    dto.ErrCodeAuthTokenMissing,
				Message: dto.ErrMsgAuthTokenMissing,
			})
			return
		}

		// 验证 Token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, dto.ErrorResponse{
				Code:    dto.ErrCodeAuthTokenInvalid,
				Message: dto.ErrMsgAuthTokenInvalid,
			})
			return
		}

		// 将用户信息注入 Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)

		c.Next()
	}
}

// AdminAuthMiddleware 管理员权限中间件
// 验证用户是否具有管理员角色
// 必须在 JWTAuthMiddleware 之后使用
// 验证: 需求 12.7, 12.8
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			c.AbortWithStatusJSON(401, dto.ErrorResponse{
				Code:    dto.ErrCodeAuthTokenMissing,
				Message: dto.ErrMsgAuthTokenMissing,
			})
			return
		}

		// 检查是否为管理员角色
		userRole, ok := role.(models.UserRole)
		if !ok || userRole != models.RoleAdmin {
			c.AbortWithStatusJSON(403, dto.ErrorResponse{
				Code:    dto.ErrCodePermissionDenied,
				Message: dto.ErrMsgPermissionDenied,
			})
			return
		}

		c.Next()
	}
}

// extractToken 从请求中提取 Bearer Token
// 支持从 Authorization header 提取
func extractToken(c *gin.Context) string {
	// 从 Authorization header 提取 Bearer token
	auth := c.GetHeader("Authorization")
	if auth != "" && strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// GetUserIDFromContext 从 Context 中获取用户 ID
// 辅助函数，用于 Handler 中获取当前登录用户 ID
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return "", false
	}
	id, ok := userID.(string)
	return id, ok
}

// GetUsernameFromContext 从 Context 中获取用户名
// 辅助函数，用于 Handler 中获取当前登录用户名
func GetUsernameFromContext(c *gin.Context) (string, bool) {
	username, exists := c.Get(ContextKeyUsername)
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}

// GetRoleFromContext 从 Context 中获取用户角色
// 辅助函数，用于 Handler 中获取当前登录用户角色
func GetRoleFromContext(c *gin.Context) (models.UserRole, bool) {
	role, exists := c.Get(ContextKeyRole)
	if !exists {
		return "", false
	}
	r, ok := role.(models.UserRole)
	return r, ok
}

// IsAdmin 检查当前用户是否为管理员
// 辅助函数，用于 Handler 中快速检查权限
func IsAdmin(c *gin.Context) bool {
	role, ok := GetRoleFromContext(c)
	return ok && role == models.RoleAdmin
}

// IsOperatorOrAdmin 检查当前用户是否为操作员或管理员
// 辅助函数，用于 Handler 中快速检查权限
func IsOperatorOrAdmin(c *gin.Context) bool {
	role, ok := GetRoleFromContext(c)
	return ok && (role == models.RoleAdmin || role == models.RoleOperator)
}
