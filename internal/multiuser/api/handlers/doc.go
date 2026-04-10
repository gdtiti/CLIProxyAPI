// Package handlers 提供多用户模块的 HTTP 请求处理器
//
// 本包实现了以下 Handler：
//   - AuthHandler: 认证接口处理器（登录、登出、注册、Token 刷新）
//   - UserHandler: 用户接口处理器（个人信息管理、密码修改）
//   - AdminHandler: 管理员接口处理器（用户管理、用户组管理）
//
// 所有 Handler 都：
//   - 使用 Gin 框架
//   - 提供 Swagger 注解
//   - 返回统一的响应格式
package handlers
