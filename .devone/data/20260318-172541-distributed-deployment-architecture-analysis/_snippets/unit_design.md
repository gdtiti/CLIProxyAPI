### UT-001

- 目标:
  验证 config/auth 写入事务与 version/outbox 同步提交。
- 覆盖函数/模块:
  未来的 store write service / outbox writer。
- 输入:
  单次配置或凭证写请求。
- 预期输出:
  DB 记录、version、outbox 三者同时可见或同时不可见。
- 失败表现:
  任一对象提交成功但事件/版本缺失。

### UT-002

- 目标:
  验证节点消费重复事件时不会重复应用。
- 覆盖函数/模块:
  future event consumer / apply service。
- 输入:
  相同 event id 重复投递。
- 预期输出:
  本地版本和 runtime 状态只前进一次。
- 失败表现:
  重复 patch、错误回滚或版本倒退。

### UT-003

- 目标:
  验证版本跳跃时会自动降级为全量 reload-from-store。
- 覆盖函数/模块:
  future gap detector / fallback coordinator。
- 输入:
  本地版本 10，收到 version 12 事件。
- 预期输出:
  触发 full reload，最终版本追平。
- 失败表现:
  继续做错误的增量 apply。

### UT-004

- 目标:
  验证 3h / 6h / 24h / 日 / 月窗口统计计算。
- 覆盖函数/模块:
  future stats cache builder。
- 输入:
  覆盖跨小时、跨日、跨月的固定 usage aggregates。
- 预期输出:
  各窗口数值与 PG 聚合一致。
- 失败表现:
  边界数据重复计算或遗漏。
