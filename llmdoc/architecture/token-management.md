# Token 管理架构

## 1. 身份

- **是什么：** Token 生命周期管理系统
- **目的：** 管理 OAuth Token 的存储、刷新、缓存和速率限制

## 2. 核心组件

- `internal/auth/kiro/background_refresh.go`: 后台 Token 刷新管理器
- `internal/auth/kiro/refresh_manager.go`: 刷新调度和协调
- `internal/auth/kiro/rate_limiter.go`: API 请求速率限制
- `internal/auth/kiro/cooldown.go`: 配额冷却管理
- `internal/cache/signature_cache.go`: 签名缓存

## 3. 执行流程（LLM 检索地图）

- **1. Token 请求：** 执行器请求有效 Token
- **2. 缓存检查：** 检查内存缓存是否有有效 Token
- **3. 过期检查：** 判断 Token 是否即将过期（10 分钟内）
- **4. 刷新触发：** 过期前自动触发刷新流程
- **5. 存储更新：** 新 Token 写入存储后端

## 4. 速率限制

`rate_limiter.go` 实现请求速率控制：
- 令牌桶算法
- 按提供商/凭据分别限制
- 超限时返回 429 错误

## 5. 冷却机制

`cooldown.go` 处理配额超限：
- 记录凭据冷却状态
- 自动切换到其他可用凭据
- 冷却期结束后恢复使用

## 6. 指标收集

`metrics.go` 收集运行时指标：
- Token 刷新次数
- 请求成功/失败率
- 配额使用情况
