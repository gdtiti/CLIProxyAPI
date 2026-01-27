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
    image: 17600006524/cli-proxy-api-plus:latest
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
curl -o config.yaml https://raw.githubusercontent.com/linlang781/CLIProxyAPIPlus/main/config.example.yaml

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

如果需要提交任何非第三方供应商支持的 Pull Request，请提交到主线版本。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。