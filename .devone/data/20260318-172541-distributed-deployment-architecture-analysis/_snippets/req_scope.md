### 本次必须完成

- 梳理 PostgreSQL store、management reload、watcher 持久化、usage PG
  聚合现状。
- 评估“DB + Redis 版本号/操作日志”方案与现有代码的适配度。
- 给出推荐架构、非目标和分阶段实施建议。

### 本次明确不做

- 不修改业务代码，不引入 Redis 依赖，不落地多节点联调。
- 不承诺现有系统已经具备自动跨节点热同步能力。
