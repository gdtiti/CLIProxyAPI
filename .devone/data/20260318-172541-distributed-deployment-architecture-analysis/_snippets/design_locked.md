- 入口模块:
  `cmd/server/main.go`、`internal/store/postgresstore.go`、
  `internal/store/postgres_reload.go`、
  `internal/api/handlers/management/auth_files.go`、
  `internal/api/handlers/management/config_basic.go`、
  `internal/api/handlers/management/reload_from_store.go`、`internal/usage/*`。
- 关键调用链:
  `main -> NewPostgresStore -> Bootstrap`；
  `PutConfigYAML -> watcher.reloadConfigIfChanged -> persistConfigAsync ->
  PersistConfig`；
  `persistUploadedAuthFile -> persistUploadedAuthToStore -> registerAuthFromFile`；
  `PostReloadConfigFromStore / PostReloadAuthFilesFromStore -> runtime reload
  hook`；
  `NewPersistentLoggerPlugin -> WriteQueue.doFlush -> UpsertAggregates ->
  QuerySnapshot`。
- 数据流:
  1. 写请求先写 PostgreSQL 真相源，并在同一事务内写 outbox/version。
  2. publisher 将已提交 outbox 推送到 Redis Stream。
  3. 节点消费事件，按对象类型和版本增量拉取 config/auth 变化。
  4. 版本连续且校验通过则局部 apply；发现缺口、乱序或失败则调用全量
     reload-from-store。
  5. 统计查询优先读 Redis 窗口缓存，miss 时从 PG 小时聚合重算并回填
     Redis。
- 错误处理:
  事件消费必须幂等；同一事件可重放；发现版本跳跃、checksum 不匹配或
  apply 失败时，降级为全量回灌；Redis 不可用时允许退回 PG 轮询，但不能
  把 Redis 视为唯一真相源。
- 配置变更:
  需要新增 `node_id`、Redis 连接配置、同步开关、polling fallback 间隔、
  stats cache TTL、event consumer group 名称、是否启用 outbox
  publisher 等配置。
