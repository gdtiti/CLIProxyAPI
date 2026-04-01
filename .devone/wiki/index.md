# DevOne Wiki

## 目录

- [项目知识](projects/README.md)
- [技术知识](technologies/README.md)
- [模式与最佳实践](patterns/README.md)
- [决策记录](decisions/README.md)
- [参考资料](references/README.md)

## 最近同步

- 2026-03-29：新增 `Codex Responses` 流式分流知识，明确 `/v1/responses`、`/v1/responses/compact`、Responses WebSocket prewarm 与 Codex HTTP/WS executor 的行为边界，并记录“看起来像中途切非流式”的常见误判来源。
- 2026-03-23：新增 `AuthMaintenance` 运行态维护知识，补充 watcher 内部写删抑制/去抖、`usage_limit_reached` 禁用策略，以及与 `auth-runtime`、管理端 reload、distributed sync 的配置关系。

## 说明

此目录用于沉淀项目相关知识，只记录已经落地并经代码或测试证实的事实。
