- 技术约束:
  现有实现以 PostgreSQL store 为配置与 auth 的持久化后端，并在本地 spool
  目录保留镜像文件；auth 上传链路是同步 PersistAuthFiles，config 修改链
  路主要依赖 watcher 异步 PersistConfig。
- 业务约束:
  多节点部署不能牺牲当前单节点热加载能力，且管理端“成功返回”的语义必须
  足够清晰。
- 外部依赖:
  PostgreSQL、Redis、节点唯一标识、可靠时钟/UTC 时间、后续多节点集成验
  证环境。
