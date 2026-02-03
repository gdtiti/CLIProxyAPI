// Package dto 定义多用户模块的数据传输对象
//
// 本包包含以下 DTO 类型：
//
// 请求 DTO：
//   - CreateUserRequest: 创建用户请求
//   - UpdateUserRequest: 更新用户请求
//   - LoginRequest: 登录请求
//   - RegisterRequest: 注册请求
//   - ChangePasswordRequest: 修改密码请求
//   - CreateGroupRequest: 创建用户组请求
//   - UpdateGroupRequest: 更新用户组请求
//   - ListUsersRequest: 用户列表查询请求
//   - ListGroupsRequest: 用户组列表查询请求
//
// 响应 DTO：
//   - LoginResponse: 登录响应
//   - TokenPair: Token 对
//   - UserInfo: 用户信息
//   - ListUsersResponse: 用户列表响应
//   - ListGroupsResponse: 用户组列表响应
//   - ErrorResponse: 错误响应
//   - ValidationErrorDetails: 验证错误详情
//
// 所有请求 DTO 都包含 Gin binding 验证标签
package dto
