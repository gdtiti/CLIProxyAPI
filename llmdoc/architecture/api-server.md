# API 服务器架构

## 1. 身份

- **是什么：** 基于 Gin 框架的 HTTP API 服务器
- **目的：** 提供 OpenAI/Claude/Gemini 兼容的 API 端点，处理请求路由和认证

## 2. 核心组件

- `internal/api/server.go` (Server, NewServer, setupRoutes): 服务器初始化、路由注册、中间件配置
- `internal/api/middleware/` (request_logging.go): 请求日志、响应包装中间件
- `sdk/api/handlers/` (openai/, claude/, gemini/): 各格式 API 处理器
- `internal/api/modules/amp/`: Amp CLI 模块，支持模型映射热重载

## 3. 执行流程（LLM 检索地图）

- **1. 请求到达：** Gin 引擎接收 HTTP 请求
- **2. 中间件处理：** CORS → 日志 → 认证（AuthMiddleware）
- **3. 路由分发：** 根据路径分发到对应处理器（/v1/chat/completions → OpenAI）
- **4. 处理器执行：** `sdk/api/handlers/openai/chat.go` 解析请求、调用执行器
- **5. 响应返回：** 流式或非流式响应返回客户端

## 4. API 端点

| 路径 | 格式 | 处理器 |
|------|------|--------|
| `/v1/chat/completions` | OpenAI | `openai.ChatCompletions` |
| `/v1/messages` | Claude | `claude.ClaudeMessages` |
| `/v1beta/models/*` | Gemini | `gemini.GeminiHandler` |
| `/v1/responses` | OpenAI Responses | `openai.Responses` |
| `/v0/management/*` | 管理 API | `managementHandlers` |

## 5. 认证中间件

`AuthMiddleware` 从请求头提取 API Key，通过 `accessManager` 验证：
- Bearer Token 认证
- 本地密码认证（localhost）
- 环境变量 `MANAGEMENT_PASSWORD`
