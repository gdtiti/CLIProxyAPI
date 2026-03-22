- 推荐方案:
  采用“PostgreSQL 作为真相源 + Redis 作为事件总线与短期缓存 + 节点本地
  spool/runtime 作为派生态”的三层设计。对配置与凭证使用“版本号 +
  outbox 事件 + 增量拉取 + 全量回灌兜底”；对统计使用“PG 小时聚合 +
  Redis 窗口缓存/materialized view”。
- 推荐原因:
  这条路线与现有代码最贴合：启动已经会从 PG bootstrap 到本地，auth 与
  config 已有 reload-from-store / watcher 热加载入口，usage 也已有 PG
  hourly aggregate 基础。
