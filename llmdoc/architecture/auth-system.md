# 认证系统架构

## 1. 身份

- **是什么：** 多提供商 OAuth 认证框架
- **目的：** 统一管理各 AI 提供商的认证流程、Token 存储和刷新

## 2. 核心组件

- `internal/auth/kiro/` (aws.go, oauth.go, token.go): Kiro/AWS CodeWhisperer 认证，支持 Builder ID、Google OAuth、Token 导入
- `internal/auth/claude/` (anthropic_auth.go, oauth_server.go): Claude/Anthropic OAuth 认证
- `internal/auth/copilot/` (copilot_auth.go, oauth.go): GitHub Copilot Device Flow 认证
- `internal/auth/gemini/` (gemini_auth.go): Google Gemini OAuth 认证
- `internal/auth/codex/` (openai_auth.go): OpenAI Codex OAuth 认证
- `sdk/auth/manager.go`: 统一认证管理器，协调各提供商

## 3. 执行流程（LLM 检索地图）

- **1. 用户发起登录：** `cmd/server/main.go` 解析命令行参数（如 `--kiro-login`）
- **2. 调用认证命令：** `internal/cmd/kiro_login.go` 启动对应认证流程
- **3. OAuth 流程执行：** `internal/auth/kiro/oauth.go` 处理 PKCE、授权码交换
- **4. Token 存储：** `sdk/auth/filestore.go` 或数据库存储持久化 Token
- **5. 后台刷新：** `internal/auth/kiro/background_refresh.go` 定时刷新即将过期的 Token

## 4. Token 存储后端

| 后端 | 实现 | 用途 |
|------|------|------|
| 文件系统 | `sdk/auth/filestore.go` | 默认本地存储 |
| PostgreSQL | `internal/store/postgresstore.go` | 云部署持久化 |
| Git | `internal/store/gitstore.go` | 版本控制同步 |
| S3 兼容 | `internal/store/objectstore.go` | 对象存储 |

## 5. 设计原理

- **PKCE 安全：** 所有 OAuth 流程使用 PKCE 防止授权码拦截
- **后台刷新：** Token 过期前 10 分钟自动刷新，避免请求中断
- **多账户支持：** 支持同一提供商多个凭据，通过优先级和前缀区分
