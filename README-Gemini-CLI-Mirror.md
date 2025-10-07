# CLIProxyAPI Gemini CLI 镜像节点配置指南

## 概述

CLIProxyAPI 现已支持 Gemini CLI 镜像节点配置，允许您使用自定义的镜像端点来替代 Google 官方端点。

## 新增配置项

### `gemini-cli` 配置段

```yaml
gemini-cli:
  # Code Assist API 端点（必配）
  code-assist-endpoint: "https://gcli-api.sukaka.top/cloudcode-pa"

  # OAuth2 认证端点（必配）
  oauth-endpoint: "https://gcli-api.sukaka.top/oauth2"

  # Google APIs 基础端点（必配）
  google-apis-endpoint: "https://gcli-api.sukaka.top/googleapis"

  # Resource Manager API 端点（可选）
  resource-manager-endpoint: "https://gcli-api.sukaka.top/cloudresourcemanager"

  # Service Usage API 端点（可选）
  service-usage-endpoint: "https://gcli-api.sukaka.top/serviceusage"

  # Gemini CLI 专用代理（可选，覆盖全局设置）
  proxy-url: "socks5://127.0.0.1:10808"
```

## 端点自动构建

系统会自动构建以下端点：

- **Token URL**: `{oauth-endpoint}/token`
- **Userinfo URL**: `{google-apis-endpoint}/oauth2/v2/userinfo`

## 配置优先级

1. **gemini-cli.proxy-url**（最高优先级）
2. **全局 proxy-url**
3. **直连**（默认）

## 配置文件示例

### 最小化配置

```yaml
port: 8080
auth-dir: "./auth"

gemini-cli:
  code-assist-endpoint: "https://gcli-api.sukaka.top/cloudcode-pa"
  oauth-endpoint: "https://gcli-api.sukaka.top/oauth2"
  google-apis-endpoint: "https://gcli-api.sukaka.top/googleapis"
```

### 完整配置

参考 `config-example-with-gemini-cli-mirror.yaml`

## 使用方法

1. **配置文件设置**
   ```bash
   cp config-example-with-gemini-cli-mirror.yaml config.yaml
   # 编辑 config.yaml 文件
   ```

2. **启动服务**
   ```bash
   ./cliproxy --config config.yaml
   ```

3. **验证配置**
   - 查看启动日志确认端点配置生效
   - 使用 Gemini CLI API 进行测试

## 向后兼容性

- 未配置 `gemini-cli` 时，系统将使用官方端点
- 现有配置文件无需修改即可继续使用

## 故障排除

### 1. 端点配置错误
**症状**: 启动日志显示端点错误
**解决**: 检查 `gemini-cli` 配置段中的端点 URL

### 2. 镜像节点不可用
**症状**: API 调用失败
**解决**:
- 检查镜像节点状态
- 尝试切换到官方端点
- 检查代理配置

### 3. 代理连接失败
**症状**: 代理连接错误
**解决**:
- 验证代理服务状态
- 检查代理 URL 格式
- 确认代理软件允许外部连接

## 技术细节

### 修改的文件

1. **`internal/config/config.go`**
   - 添加 `GeminiCLIConfig` 结构体
   - 实现端点获取方法

2. **`internal/runtime/executor/gemini_cli_executor.go`**
   - 移除硬编码的 `codeAssistEndpoint`
   - 使用配置驱动的端点
   - 修改 OAuth2 配置使用镜像端点

### API 端点映射

| 功能 | 配置项 | 默认值 | 镜像节点示例 |
|------|--------|--------|-------------|
| Code Assist | `code-assist-endpoint` | `https://cloudcode-pa.googleapis.com` | `https://gcli-api.sukaka.top/cloudcode-pa` |
| OAuth Auth | `oauth-endpoint` | `https://oauth2.googleapis.com` | `https://gcli-api.sukaka.top/oauth2` |
| Google APIs | `google-apis-endpoint` | `https://www.googleapis.com` | `https://gcli-api.sukaka.top/googleapis` |

## 支持的镜像节点

当前支持的镜像节点示例：

```yaml
gemini-cli:
  code-assist-endpoint: "https://gcli-api.sukaka.top/cloudcode-pa"
  oauth-endpoint: "https://gcli-api.sukaka.top/oauth2"
  google-apis-endpoint: "https://gcli-api.sukaka.top/googleapis"
  resource-manager-endpoint: "https://gcli-api.sukaka.top/cloudresourcemanager"
  service-usage-endpoint: "https://gcli-api.sukaka.top/serviceusage"
```

## 更新日志

### v2.0.0 (2025-10-07)
- ✅ 新增 Gemini CLI 镜像节点配置支持
- ✅ 移除硬编码端点，使用配置驱动
- ✅ 支持端点自动构建
- ✅ 支持独立代理配置
- ✅ 保持向后兼容性

---

**技术支持**: 如有问题，请检查配置文件格式和网络连接状态。