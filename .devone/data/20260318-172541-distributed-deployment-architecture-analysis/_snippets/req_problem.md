- 当前问题:
  需要评估 CLIProxyAPI Plus 现有“共享 PostgreSQL store + 本地 spool +
  watcher 热加载”架构，是否适合扩展为多节点部署，并补齐跨节点配置、
  凭证、统计信息同步机制。
- 影响对象:
  使用共享 PostgreSQL / Redis 的所有服务节点、管理端配置修改链路、auth
  file 上传链路、usage 统计查询链路。
- 触发背景:
  用户希望多个节点连接同一套数据库和 Redis；一个节点更新数据库后，其他
  节点能够基于版本号和操作日志做增量同步；同时希望提供月、日、24h、6h、
  3h 等统计窗口。
- 为什么现在处理:
  代码里已经具备 PostgreSQL store、auth reload-from-store、PG usage
  聚合等基础能力；现在适合统一收敛“真相源、事件传播、缓存窗口”三件事。
