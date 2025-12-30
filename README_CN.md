# CLIProxyAPI Plus

[English](README.md) | 中文

这是 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的 Plus 版本，在原有基础上增加了第三方供应商的支持。

所有的第三方供应商支持都由第三方社区维护者提供，CLIProxyAPI 不提供技术支持。如需取得支持，请与对应的社区维护者联系。

该 Plus 版本的主线功能与主线功能强制同步。

## 与主线版本版本差异

- 新增 GitHub Copilot 支持（OAuth 登录），由[em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)提供
- 新增 Kiro (AWS CodeWhisperer) 支持 (OAuth 登录), 由[fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration)、[Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)提供

## 开发

### 快速开始

使用提供的开发脚本执行常见任务：

**Windows:**
```cmd
dev.bat help
```

**Linux/macOS:**
```bash
./dev.sh help
```

### 文档

- **[开发脚本指南](docs/DEV_SCRIPTS.md)** - dev.bat/dev.sh 完整使用指南
- **[故障排除指南](docs/TROUBLESHOOTING.md)** - 常见问题解决方案，包括 Gemini CLI "ALL" 错误
- **[SDK 文档](docs/sdk-usage.md)** - SDK 集成指南

### 常用命令

```bash
# 安装依赖
dev.bat install

# 启动开发服务器
dev.bat dev

# 构建生产版本
dev.bat build

# 运行测试
dev.bat test

# 登录供应商
dev.bat login

# 检查配额
dev.bat quota
```

## 贡献

该项目仅接受第三方供应商支持的 Pull Request。任何非第三方供应商支持的 Pull Request 都将被拒绝。

如果需要提交任何非第三方供应商支持的 Pull Request，请提交到主线版本。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。