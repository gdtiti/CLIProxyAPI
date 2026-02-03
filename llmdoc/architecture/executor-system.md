# 执行器系统架构

## 1. 身份

- **是什么：** 提供商 API 请求执行引擎
- **目的：** 处理实际的 HTTP 请求发送、流式响应、错误重试

## 2. 核心组件

- `internal/runtime/executor/kiro_executor.go`: Kiro/AWS CodeWhisperer 执行器
- `internal/runtime/executor/claude_executor.go`: Claude/Anthropic 执行器
- `internal/runtime/executor/gemini_executor.go`: Gemini API 执行器
- `internal/runtime/executor/codex_executor.go`: OpenAI Codex 执行器
- `internal/runtime/executor/github_copilot_executor.go`: GitHub Copilot 执行器
- `internal/runtime/executor/openai_compat_executor.go`: OpenAI 兼容提供商执行器

## 3. 执行流程（LLM 检索地图）

- **1. 选择执行器：** 根据模型名称和凭据类型选择对应执行器
- **2. 构建请求：** `payload_helpers.go` 构建 HTTP 请求体
- **3. Token 获取：** `token_helpers.go` 从认证管理器获取有效 Token
- **4. 发送请求：** 通过代理（如配置）发送 HTTP 请求
- **5. 处理响应：** 流式响应通过 SSE 转发，非流式直接返回
- **6. 错误重试：** 根据配置重试失败请求

## 4. 辅助模块

| 文件 | 职责 |
|------|------|
| `proxy_helpers.go` | HTTP 代理配置 |
| `cache_helpers.go` | 响应缓存 |
| `usage_helpers.go` | Token 使用量统计 |
| `logging_helpers.go` | 请求日志记录 |
| `thinking_providers.go` | 思考模式处理 |

## 5. 流式响应处理

执行器支持 Server-Sent Events (SSE) 流式响应：
- 逐块读取上游响应
- 实时转发给客户端
- 处理 UTF-8 边界问题（`internal/runtime/executor/` 中的流处理逻辑）
