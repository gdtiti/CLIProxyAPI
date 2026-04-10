# Git 约定

## 1. 提交消息格式

```
<type>: <subject>

[optional body]
```

### 类型（type）

- **feat:** 新功能
- **fix:** Bug 修复
- **docs:** 文档更新
- **refactor:** 代码重构
- **test:** 测试相关
- **chore:** 构建/工具变更

### 示例

```
feat: add Kiro OAuth web authentication

- Add browser-based OAuth flow
- Support AWS Builder ID login
- Add token import from Kiro IDE
```

## 2. 分支策略

- **main:** 稳定发布分支
- **feature/*:** 功能开发分支
- **fix/*:** Bug 修复分支
- **CLIProxyAPIPlus-*:** 社区维护的特性分支

## 3. Pull Request 规范

- 标题简洁描述变更
- 描述中说明变更原因和影响
- 仅接受第三方提供商相关的 PR
- 非提供商相关变更请提交到主线仓库

## 4. 版本标签

- 语义化版本：`vMAJOR.MINOR.PATCH`
- 与主线版本保持同步
