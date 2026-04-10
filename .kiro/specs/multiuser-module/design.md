# 设计文档

## 概述

本设计文档描述 CLI Proxy API 多用户模块的技术架构和实现方案。该模块基于 Go 语言开发，使用 GORM 作为 ORM 框架，PostgreSQL 作为数据存储，Gin 作为 Web 框架，实现用户管理、用户组管理和相关 API 接口。

### 设计目标

1. **模块化设计**：采用分层架构（Repository → Service → Handler），确保代码可维护性
2. **可扩展性**：支持未来扩展更多功能（API Key、权限、计费等）
3. **安全性**：JWT 认证、密码加密、输入验证
4. **文档完整**：集成 Swagger 提供完整的 API 文档

### 技术栈

| 组件 | 技术方案 | 说明 |
|------|----------|------|
| 语言 | Go 1.24+ | 与现有项目保持一致 |
| Web 框架 | Gin | 现有项目使用的框架 |
| ORM | GORM | Go 语言主流 ORM |
| 数据库 | PostgreSQL | 与现有 postgresstore.go 保持一致 |
| 认证 | JWT | 使用 golang-jwt 库 |
| 文档 | Swagger | 使用 swaggo/swag 生成 |
| 密码加密 | bcrypt | golang.org/x/crypto/bcrypt |

## 架构

### 整体架构图

```mermaid
graph TB
    subgraph "API Layer"
        A[Gin Router] --> B[Auth Middleware]
        B --> C[Admin Handlers]
        B --> D[User Handlers]
        B --> E[Auth Handlers]
    end

    subgraph "Service Layer"
        F[UserService]
        G[GroupService]
        H[AuthService]
    end
    
    subgraph "Repository Layer"
        I[UserRepository]
        J[GroupRepository]
        K[OAuthAccountRepository]
    end
    
    subgraph "Data Layer"
        L[(PostgreSQL)]
    end
    
    C --> F
    C --> G
    D --> F
    E --> H
    
    F --> I
    G --> J
    H --> I
    H --> K
    
    I --> L
    J --> L
    K --> L
```

### 目录结构

```
internal/multiuser/
├── models/                    # 数据模型
│   ├── user.go               # 用户模型
│   ├── group.go              # 用户组模型
│   ├── oauth_account.go      # 第三方登录账号模型
│   └── types.go              # 通用类型定义
│
├── repository/               # 数据访问层
│   ├── user_repo.go          # 用户仓库
│   ├── group_repo.go         # 用户组仓库
│   └── oauth_account_repo.go # 第三方账号仓库
│
├── service/                  # 业务逻辑层
│   ├── user_service.go       # 用户服务
│   ├── group_service.go      # 用户组服务
│   └── auth_service.go       # 认证服务
│
├── api/                      # HTTP API 层
│   ├── routes.go             # 路由定义
│   ├── handlers/             # 请求处理器
│   │   ├── admin_handler.go  # 管理员接口处理器
│   │   ├── user_handler.go   # 用户接口处理器
│   │   └── auth_handler.go   # 认证接口处理器
│   ├── middleware/           # 中间件
│   │   └── auth.go           # JWT 认证中间件
│   └── dto/                  # 数据传输对象
│       ├── request.go        # 请求 DTO
│       └── response.go       # 响应 DTO
│
├── config/                   # 配置管理
│   └── config.go             # 多用户模块配置
│
└── migration/                # 数据库迁移
    └── migrations.go         # 迁移脚本
```

## 组件和接口

### 数据模型层 (Models)

#### User 模型

```go
// internal/multiuser/models/user.go
type User struct {
    ID             string         `gorm:"primaryKey;type:varchar(36)" json:"id"`
    Username       string         `gorm:"uniqueIndex;type:varchar(64);not null" json:"username"`
    Email          string         `gorm:"uniqueIndex;type:varchar(255)" json:"email"`
    PasswordHash   string         `gorm:"type:varchar(255)" json:"-"`
    DisplayName    string         `gorm:"type:varchar(128)" json:"display_name"`
    AvatarURL      string         `gorm:"type:varchar(512)" json:"avatar_url"`
    Role           UserRole       `gorm:"type:varchar(32);default:'user'" json:"role"`
    Status         UserStatus     `gorm:"type:varchar(32);default:'active'" json:"status"`
    Balance        decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"balance"`
    CreditLimit    decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"credit_limit"`
    GroupID        *string        `gorm:"type:varchar(36);index" json:"group_id"`
    RegisterSource RegisterSource `gorm:"type:varchar(32);default:'local'" json:"register_source"`
    RegisterIP     string         `gorm:"type:varchar(64)" json:"register_ip"`
    Metadata       JSON           `gorm:"type:jsonb" json:"metadata"`
    CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    LastLoginAt    *time.Time     `json:"last_login_at"`
    
    // 关联
    Group         *UserGroup     `gorm:"foreignKey:GroupID" json:"group,omitempty"`
    OAuthAccounts []OAuthAccount `gorm:"foreignKey:UserID" json:"oauth_accounts,omitempty"`
}

type UserRole string
const (
    RoleAdmin    UserRole = "admin"
    RoleOperator UserRole = "operator"
    RoleUser     UserRole = "user"
)

type UserStatus string
const (
    StatusActive    UserStatus = "active"
    StatusSuspended UserStatus = "suspended"
    StatusDisabled  UserStatus = "disabled"
    StatusPending   UserStatus = "pending"
)

type RegisterSource string
const (
    RegisterSourceLocal  RegisterSource = "local"
    RegisterSourceGoogle RegisterSource = "google"
    RegisterSourceWechat RegisterSource = "wechat"
    RegisterSourceGitHub RegisterSource = "github"
    RegisterSourceInvite RegisterSource = "invite"
    RegisterSourceAdmin  RegisterSource = "admin"
)
```

#### UserGroup 模型

```go
// internal/multiuser/models/group.go
type UserGroup struct {
    ID             string          `gorm:"primaryKey;type:varchar(36)" json:"id"`
    Name           string          `gorm:"uniqueIndex;type:varchar(64);not null" json:"name"`
    Description    string          `gorm:"type:text" json:"description"`
    ParentID       *string         `gorm:"type:varchar(36);index" json:"parent_id"`
    BalancePool    decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"balance_pool"`
    SharedBalance  bool            `gorm:"default:false" json:"shared_balance"`
    RateMultiplier decimal.Decimal `gorm:"type:decimal(10,4);default:1.0" json:"rate_multiplier"`
    Priority       int             `gorm:"default:0" json:"priority"`
    Metadata       JSON            `gorm:"type:jsonb" json:"metadata"`
    CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
    
    // 关联
    Parent   *UserGroup  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
    Children []UserGroup `gorm:"foreignKey:ParentID" json:"children,omitempty"`
    Users    []User      `gorm:"foreignKey:GroupID" json:"users,omitempty"`
}
```

#### OAuthAccount 模型

```go
// internal/multiuser/models/oauth_account.go
type OAuthAccount struct {
    ID             string        `gorm:"primaryKey;type:varchar(36)" json:"id"`
    UserID         string        `gorm:"type:varchar(36);index;not null" json:"user_id"`
    Provider       OAuthProvider `gorm:"type:varchar(32);index;not null" json:"provider"`
    ProviderUserID string        `gorm:"type:varchar(128);index;not null" json:"provider_user_id"`
    Email          string        `gorm:"type:varchar(255)" json:"email"`
    DisplayName    string        `gorm:"type:varchar(128)" json:"display_name"`
    AvatarURL      string        `gorm:"type:varchar(512)" json:"avatar_url"`
    AccessToken    string        `gorm:"type:text" json:"-"`
    RefreshToken   string        `gorm:"type:text" json:"-"`
    TokenExpiresAt *time.Time    `json:"token_expires_at"`
    Scopes         StringArray   `gorm:"type:text[]" json:"scopes"`
    RawData        JSON          `gorm:"type:jsonb" json:"-"`
    CreatedAt      time.Time     `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt      time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
    LastUsedAt     *time.Time    `json:"last_used_at"`
    
    User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type OAuthProvider string
const (
    OAuthProviderGoogle    OAuthProvider = "google"
    OAuthProviderWechat    OAuthProvider = "wechat"
    OAuthProviderGitHub    OAuthProvider = "github"
    OAuthProviderApple     OAuthProvider = "apple"
    OAuthProviderMicrosoft OAuthProvider = "microsoft"
)
```

### 数据访问层 (Repository)

#### UserRepository 接口

```go
// internal/multiuser/repository/user_repo.go
type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    GetByID(ctx context.Context, id string) (*models.User, error)
    GetByUsername(ctx context.Context, username string) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, opts *ListOptions) ([]*models.User, int64, error)
    ExistsByUsername(ctx context.Context, username string) (bool, error)
    ExistsByEmail(ctx context.Context, email string) (bool, error)
    UpdateStatus(ctx context.Context, id string, status models.UserStatus) error
    UpdateRole(ctx context.Context, id string, role models.UserRole) error
    UpdateLastLogin(ctx context.Context, id string) error
}

type ListOptions struct {
    Page     int
    PageSize int
    SortBy   string
    SortDesc bool
    Filters  map[string]interface{}
}
```

#### GroupRepository 接口

```go
// internal/multiuser/repository/group_repo.go
type GroupRepository interface {
    Create(ctx context.Context, group *models.UserGroup) error
    GetByID(ctx context.Context, id string) (*models.UserGroup, error)
    GetByName(ctx context.Context, name string) (*models.UserGroup, error)
    Update(ctx context.Context, group *models.UserGroup) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, opts *ListOptions) ([]*models.UserGroup, int64, error)
    ExistsByName(ctx context.Context, name string) (bool, error)
    GetMembers(ctx context.Context, groupID string) ([]*models.User, error)
    GetChildren(ctx context.Context, parentID string) ([]*models.UserGroup, error)
}
```

### 业务逻辑层 (Service)

#### UserService 接口

```go
// internal/multiuser/service/user_service.go
type UserService interface {
    // 用户 CRUD
    CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*models.User, error)
    GetUser(ctx context.Context, id string) (*models.User, error)
    GetUserByUsername(ctx context.Context, username string) (*models.User, error)
    UpdateUser(ctx context.Context, id string, req *dto.UpdateUserRequest) (*models.User, error)
    DeleteUser(ctx context.Context, id string) error
    ListUsers(ctx context.Context, req *dto.ListUsersRequest) (*dto.ListUsersResponse, error)
    
    // 状态管理
    SetUserStatus(ctx context.Context, id string, status models.UserStatus) error
    SetUserRole(ctx context.Context, id string, role models.UserRole) error
    
    // 密码管理
    ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error
    SetPassword(ctx context.Context, userID string, password string) error
    
    // 验证
    CheckUsernameAvailable(ctx context.Context, username string) (bool, error)
    CheckEmailAvailable(ctx context.Context, email string) (bool, error)
}
```

#### GroupService 接口

```go
// internal/multiuser/service/group_service.go
type GroupService interface {
    // 用户组 CRUD
    CreateGroup(ctx context.Context, req *dto.CreateGroupRequest) (*models.UserGroup, error)
    GetGroup(ctx context.Context, id string) (*models.UserGroup, error)
    UpdateGroup(ctx context.Context, id string, req *dto.UpdateGroupRequest) (*models.UserGroup, error)
    DeleteGroup(ctx context.Context, id string) error
    ListGroups(ctx context.Context, req *dto.ListGroupsRequest) (*dto.ListGroupsResponse, error)
    
    // 成员管理
    AddUserToGroup(ctx context.Context, groupID, userID string) error
    RemoveUserFromGroup(ctx context.Context, groupID, userID string) error
    ListGroupMembers(ctx context.Context, groupID string) ([]*models.User, error)
}
```

#### AuthService 接口

```go
// internal/multiuser/service/auth_service.go
type AuthService interface {
    // 本地认证
    Register(ctx context.Context, req *dto.RegisterRequest) (*models.User, error)
    Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
    Logout(ctx context.Context, token string) error
    RefreshToken(ctx context.Context, refreshToken string) (*dto.TokenPair, error)
    
    // Token 管理
    GenerateTokenPair(user *models.User) (*dto.TokenPair, error)
    ValidateToken(token string) (*dto.TokenClaims, error)
    InvalidateToken(token string) error
}

// JWT Token 声明
type TokenClaims struct {
    UserID   string           `json:"user_id"`
    Username string           `json:"username"`
    Role     models.UserRole  `json:"role"`
    jwt.RegisteredClaims
}
```

### HTTP API 层 (Handlers)

#### DTO 定义

```go
// internal/multiuser/api/dto/request.go

// 用户创建请求
type CreateUserRequest struct {
    Username    string `json:"username" binding:"required,min=3,max=64"`
    Email       string `json:"email" binding:"required,email"`
    Password    string `json:"password" binding:"required,min=8"`
    DisplayName string `json:"display_name" binding:"max=128"`
    Role        string `json:"role" binding:"omitempty,oneof=admin operator user"`
    GroupID     string `json:"group_id"`
}

// 用户更新请求
type UpdateUserRequest struct {
    Email       *string `json:"email" binding:"omitempty,email"`
    DisplayName *string `json:"display_name" binding:"omitempty,max=128"`
    AvatarURL   *string `json:"avatar_url" binding:"omitempty,url"`
}

// 登录请求
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// 注册请求
type RegisterRequest struct {
    Username    string `json:"username" binding:"required,min=3,max=64"`
    Email       string `json:"email" binding:"required,email"`
    Password    string `json:"password" binding:"required,min=8"`
    DisplayName string `json:"display_name" binding:"max=128"`
}

// 修改密码请求
type ChangePasswordRequest struct {
    OldPassword string `json:"old_password" binding:"required"`
    NewPassword string `json:"new_password" binding:"required,min=8"`
}

// 用户组创建请求
type CreateGroupRequest struct {
    Name        string `json:"name" binding:"required,min=1,max=64"`
    Description string `json:"description"`
    ParentID    string `json:"parent_id"`
}

// 用户组更新请求
type UpdateGroupRequest struct {
    Name        *string `json:"name" binding:"omitempty,min=1,max=64"`
    Description *string `json:"description"`
}

// 分页请求
type ListUsersRequest struct {
    Page     int    `form:"page" binding:"min=1"`
    PageSize int    `form:"page_size" binding:"min=1,max=100"`
    SortBy   string `form:"sort_by"`
    SortDesc bool   `form:"sort_desc"`
    Status   string `form:"status"`
    Role     string `form:"role"`
    GroupID  string `form:"group_id"`
}
```

```go
// internal/multiuser/api/dto/response.go

// 登录响应
type LoginResponse struct {
    AccessToken  string      `json:"access_token"`
    RefreshToken string      `json:"refresh_token"`
    ExpiresIn    int64       `json:"expires_in"`
    TokenType    string      `json:"token_type"`
    User         *UserInfo   `json:"user"`
}

// Token 对
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}

// 用户信息响应
type UserInfo struct {
    ID          string `json:"id"`
    Username    string `json:"username"`
    Email       string `json:"email"`
    DisplayName string `json:"display_name"`
    AvatarURL   string `json:"avatar_url"`
    Role        string `json:"role"`
    Status      string `json:"status"`
    Balance     string `json:"balance"`
    GroupID     string `json:"group_id,omitempty"`
    CreatedAt   string `json:"created_at"`
}

// 分页响应
type ListUsersResponse struct {
    Total    int64       `json:"total"`
    Page     int         `json:"page"`
    PageSize int         `json:"page_size"`
    Items    []*UserInfo `json:"items"`
}

// 错误响应
type ErrorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
}
```

### 中间件

#### JWT 认证中间件

```go
// internal/multiuser/api/middleware/auth.go

// JWTAuthMiddleware JWT 认证中间件
func JWTAuthMiddleware(authService service.AuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := extractToken(c)
        if token == "" {
            c.AbortWithStatusJSON(401, dto.ErrorResponse{
                Code:    "AUTH_TOKEN_MISSING",
                Message: "缺少认证 Token",
            })
            return
        }
        
        claims, err := authService.ValidateToken(token)
        if err != nil {
            c.AbortWithStatusJSON(401, dto.ErrorResponse{
                Code:    "AUTH_TOKEN_INVALID",
                Message: "Token 无效或已过期",
            })
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("role", claims.Role)
        c.Next()
    }
}

// AdminAuthMiddleware 管理员权限中间件
func AdminAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        role, exists := c.Get("role")
        if !exists || role != models.RoleAdmin {
            c.AbortWithStatusJSON(403, dto.ErrorResponse{
                Code:    "PERMISSION_DENIED",
                Message: "需要管理员权限",
            })
            return
        }
        c.Next()
    }
}

func extractToken(c *gin.Context) string {
    // 从 Authorization header 提取 Bearer token
    auth := c.GetHeader("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}
```

### 路由定义

```go
// internal/multiuser/api/routes.go

func SetupRoutes(r *gin.Engine, services *Services) {
    // Swagger 文档
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
    
    // 认证接口（公开）
    auth := r.Group("/v0/auth")
    {
        auth.POST("/login", handlers.Login)
        auth.POST("/logout", handlers.Logout)
        auth.POST("/register", handlers.Register)
        auth.POST("/refresh", handlers.RefreshToken)
        auth.GET("/check-username", handlers.CheckUsername)
        auth.GET("/check-email", handlers.CheckEmail)
    }
    
    // 用户接口（需要登录）
    user := r.Group("/v0/user")
    user.Use(middleware.JWTAuthMiddleware(services.AuthService))
    {
        user.GET("/profile", handlers.GetProfile)
        user.PUT("/profile", handlers.UpdateProfile)
        user.PUT("/password", handlers.ChangePassword)
    }
    
    // 管理员接口（需要管理员权限）
    admin := r.Group("/v0/admin")
    admin.Use(middleware.JWTAuthMiddleware(services.AuthService))
    admin.Use(middleware.AdminAuthMiddleware())
    {
        // 用户管理
        admin.POST("/users", handlers.CreateUser)
        admin.GET("/users", handlers.ListUsers)
        admin.GET("/users/:id", handlers.GetUser)
        admin.PUT("/users/:id", handlers.UpdateUser)
        admin.DELETE("/users/:id", handlers.DeleteUser)
        admin.PUT("/users/:id/status", handlers.SetUserStatus)
        admin.PUT("/users/:id/role", handlers.SetUserRole)
        admin.PUT("/users/:id/balance", handlers.SetUserBalance)
        
        // 用户组管理
        admin.POST("/groups", handlers.CreateGroup)
        admin.GET("/groups", handlers.ListGroups)
        admin.GET("/groups/:id", handlers.GetGroup)
        admin.PUT("/groups/:id", handlers.UpdateGroup)
        admin.DELETE("/groups/:id", handlers.DeleteGroup)
        admin.POST("/groups/:id/members", handlers.AddGroupMember)
        admin.DELETE("/groups/:id/members/:user_id", handlers.RemoveGroupMember)
    }
}
```

## 数据模型

### 数据库 ER 图

```mermaid
erDiagram
    User ||--o{ OAuthAccount : has
    User }o--|| UserGroup : belongs_to
    UserGroup ||--o{ UserGroup : parent_of
    
    User {
        varchar(36) id PK
        varchar(64) username UK
        varchar(255) email UK
        varchar(255) password_hash
        varchar(128) display_name
        varchar(512) avatar_url
        varchar(32) role
        varchar(32) status
        decimal balance
        decimal credit_limit
        varchar(36) group_id FK
        varchar(32) register_source
        varchar(64) register_ip
        jsonb metadata
        timestamp created_at
        timestamp updated_at
        timestamp last_login_at
    }
    
    UserGroup {
        varchar(36) id PK
        varchar(64) name UK
        text description
        varchar(36) parent_id FK
        decimal balance_pool
        boolean shared_balance
        decimal rate_multiplier
        int priority
        jsonb metadata
        timestamp created_at
        timestamp updated_at
    }
    
    OAuthAccount {
        varchar(36) id PK
        varchar(36) user_id FK
        varchar(32) provider
        varchar(128) provider_user_id
        varchar(255) email
        varchar(128) display_name
        varchar(512) avatar_url
        text access_token
        text refresh_token
        timestamp token_expires_at
        text[] scopes
        jsonb raw_data
        timestamp created_at
        timestamp updated_at
        timestamp last_used_at
    }
```

### 数据库迁移

```go
// internal/multiuser/migration/migrations.go

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{},
        &models.UserGroup{},
        &models.OAuthAccount{},
    )
}

// 创建索引
func CreateIndexes(db *gorm.DB) error {
    // OAuthAccount 唯一约束
    return db.Exec(`
        CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_provider_user 
        ON oauth_accounts(provider, provider_user_id)
    `).Error
}

// 创建默认管理员
func CreateDefaultAdmin(db *gorm.DB, config *config.MultiUserConfig) error {
    var count int64
    db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count)
    if count > 0 {
        return nil // 已存在管理员
    }
    
    password := config.Admin.Password
    if password == "" {
        password = generateRandomPassword()
        log.Printf("生成默认管理员密码: %s", password)
    }
    
    hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
    admin := &models.User{
        ID:           uuid.New().String(),
        Username:     config.Admin.Username,
        Email:        config.Admin.Email,
        PasswordHash: string(hash),
        Role:         models.RoleAdmin,
        Status:       models.StatusActive,
        RegisterSource: models.RegisterSourceAdmin,
    }
    
    return db.Create(admin).Error
}
```

## 正确性属性

*正确性属性是指在系统所有有效执行中都应该保持为真的特征或行为。属性作为人类可读规范和机器可验证正确性保证之间的桥梁。*

### Property 1: 用户名和邮箱唯一性约束

*对于任意* 用户名或邮箱，如果该值已存在于数据库中，则创建新用户时应该返回错误，数据库中不应存在重复记录。

**验证: 需求 5.1, 7.7, 10.1**

### Property 2: 密码加密存储

*对于任意* 用户密码，存储到数据库的值应该是 bcrypt 哈希值，且原始密码可以通过 bcrypt.CompareHashAndPassword 验证成功。

**验证: 需求 5.2, 8.5**

### Property 3: 用户查询一致性

*对于任意* 已创建的用户，通过 ID、用户名、邮箱三种方式查询应该返回相同的用户实体。

**验证: 需求 5.3**

### Property 4: 分页查询正确性

*对于任意* 分页参数（page, pageSize），返回的结果数量应该 <= pageSize，且 total 应该等于符合条件的总记录数。

**验证: 需求 5.4**

### Property 5: 更新时间记录

*对于任意* 用户或用户组的更新操作，UpdatedAt 字段应该被更新为当前时间，且新时间应该 >= 原时间。

**验证: 需求 5.5, 9.2**

### Property 6: 登录凭据验证

*对于任意* 用户名/邮箱和密码组合，如果凭据有效且用户状态为 active，则登录应该成功；否则应该失败。

**验证: 需求 6.1, 6.6**

### Property 7: JWT Token 往返一致性

*对于任意* 成功登录生成的 Token，解析该 Token 应该能够还原出正确的用户信息（UserID、Username、Role）。

**验证: 需求 6.2, 6.4**

### Property 8: Token 刷新有效性

*对于任意* 有效的 Refresh Token，刷新操作应该返回新的有效 Token 对，且新 Token 包含相同的用户信息。

**验证: 需求 6.4**

### Property 9: 登出后 Token 失效

*对于任意* 用户登出操作，登出后使用原 Access Token 访问受保护资源应该返回 401 错误。

**验证: 需求 6.5**

### Property 10: 用户状态影响登录

*对于任意* 状态为 suspended 或 disabled 的用户，使用正确的凭据登录应该失败并返回相应错误。

**验证: 需求 6.6**

### Property 11: 注册默认值分配

*对于任意* 新注册用户，应该根据配置分配默认余额、默认角色和默认状态。

**验证: 需求 7.3, 7.6**

### Property 12: 输入验证规则

*对于任意* 用户输入：
- 用户名长度应在 3-64 字符之间，仅包含字母数字和下划线
- 邮箱应符合标准邮箱格式
- 密码长度应 >= 8 字符
- 用户组名长度应在 1-64 字符之间

不符合规则的输入应该返回 400 错误。

**验证: 需求 17.1, 17.2, 17.3, 17.4, 17.5**

### Property 13: 密码修改验证

*对于任意* 密码修改请求，只有当旧密码验证正确且新密码符合强度要求时，修改才应该成功。

**验证: 需求 8.1, 8.2**

### Property 14: 用户状态变更即时生效

*对于任意* 用户状态从 active 变更为 suspended 或 disabled，该用户的所有现有 Token 应该立即失效。

**验证: 需求 9.3**

### Property 15: 用户组层级结构完整性

*对于任意* 具有父子关系的用户组，子组的 ParentID 应该指向有效的父组，且不应存在循环引用。

**验证: 需求 4.2, 10.2**

### Property 16: 用户组成员管理一致性

*对于任意* 用户添加到用户组的操作，添加后：
- 用户的 GroupID 应该等于目标组 ID
- 查询该组成员应该包含该用户

*对于任意* 用户从用户组移除的操作，移除后：
- 用户的 GroupID 应该为 null
- 查询该组成员不应包含该用户

**验证: 需求 11.1, 11.2, 11.3, 11.4**

### Property 17: OAuth 账号唯一绑定

*对于任意* OAuth 供应商和供应商用户 ID 组合，在系统中最多只能绑定到一个用户。

**验证: 需求 3.3**

### Property 18: API 响应格式一致性

*对于任意* API 响应：
- 成功响应应该包含正确的数据结构
- 错误响应应该包含 code 和 message 字段
- 分页响应应该包含 total、page、page_size、items 字段

**验证: 需求 16.1, 16.2, 16.3**

### Property 19: XSS 防护

*对于任意* 包含 HTML 标签或 JavaScript 代码的字符串输入，存储和返回时应该被正确转义或过滤。

**验证: 需求 17.6**

## 错误处理

### 错误码定义

| 错误码 | HTTP 状态码 | 描述 |
|--------|-------------|------|
| `AUTH_TOKEN_MISSING` | 401 | 缺少认证 Token |
| `AUTH_TOKEN_INVALID` | 401 | Token 无效或已过期 |
| `AUTH_TOKEN_EXPIRED` | 401 | Token 已过期 |
| `AUTH_INVALID_CREDENTIALS` | 401 | 用户名或密码错误 |
| `AUTH_USER_SUSPENDED` | 403 | 用户已被暂停 |
| `AUTH_USER_DISABLED` | 403 | 用户已被禁用 |
| `PERMISSION_DENIED` | 403 | 权限不足 |
| `USER_NOT_FOUND` | 404 | 用户不存在 |
| `GROUP_NOT_FOUND` | 404 | 用户组不存在 |
| `USER_ALREADY_EXISTS` | 409 | 用户名或邮箱已存在 |
| `GROUP_ALREADY_EXISTS` | 409 | 用户组名已存在 |
| `VALIDATION_ERROR` | 400 | 数据验证失败 |
| `INVALID_PASSWORD` | 400 | 密码不符合要求 |
| `REGISTRATION_DISABLED` | 403 | 注册功能已关闭 |
| `DATABASE_ERROR` | 500 | 数据库操作失败 |
| `INTERNAL_ERROR` | 500 | 内部服务器错误 |

### 错误响应格式

```go
// internal/multiuser/api/dto/response.go

type ErrorResponse struct {
    Code    string      `json:"code"`              // 错误码
    Message string      `json:"message"`           // 错误描述
    Details interface{} `json:"details,omitempty"` // 详细信息（可选）
}

// 验证错误详情
type ValidationErrorDetails struct {
    Field   string `json:"field"`   // 字段名
    Message string `json:"message"` // 错误信息
}
```

### 错误处理示例

```go
// 用户不存在
c.JSON(404, ErrorResponse{
    Code:    "USER_NOT_FOUND",
    Message: "用户不存在",
})

// 验证错误
c.JSON(400, ErrorResponse{
    Code:    "VALIDATION_ERROR",
    Message: "数据验证失败",
    Details: []ValidationErrorDetails{
        {Field: "username", Message: "用户名长度必须在 3-64 字符之间"},
        {Field: "email", Message: "邮箱格式无效"},
    },
})

// 登录失败（不泄露具体原因）
c.JSON(401, ErrorResponse{
    Code:    "AUTH_INVALID_CREDENTIALS",
    Message: "用户名或密码错误",
})
```

## 测试策略

### 测试类型

本模块采用双重测试策略：

1. **单元测试**：验证具体示例、边缘情况和错误条件
2. **属性测试**：验证跨所有输入的通用属性

### 属性测试配置

- 使用 `github.com/leanovate/gopter` 作为属性测试库
- 每个属性测试最少运行 100 次迭代
- 每个测试必须标注对应的设计文档属性

### 测试文件结构

```
internal/multiuser/
├── models/
│   └── user_test.go           # 模型单元测试
├── repository/
│   └── user_repo_test.go      # Repository 单元测试
├── service/
│   ├── user_service_test.go   # Service 单元测试
│   └── auth_service_test.go   # 认证服务测试
├── api/
│   └── handlers/
│       └── handlers_test.go   # API 集成测试
└── tests/
    └── property_test.go       # 属性测试
```

### 属性测试示例

```go
// internal/multiuser/tests/property_test.go

// Feature: multiuser-module, Property 1: 用户名和邮箱唯一性约束
func TestProperty_UsernameEmailUniqueness(t *testing.T) {
    properties := gopter.NewProperties(gopter.DefaultTestParameters())
    properties.Property("重复用户名应该创建失败", prop.ForAll(
        func(username string) bool {
            // 创建第一个用户
            user1 := createUser(username, randomEmail())
            // 尝试创建相同用户名的第二个用户
            _, err := createUser(username, randomEmail())
            return err != nil && isUniqueConstraintError(err)
        },
        gen.AlphaString().WithMinLen(3).WithMaxLen(64),
    ))
    properties.TestingRun(t)
}

// Feature: multiuser-module, Property 2: 密码加密存储
func TestProperty_PasswordEncryption(t *testing.T) {
    properties := gopter.NewProperties(gopter.DefaultTestParameters())
    properties.Property("密码应该使用 bcrypt 加密存储", prop.ForAll(
        func(password string) bool {
            user := createUserWithPassword(password)
            // 验证存储的不是明文
            if user.PasswordHash == password {
                return false
            }
            // 验证可以通过 bcrypt 验证
            err := bcrypt.CompareHashAndPassword(
                []byte(user.PasswordHash), 
                []byte(password),
            )
            return err == nil
        },
        gen.AlphaString().WithMinLen(8).WithMaxLen(64),
    ))
    properties.TestingRun(t)
}

// Feature: multiuser-module, Property 7: JWT Token 往返一致性
func TestProperty_JWTTokenRoundTrip(t *testing.T) {
    properties := gopter.NewProperties(gopter.DefaultTestParameters())
    properties.Property("Token 解析应该还原用户信息", prop.ForAll(
        func(userID, username string, role models.UserRole) bool {
            user := &models.User{
                ID:       userID,
                Username: username,
                Role:     role,
            }
            tokenPair, _ := authService.GenerateTokenPair(user)
            claims, err := authService.ValidateToken(tokenPair.AccessToken)
            if err != nil {
                return false
            }
            return claims.UserID == userID && 
                   claims.Username == username && 
                   claims.Role == role
        },
        gen.UUIDString(),
        gen.AlphaString().WithMinLen(3).WithMaxLen(64),
        gen.OneConstOf(models.RoleAdmin, models.RoleOperator, models.RoleUser),
    ))
    properties.TestingRun(t)
}
```

### 单元测试示例

```go
// internal/multiuser/service/user_service_test.go

func TestCreateUser_Success(t *testing.T) {
    req := &dto.CreateUserRequest{
        Username:    "testuser",
        Email:       "test@example.com",
        Password:    "password123",
        DisplayName: "Test User",
    }
    
    user, err := userService.CreateUser(ctx, req)
    
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
    assert.Equal(t, req.Username, user.Username)
    assert.Equal(t, models.RoleUser, user.Role)
    assert.Equal(t, models.StatusActive, user.Status)
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
    // 先创建一个用户
    createTestUser("existinguser", "existing@example.com")
    
    // 尝试创建相同用户名的用户
    req := &dto.CreateUserRequest{
        Username: "existinguser",
        Email:    "new@example.com",
        Password: "password123",
    }
    
    _, err := userService.CreateUser(ctx, req)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "USER_ALREADY_EXISTS")
}

func TestLogin_InvalidCredentials(t *testing.T) {
    createTestUser("loginuser", "login@example.com")
    
    req := &dto.LoginRequest{
        Username: "loginuser",
        Password: "wrongpassword",
    }
    
    _, err := authService.Login(ctx, req)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "AUTH_INVALID_CREDENTIALS")
}
```

## Swagger API 文档集成

### Swagger 配置

```go
// internal/multiuser/api/swagger.go

// @title           CLI Proxy API 多用户模块
// @version         1.0
// @description     多用户管理模块 API 文档
// @host            localhost:8080
// @BasePath        /v0

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer Token 认证
```

### API 文档注解示例

```go
// internal/multiuser/api/handlers/auth_handler.go

// Login 用户登录
// @Summary      用户登录
// @Description  使用用户名/邮箱和密码登录，获取 JWT Token
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginRequest true "登录请求"
// @Success      200 {object} dto.LoginResponse "登录成功"
// @Failure      400 {object} dto.ErrorResponse "请求参数错误"
// @Failure      401 {object} dto.ErrorResponse "认证失败"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
    // 实现代码
}

// Register 用户注册
// @Summary      用户注册
// @Description  注册新用户账号
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterRequest true "注册请求"
// @Success      201 {object} models.User "注册成功"
// @Failure      400 {object} dto.ErrorResponse "请求参数错误"
// @Failure      403 {object} dto.ErrorResponse "注册功能已关闭"
// @Failure      409 {object} dto.ErrorResponse "用户名或邮箱已存在"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
    // 实现代码
}
```

```go
// internal/multiuser/api/handlers/admin_handler.go

// CreateUser 创建用户
// @Summary      创建用户
// @Description  管理员创建新用户
// @Tags         管理员-用户管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateUserRequest true "创建用户请求"
// @Success      201 {object} models.User "创建成功"
// @Failure      400 {object} dto.ErrorResponse "请求参数错误"
// @Failure      401 {object} dto.ErrorResponse "未认证"
// @Failure      403 {object} dto.ErrorResponse "权限不足"
// @Failure      409 {object} dto.ErrorResponse "用户名或邮箱已存在"
// @Router       /admin/users [post]
func (h *AdminHandler) CreateUser(c *gin.Context) {
    // 实现代码
}

// ListUsers 获取用户列表
// @Summary      获取用户列表
// @Description  分页获取用户列表，支持筛选和排序
// @Tags         管理员-用户管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "页码" default(1)
// @Param        page_size query int false "每页数量" default(20)
// @Param        sort_by query string false "排序字段"
// @Param        sort_desc query bool false "是否降序"
// @Param        status query string false "用户状态筛选"
// @Param        role query string false "用户角色筛选"
// @Success      200 {object} dto.ListUsersResponse "获取成功"
// @Failure      401 {object} dto.ErrorResponse "未认证"
// @Failure      403 {object} dto.ErrorResponse "权限不足"
// @Router       /admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
    // 实现代码
}

// CreateGroup 创建用户组
// @Summary      创建用户组
// @Description  管理员创建新用户组
// @Tags         管理员-用户组管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateGroupRequest true "创建用户组请求"
// @Success      201 {object} models.UserGroup "创建成功"
// @Failure      400 {object} dto.ErrorResponse "请求参数错误"
// @Failure      401 {object} dto.ErrorResponse "未认证"
// @Failure      403 {object} dto.ErrorResponse "权限不足"
// @Failure      409 {object} dto.ErrorResponse "用户组名已存在"
// @Router       /admin/groups [post]
func (h *AdminHandler) CreateGroup(c *gin.Context) {
    // 实现代码
}
```

### API 请求响应示例

#### 登录接口

**请求示例:**
```json
POST /v0/auth/login
Content-Type: application/json

{
    "username": "admin",
    "password": "SecurePass123!"
}
```

**成功响应:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400,
    "token_type": "Bearer",
    "user": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "username": "admin",
        "email": "admin@example.com",
        "display_name": "管理员",
        "role": "admin",
        "status": "active",
        "balance": "100.00000000",
        "created_at": "2026-01-28T10:00:00Z"
    }
}
```

**失败响应:**
```json
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
    "code": "AUTH_INVALID_CREDENTIALS",
    "message": "用户名或密码错误"
}
```

#### 创建用户接口

**请求示例:**
```json
POST /v0/admin/users
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json

{
    "username": "newuser",
    "email": "newuser@example.com",
    "password": "SecurePass123!",
    "display_name": "新用户",
    "role": "user",
    "group_id": "group-uuid-here"
}
```

**成功响应:**
```json
HTTP/1.1 201 Created
Content-Type: application/json

{
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "username": "newuser",
    "email": "newuser@example.com",
    "display_name": "新用户",
    "role": "user",
    "status": "active",
    "balance": "1.00000000",
    "group_id": "group-uuid-here",
    "register_source": "admin",
    "created_at": "2026-01-28T12:00:00Z",
    "updated_at": "2026-01-28T12:00:00Z"
}
```

#### 获取用户列表接口

**请求示例:**
```
GET /v0/admin/users?page=1&page_size=10&status=active&sort_by=created_at&sort_desc=true
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

**成功响应:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
    "total": 25,
    "page": 1,
    "page_size": 10,
    "items": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "username": "user1",
            "email": "user1@example.com",
            "display_name": "用户1",
            "role": "user",
            "status": "active",
            "balance": "50.00000000",
            "created_at": "2026-01-28T12:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "username": "user2",
            "email": "user2@example.com",
            "display_name": "用户2",
            "role": "user",
            "status": "active",
            "balance": "30.00000000",
            "created_at": "2026-01-28T11:00:00Z"
        }
    ]
}
```

#### 创建用户组接口

**请求示例:**
```json
POST /v0/admin/groups
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json

{
    "name": "开发团队",
    "description": "研发部门用户组",
    "parent_id": null
}
```

**成功响应:**
```json
HTTP/1.1 201 Created
Content-Type: application/json

{
    "id": "group-uuid-here",
    "name": "开发团队",
    "description": "研发部门用户组",
    "parent_id": null,
    "balance_pool": "0.00000000",
    "shared_balance": false,
    "rate_multiplier": "1.0000",
    "priority": 0,
    "created_at": "2026-01-28T12:00:00Z",
    "updated_at": "2026-01-28T12:00:00Z"
}
```

#### 添加用户组成员接口

**请求示例:**
```json
POST /v0/admin/groups/group-uuid-here/members
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json

{
    "user_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**成功响应:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "用户已添加到用户组"
}
```

#### 用户注册接口

**请求示例:**
```json
POST /v0/auth/register
Content-Type: application/json

{
    "username": "newuser",
    "email": "newuser@example.com",
    "password": "SecurePass123!",
    "display_name": "新用户"
}
```

**成功响应:**
```json
HTTP/1.1 201 Created
Content-Type: application/json

{
    "id": "550e8400-e29b-41d4-a716-446655440003",
    "username": "newuser",
    "email": "newuser@example.com",
    "display_name": "新用户",
    "role": "user",
    "status": "active",
    "balance": "1.00000000",
    "register_source": "local",
    "created_at": "2026-01-28T14:00:00Z",
    "message": "注册成功"
}
```

#### 修改密码接口

**请求示例:**
```json
PUT /v0/user/password
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json

{
    "old_password": "OldPass123!",
    "new_password": "NewSecurePass456!"
}
```

**成功响应:**
```json
HTTP/1.1 200 OK
Content-Type: application/json

{
    "message": "密码修改成功"
}
```

**失败响应:**
```json
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
    "code": "INVALID_PASSWORD",
    "message": "当前密码错误"
}
```

## 配置设计

### 多用户模块配置结构

```go
// internal/multiuser/config/config.go

type MultiUserConfig struct {
    Enabled      bool            `yaml:"enabled"`
    Database     DatabaseConfig  `yaml:"database"`
    Auth         AuthConfig      `yaml:"auth"`
    Registration RegistrationConfig `yaml:"registration"`
    Admin        AdminConfig     `yaml:"admin"`
}

type DatabaseConfig struct {
    DSN             string        `yaml:"dsn"`
    Schema          string        `yaml:"schema"`
    MaxOpenConns    int           `yaml:"max-open-conns"`
    MaxIdleConns    int           `yaml:"max-idle-conns"`
    ConnMaxLifetime time.Duration `yaml:"conn-max-lifetime"`
}

type AuthConfig struct {
    JWTSecret       string        `yaml:"jwt-secret"`
    AccessTokenTTL  time.Duration `yaml:"access-token-ttl"`
    RefreshTokenTTL time.Duration `yaml:"refresh-token-ttl"`
}

type RegistrationConfig struct {
    AllowPublicRegistration   bool   `yaml:"allow-public-registration"`
    RequireEmailVerification  bool   `yaml:"require-email-verification"`
    RequireAdminApproval      bool   `yaml:"require-admin-approval"`
    DefaultGroupID            string `yaml:"default-group-id"`
    DefaultBalance            string `yaml:"default-balance"`
    DefaultRole               string `yaml:"default-role"`
}

type AdminConfig struct {
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    Email    string `yaml:"email"`
}
```

### 配置文件示例

```yaml
# config.yaml

multi-user:
  enabled: true
  
  database:
    dsn: "postgres://user:password@localhost:5432/cliproxy?sslmode=disable"
    schema: "multiuser"
    max-open-conns: 25
    max-idle-conns: 10
    conn-max-lifetime: 5m
  
  auth:
    jwt-secret: "your-secret-key-here"
    access-token-ttl: 24h
    refresh-token-ttl: 168h
  
  registration:
    allow-public-registration: false
    require-email-verification: false
    require-admin-approval: false
    default-group-id: ""
    default-balance: "1.00"
    default-role: "user"
  
  admin:
    username: "admin"
    password: ""  # 留空则自动生成
    email: "admin@example.com"
```

## 依赖项

### 新增依赖

```go
// go.mod 新增依赖

require (
    gorm.io/gorm v1.25.x
    gorm.io/driver/postgres v1.5.x
    github.com/golang-jwt/jwt/v5 v5.x.x
    github.com/shopspring/decimal v1.3.x
    github.com/swaggo/swag v1.16.x
    github.com/swaggo/gin-swagger v1.6.x
    github.com/swaggo/files v1.0.x
    github.com/leanovate/gopter v0.2.x  // 属性测试
)
```

## 安全考虑

1. **密码安全**：使用 bcrypt 加密，cost >= 12
2. **JWT 安全**：使用 HS256 签名，支持 Token 黑名单
3. **输入验证**：所有输入进行严格验证和 XSS 防护
4. **权限控制**：基于角色的访问控制（RBAC）
5. **日志安全**：敏感信息（密码、Token）不记录到日志
6. **错误信息**：登录失败不泄露具体原因
