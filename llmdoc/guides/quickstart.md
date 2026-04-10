# 快速开始

开发环境搭建和基本使用指南。

## 1. 环境准备

1. 安装 Go 1.24+
2. 克隆仓库并进入目录
3. 复制配置文件：`copy config.example.yaml config.yaml`

## 2. 安装依赖

```bash
dev.bat install
# 或
go mod download
```

## 3. 登录提供商

```bash
# Kiro (AWS CodeWhisperer)
dev.bat login   # 选择 kiro-login

# 或直接运行
go run cmd/server/main.go --kiro-login
```

## 4. 启动服务

```bash
dev.bat dev
# 或
go run cmd/server/main.go
```

## 5. 测试 API

```bash
curl http://localhost:8317/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "Hello"}]}'
```

## 6. 验证

- 访问 `http://localhost:8317/` 查看服务状态
- 访问 `http://localhost:8317/management.html` 打开管理面板（需配置密钥）
