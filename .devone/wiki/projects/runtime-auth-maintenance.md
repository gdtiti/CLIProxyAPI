# 运行态 Auth 维护与配置

## 目标

当前项目已经同时具备两层 auth 维护能力：

- `auth-runtime`：定义 `401` 连续失败的阈值和时间窗口。
- `auth-maintenance`：定义后台维护循环、队列节流、扩展自动禁用条件，以及 `429 / quota` 保护性禁用动作。

这两层现在是配合关系，不是替代关系。

## 当前行为边界

- `codex` 返回 `429` 且错误体包含 `usage_limit_reached` 时，auth 文件会被写成 `disabled=true`，并在运行态内存中禁用。
- 任意 `429` 现在都会走保护性禁用，不再因为维护逻辑被删除。
- 已进入 `quota exceeded` / backoff 状态的账号现在也会走保护性禁用，不再因为维护逻辑被删除。
- `401` 不走立即删除，而是继续沿用 `auth-runtime` 的阈值与窗口策略，达到阈值后自动禁用 auth 文件与运行态 auth。
- 自动检查、自动判断、后台维护队列都只允许禁用；只有远程直接调用 management delete API 时才允许删除 auth 文件。
- watcher 会对内部写入做去抖，避免维护线程和 watcher 彼此打架。

## 最近新增能力

- `auth-maintenance.disable-status-codes`
  - 可按上游 HTTP 状态码直接禁用文件型 auth。
  - `429` 已改为内建保护性禁用，即使只出现在 `delete-status-codes` 中也不会删除。
  - 若同一状态码同时出现在 `disable-status-codes` 和 `delete-status-codes`，当前实现以禁用优先。
- `auth-maintenance.codex-max-request-count`
  - 可按累计已完成请求次数自动禁用文件型 `codex` auth。
  - `0` 表示关闭；达到阈值后只会写入 `disabled=true` 并禁用运行态 auth，不会自动删除文件。
- `auth-maintenance.codex-quota-check-request-interval`
  - 可按累计已完成请求次数周期性探测 Codex usage API。
  - 只有 probe 返回 `401` 时才自动禁用 auth；非 `401` 或 probe 失败都只保留账号，不触发删除。

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
  disable-status-codes: []
  delete-quota-exceeded: false
  quota-strike-threshold: 6
  disable-codex-usage-limit-reached: true
  codex-max-request-count: 0
  codex-quota-check-request-interval: 0
```

## 组合配置示例

下面这份示例适合当前项目的常见部署方式：

- `401` 连续失败仍按原有阈值窗口自动禁用。
- `usage_limit_reached` 仍然只禁用，不删除。
- `429` 与 `quota exceeded` 现在都会自动禁用，不删除。
- 可以为任意上游 HTTP 状态码配置“立即禁用”。
- 可以额外为 `codex` 文件 auth 配置累计请求次数上限；达到阈值后自动禁用 auth 文件。
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
  disable-status-codes: []
  delete-quota-exceeded: false
  quota-strike-threshold: 6
  disable-codex-usage-limit-reached: true
  codex-max-request-count: 0
  codex-quota-check-request-interval: 0

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
  - 控制连续多少次 `401` 才触发自动禁用。
- `auth-runtime.unauthorized-delete-window-seconds`
  - 控制上述计数的滚动窗口。
- `auth-maintenance.enable`
  - 控制后台维护循环与运行态 hook。
- `auth-maintenance.scan-interval-seconds`
  - 控制后台扫描 pending candidate 的频率。
- `auth-maintenance.delete-interval-seconds`
  - 控制队列执行间隔；backlog 大时会自动加速。
- `auth-maintenance.delete-status-codes`
  - 历史兼容字段名；当前用于声明额外的自动维护状态码。
  - 默认空列表；命中时会自动禁用账号而不是删除文件；其中 `429` 也会被保护性改写为禁用。
- `auth-maintenance.disable-status-codes`
  - 额外的立即禁用状态码列表。
  - 默认空列表；命中时会把 auth 文件写成 `disabled=true`，不删除文件。
  - 若与 `delete-status-codes` 同时包含同一状态码，当前实现以 disable 优先。
- `auth-maintenance.delete-quota-exceeded`
  - 历史兼容开关名，当前用于启用 quota/backoff 维护判断。
  - 命中 quota/backoff 状态时会禁用账号，不会删除文件。
- `auth-maintenance.quota-strike-threshold`
  - 当前项目使用 `Quota.BackoffLevel` 作为 strike 计数近似值。
- `auth-maintenance.disable-codex-usage-limit-reached`
  - 当前项目默认 `true`，用于固定 “usage_limit_reached 只禁用” 的行为。
- `auth-maintenance.codex-max-request-count`
  - 控制单个文件型 `codex` auth 的累计已完成请求次数上限。
  - `0` 表示关闭；`>0` 表示达到阈值后自动禁用 auth 文件。
- `auth-maintenance.codex-quota-check-request-interval`
  - 控制单个文件型 `codex` auth 每累计多少次已完成请求后，主动请求一次 Codex usage API。
  - `0` 表示关闭；`>0` 表示命中间隔时触发 probe。
  - 只有 probe 返回 `401` 时才自动禁用 auth；非 `401` 或 probe 失败都不会触发删除。
  - 若同时配置 `codex-max-request-count`，当前实现先判断累计次数禁用；达到上限后不会再继续 probe。

## 使用建议

- 单机最小启用步骤：
  - 配置 `auth-runtime.*`
  - 配置 `auth-maintenance.*`
  - 重启服务并观察 auth 文件是否按预期进入 disable
- 多节点部署步骤：
  - 先按此前 distributed sync 文档完成 `distributed-sync.*`
  - 再增加 `auth-runtime.*` 与 `auth-maintenance.*`
  - 确认本地节点文件变更后，其他节点能通过既有同步链路看到 reload 结果
- 只想保留当前项目既有行为时：
  - 保持 `delete-status-codes: []`
  - 保持 `disable-status-codes: []`
  - 保持 `disable-codex-usage-limit-reached: true`
- 希望某些上游错误码直接禁用账号时：
  - 在 `disable-status-codes` 中显式列出，例如 `403`
  - 若同一状态码同时也在 `delete-status-codes` 中，当前实现会优先禁用，不会删除
- 希望沿用历史兼容字段名来触发自动维护时：
  - 可以继续配置 `delete-status-codes`，但当前实现只会自动禁用，不会自动删除
  - `429` 不再属于删除候选，即使误配到这里也只会禁用
- 希望一个 `codex` 文件 auth 用满固定次数后自动停用时：
  - 设置 `codex-max-request-count: <正整数>`
  - 当前实现按“累计所有已完成请求”计数，不区分成功和失败
- 希望按固定间隔主动检查 `codex` 配额状态并在 `401` 时立刻停用时：
  - 设置 `codex-quota-check-request-interval: <正整数>`
  - 当前实现按“累计所有已完成请求”计数，到达间隔点时请求 `/backend-api/wham/usage`
  - 只有该 probe 返回 `401` 才自动禁用 auth；`200`、`429` 或请求失败都保留账号
- 只想关闭后台运行态维护，但保留管理端现有逻辑时：
  - 设置 `auth-maintenance.enable: false`

## 与此前改动的关系

- 管理端 `AuthRuntimeMaintenanceHook`
  - 仍然存在，继续保护 management API 相关链路。
- watcher 自动重载
  - 维护线程对 auth 文件的写入会被 watcher 去抖处理。
- 管理端运行时 reload
  - `ReloadConfigFromDisk` / `ReloadAuthFilesFromDisk` 仍由 watcher 包装器提供。
- distributed sync
  - 仍由 `distributed-sync.*` 控制。
  - auth 文件被本地维护线程禁用后，最终仍通过现有 reload / store / sync 机制传播，不新增第二套同步通道。

## 关联文档

- distributed sync 联调与配置：
  - `docs/distributed-sync-two-node-setup_CN.md`
  - `docs/distributed-sync-checklist_CN.md`
  - `docs/distributed-sync-incident-matrix_CN.md`
