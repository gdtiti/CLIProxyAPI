#!/bin/bash
# 1000 次高并发压测 /v1/chat/completions
# 用法: ./load_test.sh [并发数] [请求数]

CONCURRENCY=${1:-80}
TOTAL=${2:-1000}
URL="http://127.0.0.1:18317/v1/chat/completions"
BODY='{"model":"gpt-5-codex-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":5}'

echo "=== Load Test: $TOTAL requests, $CONCURRENCY concurrent ==="
echo "URL: $URL"
echo ""

start=$(date +%s)
seq 1 $TOTAL | xargs -P $CONCURRENCY -I {} curl -sS -w "%{http_code}\n" -o /dev/null -m 30 \
  -X POST "$URL" \
  -H "Authorization: Bearer local-test-key" \
  -H "Content-Type: application/json" \
  -d "$BODY" 2>/dev/null | sort | uniq -c
end=$(date +%s)

echo ""
echo "=== 完成: $((end-start))s ==="
