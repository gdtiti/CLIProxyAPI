# iFlow 上游反向代理稳定性修复设计

**日期**: 2026-02-20  
**作者**: CLIProxyAPI Team  
**状态**: 已批准，待实施

## 背景

CLIProxyAPI 反代 iFlow 的 glm-5 模型时遇到稳定性问题，主要表现为：

1. **偶发 406 错误**: `{"error":{"message":"status 406","type":"invalid_request_error"}}`
2. **偶发长时间无响应**: 看起来像卡住，通常再发一句才继续

## 根因分析

### 1. 请求头与 iFlow 官方实现有差异
- **显式 `Accept: text/event-stream`**: 在某些上游节点会触发间歇性 406
- **`conversation-id` 头行为不一致**: 会放大兼容性问题

### 2. 流式链路容错不足
- 首包前失败时缺少自动补偿，客户端表现为"无响应"

### 3. Auth 状态机过于激进
- 短时上游错误触发冷却，可能把可用 auth 池打空，出现 `auth_unavailable`

## 设计目标

1. 与 iFlow 官方实现对齐请求头策略
2. 增加 406 专项兜底机制
3. 增强流式稳定性
4. 修正 auth 可用性策略

## 详细设计

### 1. 请求头对齐 (iflow_executor.go)

#### 1.1 移除显式 Accept 头设置
```go
// 修改 applyIFlowHeaders 函数
// 流式请求不再设置 Accept: text/event-stream
// 让 HTTP 客户端使用默认行为
```

#### 1.2 添加 conversation-id 头
```go
// 始终设置 conversation-id 头
// 格式: "conversation-id": <uuid>
// 即使是空值也要带键，与 iFlow 官方行为一致
```

### 2. 406 专项兜底 (iflow_executor.go)

#### 2.1 406 检测与重试
```go
// 在 Execute 和 ExecuteStream 中增加 406 处理
// 遇到 406 时，自动进行一次"无签名头重试"
// 重试时移除 x-iflow-signature 头
// 仅重试一次，防止无限循环
```

#### 2.2 重试条件
- 状态码: 406
- 错误类型: `invalid_request_error`
- 重试次数: 仅 1 次
- 重试策略: 移除签名头后重试

### 3. 流式稳定性增强 (iflow_executor.go)

#### 3.1 首包前失败重试
```go
// 在 ExecuteStream 中增加首包超时检测
// 如果在指定时间内未收到首包，自动重试一次
// 超时时间: 30 秒（可配置）
```

#### 3.2 心跳机制
```go
// 长请求期间持续发送心跳
// 防止连接被中间件断开
// 心跳间隔: 15 秒（可配置）
```

### 4. Auth 可用性策略修正 (conductor.go)

#### 4.1 错误码区分
```go
// auth_unavailable 时明确返回 503
// 区分客户端错误 (4xx) 和服务端错误 (5xx)
// 仅服务端错误触发 auth 冷却
```

#### 4.2 冷却策略调整
```go
// 仅在有替代 auth 时才冷却当前 auth
// 单 auth 场景不再自我熔断
// 增加 406 错误码的特殊处理
```

#### 4.3 406 错误处理
```go
// 406 不触发 auth 冷却
// 因为 406 通常是请求格式问题，不是 auth 问题
```

## 实施范围

### 修改文件
1. `internal/runtime/executor/iflow_executor.go`
2. `sdk/cliproxy/auth/conductor.go`

### 新增配置项
```yaml
iflow:
  # 首包超时时间（秒）
  first_packet_timeout: 30
  # 心跳间隔（秒）
  heartbeat_interval: 15
  # 是否启用 406 重试
  retry_on_406: true
```

## 预期效果

1. **406 发生率明显下降**: 通过请求头对齐和 406 兜底机制
2. **"偶发无响应"大幅减少**: 通过首包超时重试和心跳机制
3. **错误更明确**: auth_unavailable 返回 503，便于监控与告警
4. **单 auth 场景更稳定**: 不再因临时错误导致服务不可用

## 测试计划

1. **单元测试**: 测试 406 重试逻辑、首包超时逻辑
2. **集成测试**: 模拟 iFlow 上游各种错误场景
3. **压力测试**: 验证高并发下 auth 冷却策略
4. **监控验证**: 确认 503 错误正确上报

## 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 移除 Accept 头可能影响其他上游 | 中 | 仅针对 iFlow executor 修改 |
| 406 重试增加延迟 | 低 | 仅重试一次，且 406 本身已是错误 |
| 心跳机制增加负载 | 低 | 心跳间隔可配置，默认 15 秒 |

## 后续优化

1. 考虑将 406 重试机制通用化，支持其他 executor
2. 增加更细粒度的 auth 健康检查
3. 考虑实现自适应超时机制

## 参考

- iFlow 官方 API 文档
- CLIProxyAPI 现有 executor 实现模式
- 生产环境 glm-5 稳定性问题报告
