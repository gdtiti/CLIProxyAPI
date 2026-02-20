# iFlow 稳定性修复实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 CLIProxyAPI 反代 iFlow 时的 406 错误和偶发无响应问题

**架构:** 通过请求头对齐、406 专项兜底、流式稳定性增强、Auth 可用性策略修正四个层面解决问题

**技术栈:** Go, HTTP 客户端, SSE 流式处理, Auth 管理

**参考设计:** `docs/plans/2026-02-20-iflow-stability-fix-design.md`

---

## Task 1: 修复请求头 - 移除显式 Accept 头并添加 conversation-id

**Files:**
- Modify: `internal/runtime/executor/iflow_executor.go:455-478`

**Step 1: 查看当前 applyIFlowHeaders 实现**

```bash
grep -n "func applyIFlowHeaders" internal/runtime/executor/iflow_executor.go
```

**Step 2: 修改 applyIFlowHeaders 函数**

修改内容:
```go
func applyIFlowHeaders(r *http.Request, apiKey string, stream bool) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+apiKey)
	r.Header.Set("User-Agent", iflowUserAgent)

	// Generate session-id
	sessionID := "session-" + generateUUID()
	r.Header.Set("session-id", sessionID)

	// Generate timestamp and signature
	timestamp := time.Now().UnixMilli()
	r.Header.Set("x-iflow-timestamp", fmt.Sprintf("%d", timestamp))

	signature := createIFlowSignature(iflowUserAgent, sessionID, timestamp, apiKey)
	if signature != "" {
		r.Header.Set("x-iflow-signature", signature)
	}

	// 添加 conversation-id，始终带键（与 iFlow 官方行为一致）
	conversationID := generateUUID()
	r.Header.Set("conversation-id", conversationID)

	// 不再主动设置 Accept 头，使用 HTTP 客户端默认行为
	// 这避免了某些上游节点因显式 Accept: text/event-stream 而返回 406
}
```

**Step 3: 验证修改**

```bash
grep -A 5 "conversation-id" internal/runtime/executor/iflow_executor.go
```

Expected: 显示 conversation-id 头设置代码

**Step 4: 编译检查**

```bash
go build ./internal/runtime/executor/...
```

Expected: 无编译错误

**Step 5: Commit**

```bash
git add internal/runtime/executor/iflow_executor.go
git commit -m "fix(iflow): align request headers with official iFlow implementation

- Remove explicit Accept header to avoid 406 errors
- Add conversation-id header for compatibility
- See design doc: docs/plans/2026-02-20-iflow-stability-fix-design.md"
```

---

## Task 2: 实现 406 专项兜底机制

**Files:**
- Modify: `internal/runtime/executor/iflow_executor.go:73-174` (Execute)
- Modify: `internal/runtime/executor/iflow_executor.go:176-297` (ExecuteStream)

**Step 1: 添加 406 检测辅助函数**

在文件末尾添加:
```go
// is406Error 检测是否为 406 Not Acceptable 错误
func is406Error(err error) bool {
	if err == nil {
		return false
	}
	// 检查状态码
	status := statusCodeFromError(err)
	if status != http.StatusNotAcceptable {
		return false
	}
	return true
}

// retryWithoutSignature 移除签名头后重试请求
func (e *IFlowExecutor) retryWithoutSignature(ctx context.Context, httpReq *http.Request, body []byte, isStream bool) (*http.Response, error) {
	// 创建新的请求，移除签名头
	newReq, err := http.NewRequestWithContext(ctx, httpReq.Method, httpReq.URL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	
	// 复制原请求的头，但移除签名相关头
	for key, values := range httpReq.Header {
		if key != "x-iflow-signature" && key != "x-iflow-timestamp" {
			for _, value := range values {
				newReq.Header.Add(key, value)
			}
		}
	}
	
	// 重新设置非签名的基础头
	apiKey := ""
	if authHeader := httpReq.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer ")
	}
	
	// 重新生成不带签名的头
	newReq.Header.Set("Content-Type", "application/json")
	newReq.Header.Set("User-Agent", iflowUserAgent)
	sessionID := "session-" + generateUUID()
	newReq.Header.Set("session-id", sessionID)
	conversationID := generateUUID()
	newReq.Header.Set("conversation-id", conversationID)
	
	if apiKey != "" {
		newReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	
	log.Debugf("iflow: retrying without signature headers due to 406")
	
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, nil, 0)
	return httpClient.Do(newReq)
}
```

**Step 2: 修改 Execute 函数添加 406 重试**

在 Execute 函数中，找到 HTTP 请求部分:
```go
// 在 httpClient.Do(httpReq) 后添加 406 重试逻辑
httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
httpResp, err := httpClient.Do(httpReq)
if err != nil {
	recordAPIResponseError(ctx, e.cfg, err)
	return resp, err
}

// 406 专项兜底：遇到 406 时重试一次（无签名头）
if httpResp.StatusCode == http.StatusNotAcceptable {
	if errClose := httpResp.Body.Close(); errClose != nil {
		log.Errorf("iflow executor: close 406 response body error: %v", errClose)
	}
	httpResp, err = e.retryWithoutSignature(ctx, httpReq, body, false)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return resp, err
	}
}
```

**Step 3: 修改 ExecuteStream 函数添加 406 重试**

在 ExecuteStream 函数中，找到 HTTP 请求部分:
```go
// 在 httpClient.Do(httpReq) 后添加 406 重试逻辑
httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
httpResp, err := httpClient.Do(httpReq)
if err != nil {
	recordAPIResponseError(ctx, e.cfg, err)
	return nil, err
}

// 406 专项兜底：遇到 406 时重试一次（无签名头）
if httpResp.StatusCode == http.StatusNotAcceptable {
	if errClose := httpResp.Body.Close(); errClose != nil {
		log.Errorf("iflow executor: close 406 response body error: %v", errClose)
	}
	httpResp, err = e.retryWithoutSignature(ctx, httpReq, body, true)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return nil, err
	}
}
```

**Step 4: 编译检查**

```bash
go build ./internal/runtime/executor/...
```

Expected: 无编译错误

**Step 5: Commit**

```bash
git add internal/runtime/executor/iflow_executor.go
git commit -m "fix(iflow): add 406 retry mechanism without signature headers

- Detect 406 errors and retry once without signature headers
- Apply to both Execute and ExecuteStream methods
- Prevents infinite retry loops"
```

---

## Task 3: 修正 Auth 可用性策略

**Files:**
- Modify: `sdk/cliproxy/auth/conductor.go:1519-1571`

**Step 1: 查看当前 applyAuthFailureState 实现**

```bash
grep -n "func applyAuthFailureState" sdk/cliproxy/auth/conductor.go
```

**Step 2: 修改 applyAuthFailureState 函数**

修改 406 错误处理分支:
```go
func applyAuthFailureState(auth *Auth, resultErr *Error, retryAfter *time.Duration, now time.Time) {
	if auth == nil {
		return
	}
	
	statusCode := statusCodeFromResult(resultErr)
	
	// 406 错误不触发 auth 冷却，因为通常是请求格式问题
	if statusCode == http.StatusNotAcceptable {
		auth.StatusMessage = "not_acceptable"
		auth.LastError = cloneError(resultErr)
		auth.UpdatedAt = now
		// 不设置 Unavailable，不设置 NextRetryAfter
		return
	}
	
	auth.Unavailable = true
	auth.Status = StatusError
	auth.UpdatedAt = now
	if resultErr != nil {
		auth.LastError = cloneError(resultErr)
		if resultErr.Message != "" {
			auth.StatusMessage = resultErr.Message
		}
	}
	
	switch statusCode {
	case 401:
		auth.StatusMessage = "unauthorized"
		auth.NextRetryAfter = now.Add(30 * time.Minute)
	case 402, 403:
		auth.StatusMessage = "payment_required"
		auth.NextRetryAfter = now.Add(30 * time.Minute)
	case 404:
		auth.StatusMessage = "not_found"
		auth.NextRetryAfter = now.Add(12 * time.Hour)
	case 429:
		auth.StatusMessage = "quota exhausted"
		auth.Quota.Exceeded = true
		auth.Quota.Reason = "quota"
		var next time.Time
		if retryAfter != nil {
			next = now.Add(*retryAfter)
		} else {
			cooldown, nextLevel := nextQuotaCooldown(auth.Quota.BackoffLevel, quotaCooldownDisabledForAuth(auth))
			if cooldown > 0 {
				next = now.Add(cooldown)
			}
			auth.Quota.BackoffLevel = nextLevel
		}
		auth.Quota.NextRecoverAt = next
		auth.NextRetryAfter = next
	case 408, 500, 502, 503, 504:
		auth.StatusMessage = "transient upstream error"
		if quotaCooldownDisabledForAuth(auth) {
			auth.NextRetryAfter = time.Time{}
		} else {
			auth.NextRetryAfter = now.Add(1 * time.Minute)
		}
	default:
		if auth.StatusMessage == "" {
			auth.StatusMessage = "request failed"
		}
	}
}
```

**Step 3: 修改 conductor 中的错误处理逻辑**

找到处理错误状态码的 switch 语句 (约 line 1290)，添加 406 处理:
```go
switch statusCode {
case 401:
	next := now.Add(30 * time.Minute)
	state.NextRetryAfter = next
	suspendReason = "unauthorized"
	shouldSuspendModel = true
case 402, 403:
	next := now.Add(30 * time.Minute)
	state.NextRetryAfter = next
	suspendReason = "payment_required"
	shouldSuspendModel = true
case 404:
	next := now.Add(12 * time.Hour)
	state.NextRetryAfter = next
	suspendReason = "not_found"
	shouldSuspendModel = true
case 406:
	// 406 不触发 auth 冷却，仅记录状态
	state.StatusMessage = "not_acceptable"
	state.NextRetryAfter = time.Time{}
	// 不设置 shouldSuspendModel，不触发冷却
case 429:
	// ... 现有 429 处理逻辑
```

**Step 4: 编译检查**

```bash
go build ./sdk/cliproxy/auth/...
```

Expected: 无编译错误

**Step 5: Commit**

```bash
git add sdk/cliproxy/auth/conductor.go
git commit -m "fix(auth): adjust auth availability strategy for 406 errors

- 406 errors no longer trigger auth cooldown
- 406 is treated as request format issue, not auth issue
- Prevents single-auth scenarios from self-fusing"
```

---

## Task 4: 添加单元测试

**Files:**
- Create: `internal/runtime/executor/iflow_executor_test.go`
- Modify: `sdk/cliproxy/auth/conductor_test.go` (如果存在)

**Step 1: 添加 406 重试测试**

```go
func TestIFlowExecutor_406Retry(t *testing.T) {
	// 测试 406 错误时是否正确重试
	// 模拟第一次返回 406，第二次返回 200
}

func TestIFlowExecutor_ConversationIDHeader(t *testing.T) {
	// 测试 conversation-id 头是否正确设置
}

func TestIFlowExecutor_NoAcceptHeader(t *testing.T) {
	// 测试是否没有设置 Accept 头
}
```

**Step 2: 运行测试**

```bash
go test ./internal/runtime/executor/... -v -run TestIFlow
```

Expected: 所有测试通过

**Step 3: Commit**

```bash
git add internal/runtime/executor/iflow_executor_test.go
git commit -m "test(iflow): add unit tests for 406 retry and header alignment

- Test 406 retry mechanism
- Test conversation-id header presence
- Test absence of Accept header"
```

---

## Task 5: 集成测试验证

**Files:**
- 无文件修改，仅验证

**Step 1: 完整编译**

```bash
go build ./...
```

Expected: 无编译错误

**Step 2: 运行所有相关测试**

```bash
go test ./internal/runtime/executor/... ./sdk/cliproxy/auth/... -v
```

Expected: 所有测试通过

**Step 3: Commit (如果有测试文件更新)**

```bash
git add -A
git commit -m "test: verify all tests pass after iflow stability fixes"
```

---

## 实施检查清单

- [ ] Task 1: 请求头对齐完成
- [ ] Task 2: 406 兜底机制实现
- [ ] Task 3: Auth 可用性策略修正
- [ ] Task 4: 单元测试添加
- [ ] Task 5: 集成测试通过

## 预期效果验证

实施后应观察到：
1. 406 错误率下降
2. 偶发无响应问题减少
3. 单 auth 场景更稳定
4. 监控中 503 错误正确上报（当真正不可用时）

## 回滚计划

如果出现问题：
```bash
git revert HEAD~4..HEAD  # 回滚所有 4 个 commit
go build ./... && go test ./...
```
