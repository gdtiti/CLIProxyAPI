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
