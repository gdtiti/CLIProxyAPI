# CLIProxyAPI Plus

[English](README.md) | 中文

这是 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的 Plus 版本，在原有基础上增加了第三方供应商的支持。

所有的第三方供应商支持都由第三方社区维护者提供，CLIProxyAPI 不提供技术支持。如需取得支持，请与对应的社区维护者联系。

该 Plus 版本的主线功能与主线功能强制同步。

## 与主线版本版本差异

- 新增 GitHub Copilot 支持（OAuth 登录），由[em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)提供
- 新增 Kiro (AWS CodeWhisperer) 支持 (OAuth 登录), 由[fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration)、[Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)提供

## 新增功能 (Plus 增强版)

- **OAuth Web 认证**: 基于浏览器的 Kiro OAuth 登录，提供美观的 Web UI
- **请求限流器**: 内置请求限流，防止 API 滥用
- **后台令牌刷新**: 过期前 10 分钟自动刷新令牌
- **监控指标**: 请求指标收集，用于监控和调试
- **设备指纹**: 设备指纹生成，增强安全性
- **冷却管理**: 智能冷却机制，应对 API 速率限制
- **用量检查器**: 实时用量监控和配额管理
- **模型转换器**: 跨供应商的统一模型名称转换
- **UTF-8 流处理**: 改进的流式响应处理

## Kiro 认证

### 命令行登录

> **注意：** 由于 AWS Cognito 限制，Google/GitHub 登录不可用于第三方应用。

**AWS Builder ID**（推荐）：

```bash
# 设备码流程
./CLIProxyAPI --kiro-aws-login

# 授权码流程
./CLIProxyAPI --kiro-aws-authcode
```

**从 Kiro IDE 导入令牌：**

```bash
./CLIProxyAPI --kiro-import
```

获取令牌步骤：

1. 打开 Kiro IDE，使用 Google（或 GitHub）登录
2. 找到令牌文件：`~/.kiro/kiro-auth-token.json`
3. 运行：`./CLIProxyAPI --kiro-import`

**AWS IAM Identity Center (IDC)：**

```bash
./CLIProxyAPI --kiro-idc-login --kiro-idc-start-url https://d-xxxxxxxxxx.awsapps.com/start

# 指定区域
./CLIProxyAPI --kiro-idc-login --kiro-idc-start-url https://d-xxxxxxxxxx.awsapps.com/start --kiro-idc-region us-west-2
```

**附加参数：**

| 参数 | 说明 |
|------|------|
| `--no-browser` | 不自动打开浏览器，打印 URL |
| `--no-incognito` | 使用已有浏览器会话（Kiro 默认使用无痕模式），适用于需要已登录浏览器会话的企业 SSO 场景 |
| `--kiro-idc-start-url` | IDC Start URL（`--kiro-idc-login` 必需） |
| `--kiro-idc-region` | IDC 区域（默认：`us-east-1`） |
| `--kiro-idc-flow` | IDC 流程类型：`authcode`（默认）或 `device` |

### 网页端 OAuth 登录

访问 Kiro OAuth 网页认证界面：

```
http://your-server:8080/v0/oauth/kiro
```

提供基于浏览器的 Kiro (AWS CodeWhisperer) OAuth 认证流程，支持：
- AWS Builder ID 登录
- AWS Identity Center (IDC) 登录
- 从 Kiro IDE 导入令牌

## 开发

### 快速开始

**交互式菜单（推荐）：**

直接运行脚本即可获得交互式菜单：

```cmd
dev.bat
```

**或直接使用命令：**

```bash
dev.bat help    # 显示所有命令
dev.bat dev     # 启动开发服务器
dev.bat build   # 构建生产版本
```

### 文档

- **[开发脚本指南](docs/DEV_SCRIPTS.md)** - dev.bat/dev.sh 完整使用指南
- **[故障排除指南](docs/TROUBLESHOOTING.md)** - 常见问题解决方案，包括 Gemini CLI "ALL" 错误（已修复）
- **[Docker 分支构建指南](docs/DOCKER_BRANCH_BUILD.md)** - 从开发分支构建和使用 Docker 镜像
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

## Docker 快速部署

### 一键部署

```bash
# 创建部署目录
mkdir -p ~/cli-proxy && cd ~/cli-proxy

# 创建 docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    container_name: cli-proxy-api-plus
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api
      - ./logs:/CLIProxyAPI/logs
    restart: unless-stopped
EOF

# 下载示例配置
curl -o config.yaml https://raw.githubusercontent.com/router-for-me/CLIProxyAPIPlus/main/config.example.yaml

# 拉取并启动
docker compose pull && docker compose up -d
```

### 使用预构建镜像

**生产环境（稳定版本）：**
```bash
# 最新稳定版本
docker pull ghcr.io/your-org/cliproxyapiplus:latest

# 特定版本
docker pull ghcr.io/your-org/cliproxyapiplus:v1.2.3
```

**开发分支：**
```bash
# 从 CLIProxyAPIPlus-gdtiti 分支获取最新版本（包含 Gemini CLI 修复）
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti

# 运行开发镜像
docker run -d \
  --name cliproxyapi-dev \
  -p 8317:8317 \
  -v $(pwd)/config.yaml:/CLIProxyAPI/config.yaml \
  -v ~/.gemini:/root/.gemini \
  ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

查看 **[Docker 分支构建指南](docs/DOCKER_BRANCH_BUILD.md)** 了解如何使用开发分支镜像的详细说明。

### 本地构建

```bash
docker build -t cliproxyapi-local .
docker run -p 8317:8317 cliproxyapi-local
```

### 配置说明

启动前请编辑 `config.yaml`：

```yaml
# 基本配置示例
server:
  port: 8317

# 在此添加你的供应商配置
```

### 更新到最新版本

```bash
cd ~/cli-proxy
docker compose pull && docker compose up -d
```

## 最近更新

### ✅ Gemini CLI "ALL" 选项修复（CLIProxyAPIPlus-gdtiti 分支）

修复了选择 "ALL" 时 Gemini CLI 登录静默失败的关键问题：
- ✅ 现在会跳过有问题的项目并继续处理其他项目
- ✅ 为成功激活的项目保存凭证
- ✅ 为失败的项目提供清晰的警告信息
- ✅ 报告成功和失败的汇总信息

**立即试用：**
```bash
# 使用 Docker
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
docker run -it ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti sh
# 在容器内：gemini-cli login

# 或从源码构建
git checkout CLIProxyAPIPlus-gdtiti
go run cmd/server/main.go
```

查看 [docs/GEMINI_ALL_FIX.md](docs/GEMINI_ALL_FIX.md) 了解技术细节。

## 贡献

该项目仅接受第三方供应商支持的 Pull Request。任何非第三方供应商支持的 Pull Request 都将被拒绝。

1. Fork 仓库
2. 创建您的功能分支（`git checkout -b feature/amazing-feature`）
3. 提交您的更改（`git commit -m 'Add some amazing feature'`）
4. 推送到分支（`git push origin feature/amazing-feature`）
5. 打开 Pull Request

## 谁与我们在一起？

这些项目基于 CLIProxyAPI:

### [vibeproxy](https://github.com/automazeio/vibeproxy)

一个原生 macOS 菜单栏应用，让您可以使用 Claude Code & ChatGPT 订阅服务和 AI 编程工具，无需 API 密钥。

### [Subtitle Translator](https://github.com/VjayC/SRT-Subtitle-Translator-Validator)

一款基于浏览器的 SRT 字幕翻译工具，可通过 CLI 代理 API 使用您的 Gemini 订阅。内置自动验证与错误修正功能，无需 API 密钥。

### [CCS (Claude Code Switch)](https://github.com/kaitranntt/ccs)

CLI 封装器，用于通过 CLIProxyAPI OAuth 即时切换多个 Claude 账户和替代模型（Gemini, Codex, Antigravity），无需 API 密钥。

### [ProxyPal](https://github.com/heyhuynhgiabuu/proxypal)

基于 macOS 平台的原生 CLIProxyAPI GUI：配置供应商、模型映射以及OAuth端点，无需 API 密钥。

### [Quotio](https://github.com/nguyenphutrong/quotio)

原生 macOS 菜单栏应用，统一管理 Claude、Gemini、OpenAI、Qwen 和 Antigravity 订阅，提供实时配额追踪和智能自动故障转移，支持 Claude Code、OpenCode 和 Droid 等 AI 编程工具，无需 API 密钥。

### [CodMate](https://github.com/loocor/CodMate)

原生 macOS SwiftUI 应用，用于管理 CLI AI 会话（Claude Code、Codex、Gemini CLI），提供统一的提供商管理、Git 审查、项目组织、全局搜索和终端集成。集成 CLIProxyAPI 为 Codex、Claude、Gemini、Antigravity 和 Qwen Code 提供统一的 OAuth 认证，支持内置和第三方提供商通过单一代理端点重路由 - OAuth 提供商无需 API 密钥。

### [ProxyPilot](https://github.com/Finesssee/ProxyPilot)

原生 Windows CLIProxyAPI 分支，集成 TUI、系统托盘及多服务商 OAuth 认证，专为 AI 编程工具打造，无需 API 密钥。

### [Claude Proxy VSCode](https://github.com/uzhao/claude-proxy-vscode)

一款 VSCode 扩展，提供了在 VSCode 中快速切换 Claude Code 模型的功能，内置 CLIProxyAPI 作为其后端，支持后台自动启动和关闭。

### [ZeroLimit](https://github.com/0xtbug/zero-limit)

Windows 桌面应用，基于 Tauri + React 构建，用于通过 CLIProxyAPI 监控 AI 编程助手配额。支持跨 Gemini、Claude、OpenAI Codex 和 Antigravity 账户的使用量追踪，提供实时仪表盘、系统托盘集成和一键代理控制，无需 API 密钥。

### [CPA-XXX Panel](https://github.com/ferretgeek/CPA-X)

面向 CLIProxyAPI 的 Web 管理面板，提供健康检查、资源监控、日志查看、自动更新、请求统计与定价展示，支持一键安装与 systemd 服务。

### [CLIProxyAPI Tray](https://github.com/kitephp/CLIProxyAPI_Tray)

Windows 托盘应用，基于 PowerShell 脚本实现，不依赖任何第三方库。主要功能包括：自动创建快捷方式、静默运行、密码管理、通道切换（Main / Plus）以及自动下载与更新。

> [!NOTE]  
> 如果你开发了基于 CLIProxyAPI 的项目，请提交一个 PR（拉取请求）将其添加到此列表中。

## 更多选择

以下项目是 CLIProxyAPI 的移植版或受其启发：

### [9Router](https://github.com/decolua/9router)

基于 Next.js 的实现，灵感来自 CLIProxyAPI，易于安装使用；自研格式转换（OpenAI/Claude/Gemini/Ollama）、组合系统与自动回退、多账户管理（指数退避）、Next.js Web 控制台，并支持 Cursor、Claude Code、Cline、RooCode 等 CLI 工具，无需 API 密钥。

> [!NOTE]  
> 如果你开发了 CLIProxyAPI 的移植或衍生项目，请提交 PR 将其添加到此列表中。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。