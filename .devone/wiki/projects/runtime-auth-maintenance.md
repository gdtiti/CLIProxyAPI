# 运行态 Auth 维护与配置

## 目标

当前项目已经同时具备两层 auth 维护能力：

- `auth-runtime`：定义 `401` 连续失败的阈值和时间窗口。
- `auth-maintenance`：定义后台维护循环、队列节流、扩展删除条件，以及 `codex usage_limit_reached` 的禁用动作。

这两层现在是配合关系，不是替代关系。

## 当前行为边界

- `codex` 返回 `429` 且错误体包含 `usage_limit_reached` 时，auth 文件会被写成 `disabled=true`，并在运行态内存中禁用。
- 普通 `codex 429` 不会被误禁用，也不会被误删。
- `401` 不走立即删除，而是继续沿用 `auth-runtime` 的阈值与窗口策略，达到阈值后由后台维护队列删除 auth 文件。
- watcher 会对内部删除做短时抑制，对内部写入做去抖，避免维护线程和 watcher 彼此打架。

## 配置方法

```yaml
auth-runtime:
  unauthorized-delete-threshold: 3
  unauthorized-delete-window-seconds: 600

auth-maintenance:
  enable: true
  scan-interval-seconds: 30
  delete-interval-seconds: 5
  delete-status-codes: []
  delete-quota-exceeded: false
  quota-strike-threshold: 6
  disable-codex-usage-limit-reached: true
```

## 组合配置示例

下面这份示例适合当前项目的常见部署方式：

- `401` 连续失败仍按原有阈值窗口删除。
- `usage_limit_reached` 仍然只禁用，不删除。
- 多节点部署时继续复用此前已经落地的 `distributed-sync` 配置。

```yaml
auth-runtime:
  unauthorized-delete-threshold: 3
  unauthorized-delete-window-seconds: 600

auth-maintenance:
  enable: true
  scan-interval-seconds: 30
  delete-interval-seconds: 5
  delete-status-codes: []
  delete-quota-exceeded: false
  quota-strike-threshold: 6
  disable-codex-usage-limit-reached: true

distributed-sync:
  enabled: true
  node-id: "node-a"
  channel: "cliproxy:distributed-sync:events"
  poll-interval-seconds: 15
  redis:
    addr: "redis:6379"
```

单机部署时，可以直接省略 `distributed-sync` 整段。

## 字段说明

- `auth-runtime.unauthorized-delete-threshold`
  - 控制连续多少次 `401` 才触发删除。
- `auth-runtime.unauthorized-delete-window-seconds`
  - 控制上述计数的滚动窗口。
- `auth-maintenance.enable`
  - 控制后台维护循环与运行态 hook。
- `auth-maintenance.scan-interval-seconds`
  - 控制后台扫描 pending candidate 的频率。
- `auth-maintenance.delete-interval-seconds`
  - 控制队列执行间隔；backlog 大时会自动加速。
- `auth-maintenance.delete-status-codes`
  - 额外的立即删除状态码列表。
  - 默认空列表，避免把 generic `429` 误处理成删除。
- `auth-maintenance.delete-quota-exceeded`
  - 打开后允许根据 quota/backoff 状态生成删除 candidate。
- `auth-maintenance.quota-strike-threshold`
  - 当前项目使用 `Quota.BackoffLevel` 作为 strike 计数近似值。
- `auth-maintenance.disable-codex-usage-limit-reached`
  - 当前项目默认 `true`，用于固定 “usage_limit_reached 只禁用” 的行为。

## 使用建议

- 单机最小启用步骤：
  - 配置 `auth-runtime.*`
  - 配置 `auth-maintenance.*`
  - 重启服务并观察 auth 文件是否按预期进入 disable / delete
- 多节点部署步骤：
  - 先按此前 distributed sync 文档完成 `distributed-sync.*`
  - 再增加 `auth-runtime.*` 与 `auth-maintenance.*`
  - 确认本地节点文件变更后，其他节点能通过既有同步链路看到 reload 结果
- 只想保留当前项目既有行为时：
  - 保持 `delete-status-codes: []`
  - 保持 `disable-codex-usage-limit-reached: true`
- 希望扩展某些确定不可恢复的错误码为立即删除时：
  - 在 `delete-status-codes` 中显式列出，例如 `402`、`404`
  - 不建议在没有充分验证前把 generic `429` 放进去
- 只想关闭后台运行态维护，但保留管理端现有逻辑时：
  - 设置 `auth-maintenance.enable: false`

## 与此前改动的关系

- 管理端 `AuthRuntimeMaintenanceHook`
  - 仍然存在，继续保护 management API 相关链路。
- watcher 自动重载
  - 维护线程对 auth 文件的写入会被 watcher 去抖处理。
  - 维护线程对 auth 文件的删除会先做 `SuppressAuthPath`，避免 remove 事件被二次处理。
- 管理端运行时 reload
  - `ReloadConfigFromDisk` / `ReloadAuthFilesFromDisk` 仍由 watcher 包装器提供。
- distributed sync
  - 仍由 `distributed-sync.*` 控制。
  - auth 文件被本地维护线程删除/禁用后，最终仍通过现有 reload / store / sync 机制传播，不新增第二套同步通道。

## 关联文档

- distributed sync 联调与配置：
  - `docs/distributed-sync-two-node-setup_CN.md`
  - `docs/distributed-sync-checklist_CN.md`
  - `docs/distributed-sync-incident-matrix_CN.md`
