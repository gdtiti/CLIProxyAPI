# 添加新提供商

为项目添加新 AI 提供商支持的步骤指南。

## 1. 创建认证模块

在 `internal/auth/` 下创建新目录：

```
internal/auth/newprovider/
├── newprovider_auth.go   # 认证流程实现
├── newprovider_token.go  # Token 结构定义
└── oauth_server.go       # OAuth 回调处理（如需要）
```

## 2. 实现 Token 接口

参考 `internal/auth/models.go` 中的接口定义，实现：
- `GetAccessToken() string`
- `IsExpired() bool`
- `Refresh() error`

## 3. 创建执行器

在 `internal/runtime/executor/` 创建执行器：

```go
// newprovider_executor.go
type NewProviderExecutor struct {
    // ...
}

func (e *NewProviderExecutor) Execute(ctx context.Context, req *Request) (*Response, error) {
    // 实现请求执行逻辑
}
```

## 4. 注册翻译器

如需格式转换，在 `internal/translator/` 添加翻译器：
- 创建 `internal/translator/newprovider/openai/` 目录
- 实现 `TranslateRequest` 和 `TranslateResponse`

## 5. 添加配置支持

在 `internal/config/config.go` 添加配置结构：

```go
type NewProviderKey struct {
    APIKey   string `yaml:"api-key"`
    BaseURL  string `yaml:"base-url,omitempty"`
    // ...
}
```

## 6. 注册登录命令

在 `cmd/server/main.go` 添加命令行参数和处理逻辑。

## 7. 测试验证

- 编写单元测试
- 手动测试认证流程
- 验证 API 请求转发
