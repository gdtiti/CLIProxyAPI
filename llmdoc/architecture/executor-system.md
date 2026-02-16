# 执行器系统架构

## 1. 身份

- **是什么：** 提供商 API 请求执行引擎
- **目的：** 处理实际的 HTTP 请求发送、流式响应、错误重试，并在上游发送前完成必要的请求标准化

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
- **3. 请求标准化：** 发送前清理会破坏缓存稳定性的高频易变字段（见下文 Claude Code billing header 标准化）
- **4. Token 获取：** `token_helpers.go` 从认证管理器获取有效 Token
- **5. 发送请求：** 通过代理（如配置）发送 HTTP 请求
- **6. 处理响应：** 流式响应通过 SSE 转发，非流式直接返回
- **7. 错误重试：** 根据配置重试失败请求

## 4. Claude Code 缓存键稳定化（billing header 规范化）

### 问题现象

Claude Code / 插件会在 `system` 文本中注入 `x-anthropic-billing-header`，其中 `cch=...` 每次请求变化，导致同语义请求的缓存键抖动，命中率下降。

### 根因

请求负载中包含高频随机字段，并进入缓存键计算路径；即使其余提示词稳定，也会造成 prompt cache 难以复用。

### 处理策略

- 在 Claude 执行器上游发送前做请求标准化：`internal/runtime/executor/claude_executor.go:127-129`、`internal/runtime/executor/claude_executor.go:272-274`
- 规范化入口：`internal/runtime/executor/claude_executor.go:1050-1079` (`normalizeClaudeCodeBillingHeader`)
- 字段清理规则：`internal/runtime/executor/claude_executor.go:1081-1126` (`stripVolatileCCHFromBillingHeader`)
  - 移除易变 `cch=` 片段
  - 保留稳定字段（如 `cc_version`、`cc_entrypoint`）

### 生效范围

- 普通（非流式）请求路径：`internal/runtime/executor/claude_executor.go:86-230`
- 流式请求路径：`internal/runtime/executor/claude_executor.go:233-347`

### 验证与回归

- 规范化测试：`internal/runtime/executor/caching_verify_test.go:261-325` (`TestNormalizeClaudeCodeBillingHeader`)
- 关键断言：
  - 含 `cch=` 时被移除，保留稳定字段
  - 不相关 system 文本保持不变
  - 不含 `cch=` 时 payload 不变

## 5. 辅助模块

| 文件 | 职责 |
|------|------|
| `proxy_helpers.go` | HTTP 代理配置 |
| `cache_helpers.go` | 响应缓存 |
| `usage_helpers.go` | Token 使用量统计 |
| `logging_helpers.go` | 请求日志记录 |
| `thinking_providers.go` | 思考模式处理 |

## 6. 流式响应处理

执行器支持 Server-Sent Events (SSE) 流式响应：
- 逐块读取上游响应
- 实时转发给客户端
- 处理 UTF-8 边界问题（`internal/runtime/executor/` 中的流处理逻辑）
