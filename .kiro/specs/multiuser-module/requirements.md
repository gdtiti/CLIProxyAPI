# 需求文档

## 简介

本文档定义了 CLI Proxy API 多用户模块的核心需求，包括 ORM 基础架构、用户管理、用户组管理以及相关 API 接口开发。该模块旨在支持多租户场景，实现用户和用户组的精细化管理，并提供完整的 Swagger API 文档。

## 术语表

- **System**: CLI Proxy API 多用户模块系统
- **User**: 系统用户实体，包含身份认证和授权信息
- **UserGroup**: 用户组实体，支持层级结构和共享配置
- **OAuthAccount**: 第三方登录账号实体，关联用户与外部身份提供商
- **JWT_Token**: JSON Web Token，用于用户身份认证
- **GORM**: Go 语言 ORM 框架，用于数据库操作
- **PostgreSQL**: 关系型数据库，用于持久化存储
- **Swagger**: API 文档规范，用于描述 RESTful 接口
- **Repository**: 数据访问层，封装数据库操作
- **Service**: 业务逻辑层，处理核心业务规则
- **Handler**: HTTP 请求处理器，处理 API 请求和响应
- **DTO**: 数据传输对象，用于 API 请求和响应的数据结构

## 需求

### 需求 1：ORM 基础架构

**用户故事：** 作为开发者，我希望系统使用 GORM 作为 ORM 框架连接 PostgreSQL 数据库，以便实现数据的持久化存储和高效查询。

#### 验收标准

1. THE System SHALL 使用 GORM 作为 ORM 框架连接 PostgreSQL 数据库
2. THE System SHALL 支持通过配置文件指定数据库连接参数（DSN、schema、连接池配置）
3. THE System SHALL 提供数据库迁移机制，自动创建和更新数据表结构
4. WHEN 数据库连接失败时，THEN THE System SHALL 记录错误日志并返回明确的错误信息
5. THE System SHALL 支持配置连接池参数（max-open-conns、max-idle-conns、conn-max-lifetime）

### 需求 2：用户数据模型

**用户故事：** 作为系统管理员，我希望系统定义完整的用户数据模型，以便存储和管理用户信息。

#### 验收标准

1. THE User 模型 SHALL 包含以下字段：ID、Username、Email、PasswordHash、DisplayName、AvatarURL、Role、Status、Balance、CreditLimit、GroupID、RegisterSource、RegisterIP、Metadata、CreatedAt、UpdatedAt、LastLoginAt
2. THE System SHALL 支持用户角色枚举：admin（管理员）、operator（操作员）、user（普通用户）
3. THE System SHALL 支持用户状态枚举：active（活跃）、suspended（暂停）、disabled（禁用）、pending（待审核）
4. THE System SHALL 支持注册来源枚举：local（本地注册）、google、wechat、github、invite（邀请注册）、admin（管理员创建）
5. THE User 模型 SHALL 与 UserGroup 建立外键关联
6. THE User 模型 SHALL 与 OAuthAccount 建立一对多关联

### 需求 3：第三方登录账号模型

**用户故事：** 作为用户，我希望能够使用第三方账号（Google、微信、GitHub）登录系统，以便简化登录流程。

#### 验收标准

1. THE OAuthAccount 模型 SHALL 包含以下字段：ID、UserID、Provider、ProviderUserID、Email、DisplayName、AvatarURL、AccessToken、RefreshToken、TokenExpiresAt、Scopes、RawData、CreatedAt、UpdatedAt、LastUsedAt
2. THE System SHALL 支持 OAuth 供应商枚举：google、wechat、github、apple、microsoft
3. THE System SHALL 确保同一供应商的同一用户只能绑定一次（唯一约束）
4. THE OAuthAccount 模型 SHALL 与 User 建立外键关联

### 需求 4：用户组数据模型

**用户故事：** 作为系统管理员，我希望系统支持用户组管理，以便对用户进行分组管理和权限配置。

#### 验收标准

1. THE UserGroup 模型 SHALL 包含以下字段：ID、Name、Description、ParentID、BalancePool、SharedBalance、RateMultiplier、Priority、Metadata、CreatedAt、UpdatedAt
2. THE UserGroup 模型 SHALL 支持层级结构（通过 ParentID 实现父子关系）
3. THE UserGroup 模型 SHALL 与 User 建立一对多关联
4. THE System SHALL 支持组级余额池（BalancePool）和共享余额模式（SharedBalance）
5. THE System SHALL 支持组级费率倍数（RateMultiplier）配置

### 需求 5：用户 CRUD 操作

**用户故事：** 作为系统管理员，我希望能够创建、查询、更新和删除用户，以便管理系统用户。

#### 验收标准

1. WHEN 管理员创建用户时，THE System SHALL 验证用户名和邮箱的唯一性
2. WHEN 管理员创建用户时，THE System SHALL 使用 bcrypt 加密存储密码（cost >= 12）
3. THE System SHALL 支持按 ID、用户名、邮箱查询用户
4. THE System SHALL 支持分页查询用户列表，包含筛选和排序功能
5. WHEN 更新用户信息时，THE System SHALL 验证数据完整性并记录更新时间
6. WHEN 删除用户时，THE System SHALL 处理关联数据（软删除或级联删除）
7. IF 用户名或邮箱已存在，THEN THE System SHALL 返回明确的错误信息

### 需求 6：用户认证

**用户故事：** 作为用户，我希望能够通过用户名/邮箱和密码登录系统，以便访问系统功能。

#### 验收标准

1. WHEN 用户提交登录请求时，THE System SHALL 验证用户名/邮箱和密码
2. WHEN 登录成功时，THE System SHALL 生成 JWT Access Token 和 Refresh Token
3. THE System SHALL 支持配置 Access Token 和 Refresh Token 的过期时间
4. WHEN 用户请求刷新 Token 时，THE System SHALL 验证 Refresh Token 并生成新的 Token 对
5. WHEN 用户登出时，THE System SHALL 使当前 Token 失效
6. IF 用户状态为 suspended 或 disabled，THEN THE System SHALL 拒绝登录并返回相应错误信息
7. IF 登录凭据无效，THEN THE System SHALL 返回统一的错误信息（不泄露具体原因）

### 需求 7：用户注册

**用户故事：** 作为新用户，我希望能够注册账号，以便使用系统服务。

#### 验收标准

1. THE System SHALL 支持通过配置开关控制是否允许公开注册
2. WHEN 用户提交注册请求时，THE System SHALL 验证用户名、邮箱格式和密码强度
3. WHEN 注册成功时，THE System SHALL 根据配置分配默认余额和默认角色
4. THE System SHALL 支持配置是否需要邮箱验证
5. THE System SHALL 支持配置是否需要管理员审核
6. IF 配置需要审核，THEN THE System SHALL 将新用户状态设置为 pending
7. IF 用户名或邮箱已被使用，THEN THE System SHALL 返回明确的错误信息

### 需求 8：密码管理

**用户故事：** 作为用户，我希望能够修改和重置密码，以便保护账号安全。

#### 验收标准

1. WHEN 用户修改密码时，THE System SHALL 验证当前密码正确性
2. WHEN 用户修改密码时，THE System SHALL 验证新密码强度（最少 8 位）
3. THE System SHALL 支持密码重置功能（通过邮箱验证）
4. WHEN 第三方登录用户设置密码时，THE System SHALL 允许设置本地密码
5. THE System SHALL 使用 bcrypt 加密存储所有密码

### 需求 9：用户状态管理

**用户故事：** 作为系统管理员，我希望能够管理用户状态，以便控制用户的系统访问权限。

#### 验收标准

1. THE System SHALL 支持设置用户状态为 active、suspended、disabled、pending
2. WHEN 用户状态变更时，THE System SHALL 记录变更时间
3. WHEN 用户被暂停或禁用时，THE System SHALL 立即使其所有 Token 失效
4. THE System SHALL 支持管理员批量修改用户状态

### 需求 10：用户组 CRUD 操作

**用户故事：** 作为系统管理员，我希望能够创建、查询、更新和删除用户组，以便组织和管理用户。

#### 验收标准

1. WHEN 创建用户组时，THE System SHALL 验证组名的唯一性
2. THE System SHALL 支持创建具有父子关系的层级用户组
3. THE System SHALL 支持查询用户组列表，包含层级结构信息
4. WHEN 更新用户组时，THE System SHALL 验证数据完整性
5. WHEN 删除用户组时，THE System SHALL 处理组内用户（移除关联或阻止删除）
6. IF 用户组名已存在，THEN THE System SHALL 返回明确的错误信息

### 需求 11：用户组成员管理

**用户故事：** 作为系统管理员，我希望能够管理用户组成员，以便将用户分配到相应的组。

#### 验收标准

1. THE System SHALL 支持将用户添加到指定用户组
2. THE System SHALL 支持将用户从用户组中移除
3. THE System SHALL 支持查询用户组的所有成员
4. WHEN 用户被添加到组时，THE System SHALL 更新用户的 GroupID 字段
5. THE System SHALL 支持批量添加或移除用户组成员

### 需求 12：管理员 API 接口

**用户故事：** 作为系统管理员，我希望通过 RESTful API 管理用户和用户组，以便进行系统管理操作。

#### 验收标准

1. THE System SHALL 提供用户管理接口：POST/GET/PUT/DELETE /v0/admin/users
2. THE System SHALL 提供用户状态管理接口：PUT /v0/admin/users/:id/status
3. THE System SHALL 提供用户角色管理接口：PUT /v0/admin/users/:id/role
4. THE System SHALL 提供用户余额管理接口：PUT /v0/admin/users/:id/balance
5. THE System SHALL 提供用户组管理接口：POST/GET/PUT/DELETE /v0/admin/groups
6. THE System SHALL 提供用户组成员管理接口：POST/DELETE /v0/admin/groups/:id/members
7. THE System SHALL 对所有管理员接口进行权限验证（需要 admin 角色）
8. WHEN 请求缺少管理员权限时，THE System SHALL 返回 403 Forbidden 错误

### 需求 13：用户 API 接口

**用户故事：** 作为普通用户，我希望通过 API 管理个人信息，以便维护自己的账号。

#### 验收标准

1. THE System SHALL 提供获取个人信息接口：GET /v0/user/profile
2. THE System SHALL 提供更新个人信息接口：PUT /v0/user/profile
3. THE System SHALL 提供修改密码接口：PUT /v0/user/password
4. THE System SHALL 对所有用户接口进行身份验证（需要有效的 JWT Token）
5. WHEN 请求缺少有效 Token 时，THE System SHALL 返回 401 Unauthorized 错误

### 需求 14：认证 API 接口

**用户故事：** 作为用户，我希望通过 API 进行登录、登出和注册操作，以便访问系统。

#### 验收标准

1. THE System SHALL 提供登录接口：POST /v0/auth/login
2. THE System SHALL 提供登出接口：POST /v0/auth/logout
3. THE System SHALL 提供注册接口：POST /v0/auth/register
4. THE System SHALL 提供 Token 刷新接口：POST /v0/auth/refresh
5. THE System SHALL 提供用户名可用性检查接口：GET /v0/auth/check-username
6. THE System SHALL 提供邮箱可用性检查接口：GET /v0/auth/check-email
7. 登录和注册接口 SHALL 为公开接口，无需身份验证

### 需求 15：Swagger API 文档

**用户故事：** 作为开发者，我希望系统提供完整的 Swagger API 文档，以便了解和调用系统接口。

#### 验收标准

1. THE System SHALL 集成 Swagger/OpenAPI 文档生成工具
2. THE System SHALL 为每个 API 接口提供详细的描述、参数说明和响应示例
3. THE System SHALL 提供 Swagger UI 界面用于在线查看和测试 API
4. WHEN 访问 /swagger 路径时，THE System SHALL 展示完整的 API 文档
5. THE System SHALL 为每个接口提供请求示例和响应示例
6. THE System SHALL 在文档中标注接口的认证要求（公开/需要登录/需要管理员权限）

### 需求 16：API 请求响应格式

**用户故事：** 作为 API 调用者，我希望系统提供统一的请求响应格式，以便正确处理 API 交互。

#### 验收标准

1. THE System SHALL 使用 JSON 格式进行请求和响应数据传输
2. THE System SHALL 提供统一的错误响应格式，包含错误码和错误信息
3. THE System SHALL 为分页查询提供统一的分页响应格式（包含 total、page、page_size、items）
4. THE System SHALL 在响应头中包含适当的 Content-Type
5. WHEN 发生错误时，THE System SHALL 返回适当的 HTTP 状态码和错误详情

### 需求 17：数据验证

**用户故事：** 作为系统，我希望对所有输入数据进行验证，以便确保数据完整性和安全性。

#### 验收标准

1. THE System SHALL 验证用户名格式（长度 3-64 字符，仅允许字母数字和下划线）
2. THE System SHALL 验证邮箱格式的有效性
3. THE System SHALL 验证密码强度（最少 8 位字符）
4. THE System SHALL 验证用户组名格式（长度 1-64 字符）
5. IF 数据验证失败，THEN THE System SHALL 返回 400 Bad Request 和详细的验证错误信息
6. THE System SHALL 对所有字符串输入进行 XSS 防护处理

### 需求 18：日志记录

**用户故事：** 作为系统运维人员，我希望系统记录关键操作日志，以便进行问题排查和审计。

#### 验收标准

1. THE System SHALL 记录所有用户认证操作（登录、登出、注册）
2. THE System SHALL 记录所有管理员操作（用户创建、修改、删除）
3. THE System SHALL 记录所有错误和异常信息
4. THE System SHALL 在日志中包含请求 ID、用户 ID、操作类型、时间戳
5. THE System SHALL 支持配置日志级别和输出目标
