# 翻译器系统架构

## 1. 身份

- **是什么：** 请求/响应格式转换引擎
- **目的：** 在不同 AI API 格式之间转换，实现跨提供商兼容

## 2. 核心组件

- `internal/translator/init.go`: 翻译器注册初始化
- `internal/translator/translator/translator.go`: 翻译器接口定义
- `sdk/translator/registry.go`: 翻译器注册表
- `sdk/translator/pipeline.go`: 翻译管道，链式处理

## 3. 翻译器目录结构

```
internal/translator/
├── claude/           # Claude 格式转换
│   ├── openai/       # Claude → OpenAI
│   ├── gemini/       # Claude → Gemini
│   └── gemini-cli/   # Claude → Gemini CLI
├── openai/           # OpenAI 格式转换
│   ├── claude/       # OpenAI → Claude
│   └── gemini/       # OpenAI → Gemini
├── gemini/           # Gemini 格式转换
├── codex/            # Codex 格式转换
└── kiro/             # Kiro 格式转换
```

## 4. 执行流程（LLM 检索地图）

- **1. 请求接收：** 处理器接收客户端请求（如 OpenAI 格式）
- **2. 查找翻译器：** `registry.GetTranslator(sourceFormat, targetFormat)`
- **3. 请求转换：** `translator.TranslateRequest(req)` 转换为目标格式
- **4. 执行请求：** 调用目标提供商 API
- **5. 响应转换：** `translator.TranslateResponse(resp)` 转换回源格式

## 5. 支持的格式转换

| 源格式 | 目标格式 | 用途 |
|--------|----------|------|
| OpenAI | Claude | 使用 Claude 后端处理 OpenAI 请求 |
| OpenAI | Gemini | 使用 Gemini 后端处理 OpenAI 请求 |
| Claude | OpenAI | 使用 OpenAI 后端处理 Claude 请求 |
| Codex | Claude/Gemini | Codex 客户端使用其他后端 |
