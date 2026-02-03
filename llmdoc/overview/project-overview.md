# CLIProxyAPI Plus 项目概述

## 1. 身份

- **是什么：** 多提供商 AI API 代理服务，提供统一的 OpenAI/Claude/Gemini 兼容接口
- **目的：** 将多种 AI CLI 工具（Kiro、Claude、Copilot、Gemini 等）的认证和 API 统一代理，支持第三方客户端调用

## 2. 高级描述

CLIProxyAPI Plus 是 CLIProxyAPI 的增强版本，由社区维护。它作为 AI API 网关，接收标准格式的请求（OpenAI/Claude/Gemini 格式），通过翻译器转换为目标提供商格式，使用 OAuth 凭据执行请求，并将响应转换回客户端期望的格式。

## 3. 技术栈

- **语言：** Go 1.24+
- **Web 框架：** Gin
- **配置：** YAML + 热重载（fsnotify）
- **认证：** OAuth 2.0 / Device Flow / PKCE
- **存储：** 文件系统 / PostgreSQL / Git / S3 兼容对象存储
- **日志：** Logrus + Lumberjack（日志轮转）

## 4. 核心模块

| 模块 | 路径 | 职责 |
|------|------|------|
| API 服务器 | `internal/api/` | HTTP 路由、中间件、请求处理 |
| 认证系统 | `internal/auth/` | 各提供商 OAuth 认证实现 |
| 翻译器 | `internal/translator/` | 请求/响应格式转换 |
| 执行器 | `internal/runtime/executor/` | 提供商 API 调用执行 |
| 配置 | `internal/config/` | YAML 配置加载和管理 |
| SDK | `sdk/` | 可复用的核心组件库 |

## 5. 支持的提供商

- **Kiro (AWS CodeWhisperer)** - OAuth Web / AWS Builder ID / Token 导入
- **Claude (Anthropic)** - OAuth 认证
- **GitHub Copilot** - Device Flow 认证
- **Gemini (Google)** - OAuth / API Key
- **Codex (OpenAI)** - OAuth 认证
- **Qwen / iFlow / Antigravity** - 各自 OAuth 流程

## 6. 入口点

- `cmd/server/main.go` - 主程序入口，处理命令行参数和服务启动
- `internal/api/server.go` - HTTP 服务器初始化和路由注册
