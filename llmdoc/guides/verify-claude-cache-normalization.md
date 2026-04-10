# 验证 Claude Code 缓存键标准化修复

## 目标

验证 Claude Code / 插件注入的 `x-anthropic-billing-header` 中易变 `cch=...` 已在发送上游前被清理，且普通请求与流式请求均覆盖，不破坏其他 system 指令语义。

## 步骤

### Step 1: 确认标准化逻辑接入点

检查 Claude 执行器在两条请求路径都调用了标准化函数：

- 非流式路径：`internal/runtime/executor/claude_executor.go:127-129`
- 流式路径：`internal/runtime/executor/claude_executor.go:272-274`
- 规范化实现：`internal/runtime/executor/claude_executor.go:1050-1126`

验证: `go test ./internal/runtime/executor -run TestNormalizeClaudeCodeBillingHeader -v`

### Step 2: 运行针对性测试

关注测试用例 `TestNormalizeClaudeCodeBillingHeader`：

- 用例位置：`internal/runtime/executor/caching_verify_test.go:261-325`
- 预期行为：
  - 移除 `cch=`，保留稳定字段（例如 `cc_version`、`cc_entrypoint`）
  - 非 billing header 的 system 文本保持不变
  - 不含 `cch=` 时 payload 保持原样

验证: `go test ./internal/runtime/executor -run TestNormalizeClaudeCodeBillingHeader -v`

### Step 3: 回归编译与包级测试

在 executor 范围做编译/测试回归，确认未引入编译错误或缓存相关回归。

验证:
- `go test ./internal/runtime/executor`
- （可选全量）`go test ./...`

## 故障排除

| 问题 | 解决方案 |
| ---- | -------- |
| 测试失败：仍出现 `cch=` | 检查 `stripVolatileCCHFromBillingHeader` 是否仅在命中 `x-anthropic-billing-header` 且包含 `cch=` 时处理：`internal/runtime/executor/claude_executor.go:1081-1110` |
| 其他 system 文本被改写 | 对照 `normalizeClaudeCodeBillingHeader` 的 `type == text` 与“仅变更目标行”逻辑：`internal/runtime/executor/claude_executor.go:1058-1076` |
| 流式路径未生效 | 确认 `ExecuteStream` 中在上游请求构建前调用标准化：`internal/runtime/executor/claude_executor.go:233-295` |
| 误删非易变字段 | 仅应清理已知易变 `cch=`，保留其它键值对（如 `cc_version`、`cc_entrypoint`）：`internal/runtime/executor/claude_executor.go:1099-1118` |

## 注意事项

- 本修复仅清理已知易变字段 `cch=`，避免扩大匹配范围破坏其他 system 指令语义。
- 若后续出现新的高频波动字段，应先补充测试，再最小化扩展清理规则。
