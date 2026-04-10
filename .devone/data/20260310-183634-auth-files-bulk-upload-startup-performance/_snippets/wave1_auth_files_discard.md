### Wave 1

- 目标: 同步 auth_files 上传事务化逻辑到 worktree
- 改动:
  - 引入校验->写盘->store 持久化->注册与失败回滚的上传流程，并新增相关辅助函数
  - 补充分析：auth_files.go 中 UploadAuthFile 旧的直写盘/直注册路径应弃用，统一走 persistUploadedAuthFile
  - 补充分析：仍有多处‘直写盘/仅 saveTokenRecord + reloadAuthFile’旧路径未走 store 持久化（applyExcludedModelsPatchToAuthFile、PatchAuthFileStatus/PatchAuthFileFields、各 Request*Token），需在后续收敛/替换
- 结果:
  - worktree 已更新，等待测试与回归
  - 核对主工作区与 worktree 的 auth_files.go SHA256 一致，文件已在 worktree 内保留
