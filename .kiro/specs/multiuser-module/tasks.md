# 实现计划：多用户模块

## 概述

本实现计划将多用户模块的设计分解为可执行的编码任务，采用增量开发方式，确保每个步骤都能构建在前一步骤的基础上。

## 任务列表

- [ ] 1. 项目基础架构搭建
  - [x] 1.1 创建多用户模块目录结构
    - 创建 `internal/multiuser/` 及其子目录（models、repository、service、api、config、migration）
    - _需求: 1.1_
  
  - [x] 1.2 添加项目依赖
    - 在 go.mod 中添加 GORM、PostgreSQL 驱动、JWT、decimal、Swagger 等依赖
    - 运行 `go mod tidy` 确保依赖正确安装
    - _需求: 1.1_
  
  - [x] 1.3 实现配置结构
    - 创建 `internal/multiuser/config/config.go`
    - 定义 MultiUserConfig、DatabaseConfig、AuthConfig、RegistrationConfig、AdminConfig 结构体
    - _需求: 1.2, 1.5_

- [ ] 2. 数据模型层实现
  - [x] 2.1 实现通用类型定义
    - 创建 `internal/multiuser/models/types.go`
    - 定义 JSON、StringArray 自定义类型
    - 定义枚举类型：UserRole、UserStatus、RegisterSource、OAuthProvider
    - _需求: 2.2, 2.3, 2.4, 3.2_
  
  - [x] 2.2 实现 User 模型
    - 创建 `internal/multiuser/models/user.go`
    - 定义 User 结构体及所有字段
    - 添加 GORM 标签和 JSON 标签
    - _需求: 2.1, 2.5, 2.6_
  
  - [x] 2.3 实现 UserGroup 模型
    - 创建 `internal/multiuser/models/group.go`
    - 定义 UserGroup 结构体及所有字段
    - 实现父子关系自引用
    - _需求: 4.1, 4.2, 4.3, 4.4, 4.5_
  
  - [x] 2.4 实现 OAuthAccount 模型
    - 创建 `internal/multiuser/models/oauth_account.go`
    - 定义 OAuthAccount 结构体及所有字段
    - _需求: 3.1, 3.3, 3.4_
  
  - [x] 2.5 编写模型单元测试
    - 测试枚举值有效性
    - 测试模型字段定义
    - _需求: 2.1-2.6, 3.1-3.4, 4.1-4.5_

- [ ] 3. 数据库连接和迁移
  - [x] 3.1 实现数据库连接
    - 创建 `internal/multiuser/database.go`
    - 实现 GORM 数据库连接初始化
    - 支持连接池配置
    - _需求: 1.1, 1.2, 1.5_
  
  - [x] 3.2 实现数据库迁移
    - 创建 `internal/multiuser/migration/migrations.go`
    - 实现 AutoMigrate 函数
    - 实现索引创建（OAuth 唯一约束）
    - 实现默认管理员创建
    - _需求: 1.3, 1.4_
  
  - [x] 3.3 编写数据库连接测试
    - 测试连接成功场景
    - 测试连接失败错误处理
    - _需求: 1.4_

- [x] 4. 检查点 - 确保数据库层正常工作
  - 确保所有测试通过，如有问题请询问用户

- [ ] 5. Repository 层实现
  - [x] 5.1 实现 UserRepository
    - 创建 `internal/multiuser/repository/user_repo.go`
    - 实现 Create、GetByID、GetByUsername、GetByEmail、Update、Delete、List 方法
    - 实现 ExistsByUsername、ExistsByEmail、UpdateStatus、UpdateRole、UpdateLastLogin 方法
    - _需求: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_
  
  - [x] 5.2 实现 GroupRepository
    - 创建 `internal/multiuser/repository/group_repo.go`
    - 实现 Create、GetByID、GetByName、Update、Delete、List 方法
    - 实现 ExistsByName、GetMembers、GetChildren 方法
    - _需求: 10.1, 10.2, 10.3, 10.4, 10.5_
  
  - [x] 5.3 实现 OAuthAccountRepository
    - 创建 `internal/multiuser/repository/oauth_account_repo.go`
    - 实现 Create、GetByID、GetByProviderAndUserID、Delete、ListByUserID 方法
    - _需求: 3.3, 3.4_
  
  - [~] 5.4 编写 Repository 属性测试
    - **Property 1: 用户名和邮箱唯一性约束**
    - **Property 3: 用户查询一致性**
    - **Property 4: 分页查询正确性**
    - **验证: 需求 5.1, 5.3, 5.4**

- [ ] 6. Service 层实现 - 用户服务
  - [x] 6.1 实现 UserService 接口和结构体
    - 创建 `internal/multiuser/service/user_service.go`
    - 定义 UserService 接口
    - 实现 userServiceImpl 结构体
    - _需求: 5.1-5.7_
  
  - [x] 6.2 实现用户 CRUD 方法
    - 实现 CreateUser（包含密码加密、唯一性验证）
    - 实现 GetUser、GetUserByUsername、UpdateUser、DeleteUser
    - 实现 ListUsers（分页、筛选、排序）
    - _需求: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7_
  
  - [x] 6.3 实现用户状态和角色管理
    - 实现 SetUserStatus、SetUserRole 方法
    - _需求: 9.1, 9.2_
  
  - [x] 6.4 实现密码管理方法
    - 实现 ChangePassword、SetPassword 方法
    - 使用 bcrypt 加密
    - _需求: 8.1, 8.2, 8.4, 8.5_
  
  - [x] 6.5 实现用户名邮箱可用性检查
    - 实现 CheckUsernameAvailable、CheckEmailAvailable 方法
    - _需求: 5.1, 5.7_
  
  - [~] 6.6 编写 UserService 属性测试
    - **Property 2: 密码加密存储**
    - **Property 5: 更新时间记录**
    - **验证: 需求 5.2, 5.5, 8.5**

- [ ] 7. Service 层实现 - 用户组服务
  - [x] 7.1 实现 GroupService 接口和结构体
    - 创建 `internal/multiuser/service/group_service.go`
    - 定义 GroupService 接口
    - 实现 groupServiceImpl 结构体
    - _需求: 10.1-10.6, 11.1-11.5_
  
  - [x] 7.2 实现用户组 CRUD 方法
    - 实现 CreateGroup（包含唯一性验证）
    - 实现 GetGroup、UpdateGroup、DeleteGroup、ListGroups
    - _需求: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_
  
  - [x] 7.3 实现成员管理方法
    - 实现 AddUserToGroup、RemoveUserFromGroup、ListGroupMembers
    - _需求: 11.1, 11.2, 11.3, 11.4, 11.5_
  
  - [~] 7.4 编写 GroupService 属性测试
    - **Property 15: 用户组层级结构完整性**
    - **Property 16: 用户组成员管理一致性**
    - **验证: 需求 4.2, 10.2, 11.1-11.4**

- [ ] 8. Service 层实现 - 认证服务
  - [x] 8.1 实现 AuthService 接口和结构体
    - 创建 `internal/multiuser/service/auth_service.go`
    - 定义 AuthService 接口和 TokenClaims 结构体
    - 实现 authServiceImpl 结构体
    - _需求: 6.1-6.7, 7.1-7.7_
  
  - [x] 8.2 实现 JWT Token 管理
    - 实现 GenerateTokenPair 方法
    - 实现 ValidateToken 方法
    - 实现 InvalidateToken 方法（Token 黑名单）
    - _需求: 6.2, 6.3, 6.4, 6.5_
  
  - [x] 8.3 实现登录方法
    - 实现 Login 方法（凭据验证、状态检查、Token 生成）
    - _需求: 6.1, 6.6, 6.7_
  
  - [x] 8.4 实现注册方法
    - 实现 Register 方法（配置检查、输入验证、默认值分配）
    - _需求: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7_
  
  - [x] 8.5 实现登出和 Token 刷新
    - 实现 Logout 方法
    - 实现 RefreshToken 方法
    - _需求: 6.4, 6.5_
  
  - [~] 8.6 编写 AuthService 属性测试
    - **Property 6: 登录凭据验证**
    - **Property 7: JWT Token 往返一致性**
    - **Property 8: Token 刷新有效性**
    - **Property 9: 登出后 Token 失效**
    - **Property 10: 用户状态影响登录**
    - **Property 11: 注册默认值分配**
    - **验证: 需求 6.1-6.6, 7.3, 7.6**

- [x] 9. 检查点 - 确保 Service 层正常工作
  - 确保所有测试通过，如有问题请询问用户

- [ ] 10. DTO 层实现
  - [x] 10.1 实现请求 DTO
    - 创建 `internal/multiuser/api/dto/request.go`
    - 定义 CreateUserRequest、UpdateUserRequest、LoginRequest、RegisterRequest
    - 定义 ChangePasswordRequest、CreateGroupRequest、UpdateGroupRequest
    - 定义 ListUsersRequest、ListGroupsRequest
    - 添加 binding 验证标签
    - _需求: 17.1, 17.2, 17.3, 17.4_
  
  - [x] 10.2 实现响应 DTO
    - 创建 `internal/multiuser/api/dto/response.go`
    - 定义 LoginResponse、TokenPair、UserInfo
    - 定义 ListUsersResponse、ListGroupsResponse
    - 定义 ErrorResponse、ValidationErrorDetails
    - _需求: 16.1, 16.2, 16.3, 16.4_
  
  - [~] 10.3 编写 DTO 属性测试
    - **Property 12: 输入验证规则**
    - **Property 18: API 响应格式一致性**
    - **验证: 需求 17.1-17.5, 16.1-16.3**

- [ ] 11. 中间件实现
  - [x] 11.1 实现 JWT 认证中间件
    - 创建 `internal/multiuser/api/middleware/auth.go`
    - 实现 JWTAuthMiddleware 函数
    - 实现 extractToken 辅助函数
    - _需求: 13.4, 13.5_
  
  - [~] 11.2 实现管理员权限中间件
    - 实现 AdminAuthMiddleware 函数
    - _需求: 12.7, 12.8_
  
  - [~] 11.3 编写中间件单元测试
    - 测试 Token 缺失场景
    - 测试 Token 无效场景
    - 测试权限不足场景
    - _需求: 12.8, 13.5_

- [ ] 12. API Handler 实现 - 认证接口
  - [~] 12.1 实现 AuthHandler 结构体
    - 创建 `internal/multiuser/api/handlers/auth_handler.go`
    - 定义 AuthHandler 结构体和构造函数
    - _需求: 14.1-14.7_
  
  - [~] 12.2 实现登录接口
    - 实现 Login handler（POST /v0/auth/login）
    - 添加 Swagger 注解
    - _需求: 14.1_
  
  - [~] 12.3 实现登出接口
    - 实现 Logout handler（POST /v0/auth/logout）
    - 添加 Swagger 注解
    - _需求: 14.2_
  
  - [~] 12.4 实现注册接口
    - 实现 Register handler（POST /v0/auth/register）
    - 添加 Swagger 注解
    - _需求: 14.3_
  
  - [~] 12.5 实现 Token 刷新接口
    - 实现 RefreshToken handler（POST /v0/auth/refresh）
    - 添加 Swagger 注解
    - _需求: 14.4_
  
  - [~] 12.6 实现可用性检查接口
    - 实现 CheckUsername handler（GET /v0/auth/check-username）
    - 实现 CheckEmail handler（GET /v0/auth/check-email）
    - 添加 Swagger 注解
    - _需求: 14.5, 14.6_
  
  - [~] 12.7 编写认证接口集成测试
    - 测试登录成功和失败场景
    - 测试注册成功和失败场景
    - 测试 Token 刷新
    - _需求: 14.1-14.7_

- [ ] 13. API Handler 实现 - 用户接口
  - [~] 13.1 实现 UserHandler 结构体
    - 创建 `internal/multiuser/api/handlers/user_handler.go`
    - 定义 UserHandler 结构体和构造函数
    - _需求: 13.1-13.5_
  
  - [~] 13.2 实现获取个人信息接口
    - 实现 GetProfile handler（GET /v0/user/profile）
    - 添加 Swagger 注解
    - _需求: 13.1_
  
  - [~] 13.3 实现更新个人信息接口
    - 实现 UpdateProfile handler（PUT /v0/user/profile）
    - 添加 Swagger 注解
    - _需求: 13.2_
  
  - [~] 13.4 实现修改密码接口
    - 实现 ChangePassword handler（PUT /v0/user/password）
    - 添加 Swagger 注解
    - _需求: 13.3_
  
  - [~] 13.5 编写用户接口集成测试
    - 测试获取和更新个人信息
    - 测试修改密码
    - _需求: 13.1-13.5_

- [ ] 14. API Handler 实现 - 管理员接口
  - [~] 14.1 实现 AdminHandler 结构体
    - 创建 `internal/multiuser/api/handlers/admin_handler.go`
    - 定义 AdminHandler 结构体和构造函数
    - _需求: 12.1-12.8_
  
  - [~] 14.2 实现用户管理接口
    - 实现 CreateUser handler（POST /v0/admin/users）
    - 实现 ListUsers handler（GET /v0/admin/users）
    - 实现 GetUser handler（GET /v0/admin/users/:id）
    - 实现 UpdateUser handler（PUT /v0/admin/users/:id）
    - 实现 DeleteUser handler（DELETE /v0/admin/users/:id）
    - 添加 Swagger 注解
    - _需求: 12.1_
  
  - [~] 14.3 实现用户状态和角色管理接口
    - 实现 SetUserStatus handler（PUT /v0/admin/users/:id/status）
    - 实现 SetUserRole handler（PUT /v0/admin/users/:id/role）
    - 实现 SetUserBalance handler（PUT /v0/admin/users/:id/balance）
    - 添加 Swagger 注解
    - _需求: 12.2, 12.3, 12.4_
  
  - [~] 14.4 实现用户组管理接口
    - 实现 CreateGroup handler（POST /v0/admin/groups）
    - 实现 ListGroups handler（GET /v0/admin/groups）
    - 实现 GetGroup handler（GET /v0/admin/groups/:id）
    - 实现 UpdateGroup handler（PUT /v0/admin/groups/:id）
    - 实现 DeleteGroup handler（DELETE /v0/admin/groups/:id）
    - 添加 Swagger 注解
    - _需求: 12.5_
  
  - [~] 14.5 实现用户组成员管理接口
    - 实现 AddGroupMember handler（POST /v0/admin/groups/:id/members）
    - 实现 RemoveGroupMember handler（DELETE /v0/admin/groups/:id/members/:user_id）
    - 添加 Swagger 注解
    - _需求: 12.6_
  
  - [~] 14.6 编写管理员接口集成测试
    - 测试用户 CRUD 操作
    - 测试用户组 CRUD 操作
    - 测试成员管理操作
    - 测试权限验证
    - _需求: 12.1-12.8_

- [ ] 15. 检查点 - 确保 API 层正常工作
  - 确保所有测试通过，如有问题请询问用户

- [ ] 16. 路由和 Swagger 集成
  - [~] 16.1 实现路由配置
    - 创建 `internal/multiuser/api/routes.go`
    - 实现 SetupRoutes 函数
    - 配置认证、用户、管理员路由组
    - 应用中间件
    - _需求: 12.1-12.8, 13.1-13.5, 14.1-14.7_
  
  - [~] 16.2 配置 Swagger
    - 创建 `internal/multiuser/api/swagger.go`
    - 添加 Swagger 全局配置注解
    - 配置 Swagger UI 路由
    - _需求: 15.1, 15.4_
  
  - [~] 16.3 生成 Swagger 文档
    - 运行 `swag init` 生成文档
    - 验证文档完整性
    - _需求: 15.2, 15.3, 15.5, 15.6_
  
  - [~] 16.4 编写 Swagger 文档验证测试
    - 验证所有接口都有文档
    - 验证请求响应示例完整
    - _需求: 15.1-15.6_

- [ ] 17. 模块初始化和集成
  - [~] 17.1 实现模块初始化
    - 创建 `internal/multiuser/multiuser.go`
    - 实现 Initialize 函数（数据库连接、迁移、服务初始化）
    - 实现 Shutdown 函数
    - _需求: 1.1, 1.3_
  
  - [~] 17.2 集成到主程序
    - 修改 `cmd/server/main.go` 或相关入口文件
    - 根据配置启用多用户模块
    - 注册路由到主 Gin 引擎
    - _需求: 1.2_
  
  - [~] 17.3 编写集成测试
    - 测试模块初始化
    - 测试完整的用户流程（注册→登录→操作→登出）
    - _需求: 1.1-1.5_

- [ ] 18. 输入验证和安全加固
  - [~] 18.1 实现输入验证器
    - 创建 `internal/multiuser/validator/validator.go`
    - 实现用户名、邮箱、密码验证函数
    - 实现 XSS 防护函数
    - _需求: 17.1, 17.2, 17.3, 17.4, 17.6_
  
  - [~] 18.2 编写验证器属性测试
    - **Property 12: 输入验证规则**
    - **Property 19: XSS 防护**
    - **验证: 需求 17.1-17.6**

- [ ] 19. 日志记录实现
  - [~] 19.1 实现操作日志
    - 在关键操作中添加日志记录
    - 记录用户认证操作
    - 记录管理员操作
    - 记录错误和异常
    - _需求: 18.1, 18.2, 18.3, 18.4, 18.5_

- [ ] 20. 最终检查点 - 确保所有功能正常工作
  - 运行所有测试
  - 验证 Swagger 文档可访问
  - 验证 API 接口正常工作
  - 如有问题请询问用户

## 注意事项

- 每个任务都引用了具体的需求编号，确保可追溯性
- 属性测试验证通用正确性属性，单元测试验证具体示例和边缘情况
- 检查点任务用于确保增量开发的稳定性
