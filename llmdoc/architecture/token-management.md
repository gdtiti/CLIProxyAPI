# Token 管理架构

## 1. 身份

- **是什么：** Token 生命周期管理系统
- **目的：** 管理 OAuth Token 的存储、刷新、缓存和速率限制

## 2. 核心组件

- `internal/auth/kiro/background_refresh.go`: 后台 Token 刷新管理器
- `internal/auth/kiro/refresh_manager.go`: 刷新调度和协调
- `internal/auth/kiro/rate_limiter.go`: API 请求速率限制
- `internal/auth/kiro/cooldown.go`: 配额冷却管理
- `internal/auth/codex/openai_auth.go`: Codex OAuth 刷新与重试控制
- `internal/runtime/executor/codex_executor.go`: Codex 刷新失败后的凭证状态落盘入口
- `internal/cache/signature_cache.go`: 签名缓存

## 3. 执行流程（LLM 检索地图）

- **1. Token 请求：** 执行器请求有效 Token
- **2. 缓存检查：** 检查内存缓存是否有有效 Token
- **3. 过期检查：** 判断 Token 是否即将过期（10 分钟内）
- **4. 刷新触发：** 过期前自动触发刷新流程
- **5. 刷新分类：** 可恢复错误继续重试；不可恢复错误立即终止 - `internal/auth/codex/openai_auth.go:197-203`、`internal/auth/codex/openai_auth.go:256-285`
- **6. 存储更新：** 成功刷新后写入新 Token；若不可恢复则写入禁用状态 - `internal/runtime/executor/codex_executor.go:562-609`

## 4. Codex 不可恢复刷新错误（refresh_token_reused）

触发条件：
- 刷新请求返回 HTTP 401，且响应体包含 `refresh_token_reused` - `internal/auth/codex/openai_auth.go:197-201`

系统行为：
- 认证层返回哨兵错误 `ErrRefreshTokenReused`，标记为不可恢复 - `internal/auth/codex/openai_auth.go:23`、`internal/auth/codex/openai_auth.go:199-201`
- 重试层识别该错误后立即停止后续 attempt，不再指数退避重试 - `internal/auth/codex/openai_auth.go:276-278`
- 执行器刷新入口命中该错误后自动禁用凭证并返回 `nil` error，允许上层 manager 持久化禁用状态并停止后续刷新 - `internal/runtime/executor/codex_executor.go:579-587`

禁用状态写入字段：
- `auth.Disabled = true` - `internal/runtime/executor/codex_executor.go:580`
- `auth.Status = disabled` - `internal/runtime/executor/codex_executor.go:581`
- `auth.StatusMessage = "disabled: refresh token reused, sign in again"` - `internal/runtime/executor/codex_executor.go:582`
- `auth.Metadata["refresh_disabled_reason"] = "refresh_token_reused"` - `internal/runtime/executor/codex_executor.go:586`

## 5. 速率限制

`rate_limiter.go` 实现请求速率控制：
- 令牌桶算法
- 按提供商/凭据分别限制
- 超限时返回 429 错误

## 6. 冷却机制

`cooldown.go` 处理配额超限：
- 记录凭据冷却状态
- 自动切换到其他可用凭据
- 冷却期结束后恢复使用

## 7. 恢复方式

- `refresh_token_reused` 为不可恢复刷新错误，不能通过重试恢复。
- 运维/用户需要重新登录（重新完成 Codex OAuth）以生成新的 refresh token，然后替换已禁用凭证。

## 8. 指标收集

`metrics.go` 收集运行时指标：
- Token 刷新次数
- 请求成功/失败率
- 配额使用情况
