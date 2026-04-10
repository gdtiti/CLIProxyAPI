- 现有实现:
  1. PostgreSQL store 在启动时会把 config 与 auth 从数据库同步到本地 spool
     目录。
  2. auth 上传链路会先把文件写到本地，再同步 PersistAuthFiles 到 store，
     成功后刷新当前节点运行态。
  3. config 修改主要走“写本地 config 文件 -> watcher reload -> 异步
     PersistConfig”链路。
  4. management API 已提供 `config.yaml/reload-from-store` 和
     `auth-files/reload-from-store` 手动回灌入口。
  5. usage 已有 PostgreSQL 持久化、小时聚合、按时间范围查询与
     `instance_id` 维度。
- 相关文档:
  `.devone/data/20260310-080813-auth-files-store-reload-backend-coordination/*`
  明确记录过“其它节点不会自动刷新”这一历史边界。
- 历史约束:
  当前系统默认依赖 PostgreSQL 作为 shared store，但没有 Redis 级分布式同
  步实现。
