# 运行态 Auth 维护模式

## 核心模式

- 策略和机制分离：
  - `auth-runtime` 保存 `401` 阈值策略。
  - `auth-maintenance` 保存后台维护机制。
- 文件路径分组：
  - 多个 auth ID 共用一个 auth 文件时，维护动作按路径聚合，避免重复写删。
- watcher 自抑制：
  - 内部删除前先 `SuppressAuthPath`。
  - 内部写入走 pending write debounce，避免 burst write 触发多次增量处理。

## 当前项目特化

- `usage_limit_reached` 明确走 disable，不走 delete。
- Gemini virtual project 暂不进入 `Service` 层的删除路径，避免把局部 project 清理误升级成整文件删除。
- `core auth auto-refresh` 与运行态维护是并行机制：
  - auto-refresh 固定周期扫 snapshot；
  - auth-maintenance 处理 401 / 429 / quota / request-count 等自动禁用策略。
- 当前 refresh 决策对大多数 provider 仍然只看 auth 状态、`NextRefreshAfter`、过期时间与 provider refresh lead。
- 对 `codex` 来说，项目已改为“lead 判断 + warm/resident gate”双条件：
  - cold auth 不参与后台 refresh；
  - warm auth 由真实成功请求驱动；
  - resident auth 现已由 SimHash resident hint 桥接到 `ResidentUntil`，用于承接虚拟池/常驻池信号。
- 虚拟池与 refresh 的边界保持分离：
  - selector 私有 admission pool 不直接成为 refresh 判定输入；
  - selector 只输出 `BackgroundRefreshHints` 这类抽象信号；
  - manager 负责把 hint 写入 `WarmUntil/ResidentUntil` 等通用热度字段；
  - `shouldRefresh()` 继续只消费 `Auth` 运行态热度，不读取 selector 私有成员结构。
