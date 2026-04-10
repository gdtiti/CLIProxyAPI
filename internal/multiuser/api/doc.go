// Package api 提供多用户模块的 HTTP API 层
//
// 本包包含以下子包：
//   - handlers: HTTP 请求处理器
//   - middleware: 中间件（JWT 认证、权限验证）
//   - dto: 数据传输对象（请求和响应结构）
//
// API 路由组织：
//   - /v0/auth/*: 认证接口（公开）
//   - /v0/user/*: 用户接口（需要登录）
//   - /v0/admin/*: 管理员接口（需要管理员权限）
//
// 所有接口都提供 Swagger 文档注解
package api
