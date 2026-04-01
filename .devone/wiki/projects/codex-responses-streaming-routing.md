# Codex Responses 流式分流与非流式表象

## 适用范围

- OpenAI Responses HTTP 入口 `/v1/responses`
- OpenAI Responses 紧凑端点 `/v1/responses/compact`
- OpenAI Responses WebSocket 入口
- Codex HTTP executor 与 Codex WebSocket executor

## 已证实事实

- `/v1/responses` 只在请求入口根据 `stream` 字段选择流式或非流式 handler；不会在一条已经开始输出的请求中途切换执行模式。
- Codex 非流式 `Execute` 仍会向上游发送 `stream=true`，但会在本地读到 `response.completed` 后再一次性聚合返回。
- Codex 流式 `ExecuteStream` 会逐段转发上游事件；首个 payload 发出后，不再允许 bootstrap retry 改道。
- `codex -> openai responses` 的流式翻译器基本是透传，不存在“先缓存全文再输出”的聚合逻辑。
- Responses WebSocket 的规范化过程会强制把请求写成 `stream=true`。
- Responses WebSocket 首个 `response.create` 且 `generate=false` 时，会走本地 prewarm 短路，直接返回 `response.created` 与 `response.completed`。
- Codex WebSocket 上游握手若返回 `426 Upgrade Required`，只会降级到 HTTP SSE 流式执行，不会改为非流式。

## 为什么会看起来像“突然切成非流式”

- 后续某一轮请求本来就是 `stream=false`，因此入口直接走了非流式 handler。
- 客户端显式调用了 `/v1/responses/compact`，该端点只支持非流式。
- WebSocket 首个 `generate=false` prewarm 请求会立即完成，表象像“空的非流式返回”。
- WebSocket 上游回退到 HTTP SSE 上游后，虽然仍是流式，但传输形态变化会让体感接近“整段返回”。

## 排查顺序

1. 先确认命中的下游端点是不是 `/v1/responses/compact`。
2. 再确认请求体里 `stream` 是否为 `false`。
3. 如果是 Responses WebSocket，检查是否为首个 `response.create` 且 `generate=false`。
4. 如果是 WebSocket 上游失败，检查是否发生了 `426 Upgrade Required`，此时只是回退到 HTTP SSE 流式。
5. 若仍无法判定，再结合 `request-log` 或抓包核对真实请求体。

## 证据落点

- `sdk/api/handlers/openai/openai_responses_handlers.go`
- `sdk/api/handlers/openai/openai_responses_websocket.go`
- `sdk/api/handlers/handlers.go`
- `internal/runtime/executor/codex_executor.go`
- `internal/runtime/executor/codex_websockets_executor.go`
- `internal/translator/codex/openai/responses/codex_openai-responses_response.go`

## 更新记录

- 2026-03-29: 基于静态代码分析补充 Codex Responses 流式分流规则，并明确“中途切非流式”在当前实现中更像请求形态混用，而非同一条流中途改道。
