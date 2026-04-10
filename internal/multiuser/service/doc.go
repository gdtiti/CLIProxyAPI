// Package service 提供多用户模块的业务逻辑层
//
// 本包实现了以下 Service 接口：
//   - UserService: 用户管理服务，包含 CRUD、状态管理、密码管理
//   - GroupService: 用户组管理服务，包含 CRUD 和成员管理
//   - AuthService: 认证服务，包含登录、注册、Token 管理
//
// 业务逻辑层负责：
//   - 数据验证
//   - 业务规则执行
//   - 跨 Repository 操作协调
//   - 错误处理和转换
package service
