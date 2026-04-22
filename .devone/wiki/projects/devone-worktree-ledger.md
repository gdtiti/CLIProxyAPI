# DevOne Worktree 台账

> 自动维护。记录每个 DevOne task worktree 的创建时间、收尾状态、收尾时间、cleanup 状态和解决的问题。
> 当最终收尾时如果 worktree 保留，必须至少更新最近收尾状态、最近收尾时间、cleanup 状态和解决的问题。

## 条目
### 20260406-224921-ghcr-only-main-publish

- 任务名称: ghcr-only-main-publish
- 任务包: .devone/data/20260406-224921-ghcr-only-main-publish
- worktree 目录: .devone/worktree/20260406-224921-ghcr-only-main-publish
- worktree 分支: devone/20260406-224921-ghcr-only-main-publish
- 开发端口: 37206
- 创建时间: 20260406-230000
- 最近收尾状态: blocked
- 最近收尾时间: 20260407-003503
- cleanup 状态: kept
- 目标分支: main
- 解决的问题: docker-image workflow 已切到 GHCR 且认证回收站缺失源码已补齐
- 备注: 远端 devone 分支与 workflow 已成功，但当前 fixed closeout 同时受两处条件阻塞：主工作区存在既有脏改动，且本 worktree 里还有 `.tmp-go-build/`、`.tmp-run-24037097490-logs/` 两个未提交临时目录；当前保留 worktree 等待后续清理后再继续。

### 20260409-120036-disabled-auth-auto-refresh-recovery

- 任务名称: disabled-auth-auto-refresh-recovery
- 任务包: .devone/data/20260409-120036-disabled-auth-auto-refresh-recovery
- worktree 目录: .devone/worktree/20260409-120036-disabled-auth-auto-refresh-recovery
- worktree 分支: devone/20260409-120036-disabled-auth-auto-refresh-recovery
- 开发端口: 35983
- 创建时间: 20260409-131406
- 最近收尾状态: merged
- 最近收尾时间: 20260409-180709
- cleanup 状态: waiting_user_confirmation
- 目标分支: CLIProxyAPIPlus-gdtiti
- 解决的问题: 文件级 `disabled=true` 的 `codex` auth 不会进入 `core auth auto-refresh`，因为 `sdk/cliproxy/auth/conductor.go` 的 `Manager.shouldRefresh()` 开头直接跳过 disabled auth。
- 备注: 已在当前 worktree 合并 gdtiti/CLIProxyAPIPlus-gdtiti，并推送 gdtiti/devone/20260409-120036-disabled-auth-auto-refresh-recovery 与 gdtiti/CLIProxyAPIPlus-gdtiti；等待用户决定是否清理 worktree

### 20260408-205635-usage-statistics-visibility-fix

- 任务名称: usage-statistics-visibility-fix
- 任务包: .devone/data/20260408-205635-usage-statistics-visibility-fix
- worktree 目录: .devone/worktree/20260408-205635-usage-statistics-visibility-fix
- worktree 分支: devone/20260408-205635-usage-statistics-visibility-fix
- 开发端口: 34985
- 创建时间: 20260408-205950
- 最近收尾状态: blocked
- 最近收尾时间: 20260409-010529
- cleanup 状态: kept
- 目标分支: main
- 解决的问题: usage 统计页在 PostgreSQL 持久化场景与聚合-only 回包场景下均已恢复显示。
- 备注: 真实 worktree-merge --confirm 在合并 origin/main 时发生 13 个冲突项；当前保留冲突现场，待用户决定继续解冲突还是中止 merge。

### 20260405-015746-codex-smart-auth-refresh-refactor

- 任务名称: codex-smart-auth-refresh-refactor
- 任务包: .devone/data/20260405-015746-codex-smart-auth-refresh-refactor
- worktree 目录: .devone/worktree/20260405-015746-codex-smart-auth-refresh-refactor
- worktree 分支: devone/20260405-015746-codex-smart-auth-refresh-refactor
- 开发端口: 37183
- 创建时间: 20260405-020146
- 最近收尾状态: merged
- 最近收尾时间: 20260405-115912
- cleanup 状态: kept
- 目标分支: CLIProxyAPIPlus-gdtiti
- 解决的问题: Codex 后台 refresh 热度失真与 SimHash resident bridge 已通过临时集成 worktree 合入 gdtiti 主线
- 备注: 按用户要求未修改当前主工作区；使用 .devone/worktree/20260405-gdtiti-closeout 从 gdtiti/CLIProxyAPIPlus-gdtiti fast-forward 合并 gdtiti/devone/20260405-015746-codex-smart-auth-refresh-refactor，并将 302ddd11..cf7831ba 推送回远端主线；共享 worktree 继续保留。

### 20260402-095726-配置暂停分布式记录与认证回收站清理能力

- 任务名称: 配置暂停分布式记录与认证回收站清理能力
- 任务包: .devone/data/20260402-095726-配置暂停分布式记录与认证回收站清理能力
- worktree 目录: .devone/worktree/20260402-095726-配置暂停分布式记录与认证回收站清理能力
- worktree 分支: devone/20260402-095726
- 开发端口: 31076
- 创建时间: 20260402-100416
- 最近收尾状态: blocked
- 最近收尾时间: 20260402-140913
- cleanup 状态: kept
- 目标分支: main
- 解决的问题: 已完成暂停 distributed recording、auth recycle bin、distributed cleanup 与 usage history cleanup 功能并通过验收，但收尾合并被主工作区脏改动阻断
- 备注: 20260402-140913 尝试 merge dry-run into=main 时失败：主工作区存在已修改 tracked 文件；本轮保留 worktree，待主工作区清理后重试 merge 与 cleanup

### 20260401-232630-吸收-CassiopeiaCode-优化能力并分阶段合并主线

- 任务名称: 吸收 CassiopeiaCode 优化能力并分阶段合并主线
- 任务包: .devone/data/20260401-232630-吸收-CassiopeiaCode-优化能力并分阶段合并主线
- worktree 目录: .devone/worktree/20260401-232630-吸收-CassiopeiaCode-优化能力并分阶段合并主线
- worktree 分支: devone/20260401-232630-CassiopeiaCode
- 开发端口: 36191
- 创建时间: 20260401-233612
- 最近收尾状态: accepted
- 最近收尾时间: 20260404-141658
- cleanup 状态: kept-by-request
- 目标分支: gdtiti/CLIProxyAPIPlus-gdtiti
- 解决的问题: 已完成反馈式账号路由、translator fast path / request cache、request-audit hook、Codex mimic/header 拟真与 runtime auth maintenance 保护性修正，并补齐阶段 2/3/4 的主线 merge/push 闭环证据。
- 备注: 20260404-141658 已在 worktree 合并本地 `main` 生成 `302ddd11bf5bc4668092b19ffa9f40d4f8099d97`，并推送 `gdtiti/CLIProxyAPIPlus-gdtiti`（`e173d104..302ddd11`）；主工作区脏改动按用户要求保留，worktree 暂不删除
