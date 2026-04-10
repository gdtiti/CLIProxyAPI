# Codex Plus/Free 套餐区分设计方案

## 1. 配置文件扩展

### 1.1 现有配置结构
```yaml
codex-api-key:
  - api-key: "sk-xxx"
    prefix: "test"
    base-url: "https://api.example.com"
    excluded-models:
      - "gpt-5.1"
```

### 1.2 扩展后配置结构
```yaml
codex-api-key:
  - api-key: "sk-xxx-plus"
    prefix: "plus"
    base-url: "https://api.example.com"
    plan-type: "plus"  # 新增：套餐类型
    excluded-models:
      - "gpt-4-mini"   # Plus排除免费模型
  - api-key: "sk-xxx-free"
    prefix: "free"
    base-url: "https://api.example.com"
    plan-type: "free"  # 新增：套餐类型
    excluded-models:
      - "gpt-5"        # Free排除付费模型
      - "gpt-4-turbo"
```

## 2. 数据结构修改

### 2.1 CodexKey 结构扩展
```go
type CodexKey struct {
    APIKey         string       `yaml:"api-key" json:"api-key"`
    Prefix         string       `yaml:"prefix,omitempty" json:"prefix,omitempty"`
    BaseURL        string       `yaml:"base-url,omitempty" json:"base-url,omitempty"`
    PlanType       string       `yaml:"plan-type,omitempty" json:"plan-type,omitempty"` // 新增
    ProxyURL       string       `yaml:"proxy-url,omitempty" json:"proxy-url,omitempty"`
    Models         []CodexModel `yaml:"models,omitempty" json:"models,omitempty"`
    Headers        map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
    ExcludedModels []string     `yaml:"excluded-models,omitempty" json:"excluded-models,omitempty"`
}
```

## 3. 管理界面区分

### 3.1 凭证管理Tab分离
- **Plus凭证Tab**：显示 plan-type="plus" 的凭证
- **Free凭证Tab**：显示 plan-type="free" 的凭证
- **通用凭证Tab**：显示未设置plan-type的凭证

### 3.2 模型排除分离
- 按套餐类型分别管理排除模型列表
- Plus套餐默认排除Free模型
- Free套餐默认排除Plus模型

### 3.3 使用量查询分离
- 按plan-type维度统计使用量
- 支持按套餐类型筛选使用记录
- 提供套餐对比分析

## 4. 实现步骤

### 阶段1：数据结构扩展 ✅
1. ✅ 修改 `internal/config/config.go` 中的 CodexKey 结构
2. ✅ 更新配置文件解析和验证逻辑
3. ✅ 添加套餐类型枚举和验证

### 阶段2：管理界面改造 🚧
1. ✅ 修改 `internal/api/handlers/management/config_lists.go`
2. ✅ 扩展Codex相关的API接口
3. ✅ 添加套餐类型过滤功能
4. ⏳ 需要添加路由配置

### 阶段3：前端界面调整 ⏳
1. ⏳ 修改管理控制面板的Codex部分
2. ⏳ 添加套餐类型选择器
3. ⏳ 实现分Tab显示

### 阶段4：使用量统计扩展 ⏳
1. ⏳ 在使用量记录中添加套餐类型字段
2. ⏳ 修改统计查询接口支持套餐筛选
3. ⏳ 更新统计报表显示

## 5. 兼容性考虑

### 5.1 向后兼容
- 未设置plan-type的现有配置继续有效
- 默认归类为"通用"套餐
- 不影响现有API调用

### 5.2 迁移策略
- 提供配置迁移工具
- 支持批量设置套餐类型
- 保留原有配置备份

## 6. 新增API接口

### 6.1 按套餐类型获取Codex凭证
```
GET /v0/management/codex-api-key/by-plan-type?plan-type={type}
```

**参数说明：**
- `plan-type`: 套餐类型
  - `plus`: 返回Plus套餐凭证
  - `free`: 返回Free套餐凭证
  - `""` (空): 返回通用凭证（未设置plan-type）
  - `all`: 返回所有凭证

**响应示例：**
```json
{
  "codex-api-key": [
    {
      "api-key": "sk-xxx-plus",
      "prefix": "plus",
      "base-url": "https://api.example.com",
      "plan-type": "plus",
      "excluded-models": ["gpt-4-mini"]
    }
  ],
  "plan-type": "plus",
  "total": 1
}
```

### 6.2 获取套餐类型统计
```
GET /v0/management/codex-api-key/plan-types
```

**响应示例：**
```json
{
  "plan-types": {
    "plus": 2,
    "free": 3,
    "general": 1
  },
  "total": 6
}
```

## 8. 当前实现状态

### 8.1 已完成功能 ✅
1. **数据结构扩展**
   - ✅ 在 `CodexKey` 结构中添加了 `PlanType` 字段
   - ✅ 支持 YAML 配置中的 `plan-type` 字段
   - ✅ 更新了配置解析和验证逻辑

2. **管理API扩展**
   - ✅ 扩展了 `PatchCodexKey` 接口支持 `plan-type` 字段
   - ✅ 添加了 `GetCodexKeysByPlanType` 接口按套餐类型过滤
   - ✅ 添加了 `GetCodexPlanTypes` 接口获取套餐类型统计
   - ✅ 更新了路由配置

3. **配置示例更新**
   - ✅ 更新了 `config.example.yaml` 展示新字段用法
   - ✅ 创建了测试配置文件

### 8.2 待完成功能 ⏳
1. **前端界面调整**
   - ⏳ 修改管理控制面板的Codex部分
   - ⏳ 添加套餐类型选择器
   - ⏳ 实现分Tab显示

2. **使用量统计扩展**
   - ⏳ 在使用量记录中添加套餐类型字段
   - ⏳ 修改统计查询接口支持套餐筛选
   - ⏳ 更新统计报表显示

3. **代码清理**
   - ⏳ 解决Git合并冲突问题
   - ⏳ 完善单元测试

### 8.3 技术债务
- 存在Git合并冲突导致编译失败
- 需要清理不相关的访问控制代码
- 建议在独立分支上完成剩余功能开发