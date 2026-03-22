- 结论 1:
  现有架构适合作为分布式部署底座，因为它已经具备 shared PG store、
  startup bootstrap、runtime reload hook、usage PG aggregate 这些关键骨
  架。
- 结论 2:
  用户提出的“数据库更新后写 Redis 版本号和操作日志，再让低版本节点增量
  修改”方向是对的，但必须升级为“事务内 version/outbox + Redis 发布 +
  gap fallback”的完整协议，不能只做裸 Redis key。
- 结论 3:
  统计窗口不应以 Redis 原始计数为唯一真相；更稳妥的做法是继续以 PG 小时
  聚合为真相，Redis 只缓存 3h/6h/24h/日/月等物化结果。
