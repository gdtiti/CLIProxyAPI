# 配置系统架构

## 1. 身份

- **是什么：** YAML 配置管理系统
- **目的：** 加载、验证、热重载应用配置

## 2. 核心组件

- `internal/config/config.go` (Config, LoadConfig): 配置结构定义和加载
- `internal/config/sdk_config.go`: SDK 相关配置
- `internal/watcher/watcher.go`: 文件监控和热重载
- `internal/watcher/diff/`: 配置差异检测

## 3. 配置结构

```yaml
# config.yaml 主要配置项
server:
  host: ""           # 绑定地址
  port: 8317         # 监听端口

auth-dir: ""         # Token 存储目录
debug: false         # 调试模式
logging-to-file: false

# 提供商配置
gemini-api-key: []   # Gemini API Key 列表
claude-api-key: []   # Claude API Key 列表
codex-api-key: []    # Codex API Key 列表
kiro: []             # Kiro 配置列表
openai-compatibility: []  # OpenAI 兼容提供商

# 高级配置
remote-management:   # 远程管理 API
routing:             # 凭据路由策略
quota-exceeded:      # 配额超限处理
```

## 4. 执行流程（LLM 检索地图）

- **1. 启动加载：** `config.LoadConfig(path)` 读取 YAML 文件
- **2. 环境覆盖：** `applyEnvironmentOverrides()` 应用环境变量
- **3. 迁移处理：** 自动迁移旧版配置格式
- **4. 监控启动：** `watcher.Watch()` 监控配置文件变化
- **5. 热重载：** 检测到变化后重新加载，通知相关组件

## 5. 热重载机制

`internal/watcher/` 实现配置热重载：
- `fsnotify` 监控文件变化
- `diff/config_diff.go` 计算配置差异
- `synthesizer/` 合成新配置状态
- 仅重载变化的部分，避免服务中断
