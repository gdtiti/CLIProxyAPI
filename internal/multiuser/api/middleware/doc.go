// Package middleware 提供多用户模块的 HTTP 中间件
//
// 本包实现了以下中间件：
//   - JWTAuthMiddleware: JWT 认证中间件，验证 Access Token
//   - AdminAuthMiddleware: 管理员权限中间件，验证用户角色
//
// 中间件功能：
//   - 从 Authorization header 提取 Bearer Token
//   - 验证 Token 有效性
//   - 将用户信息注入 Gin Context
//   - 返回标准化的错误响应
package middleware
