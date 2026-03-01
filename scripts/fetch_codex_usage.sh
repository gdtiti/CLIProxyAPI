#!/bin/bash
# 获取 Codex usage API 实际响应格式
# 用法: ./fetch_codex_usage.sh <auth_json_file>
# 或: ./fetch_codex_usage.sh  (使用 auths 下第一个 codex 文件)

set -e
AUTH_FILE="${1:-}"
if [ -z "$AUTH_FILE" ]; then
  AUTH_DIR="${AUTH_DIR:-/Users/liu/Desktop/CLIProxyAPI/auths}"
  AUTH_FILE=$(ls "$AUTH_DIR"/codex*.json 2>/dev/null | head -1)
  if [ -z "$AUTH_FILE" ]; then
    echo "Usage: $0 <auth_json_file>"
    exit 1
  fi
fi

ACCESS_TOKEN=$(jq -r '.access_token // empty' "$AUTH_FILE")
ACCOUNT_ID=$(jq -r '.account_id // empty' "$AUTH_FILE")
if [ -z "$ACCESS_TOKEN" ] || [ -z "$ACCOUNT_ID" ]; then
  echo "Missing access_token or account_id in $AUTH_FILE"
  exit 1
fi

URL="https://chatgpt.com/backend-api/wham/usage"
OUTPUT="${2:-/tmp/codex_usage_response.json}"

echo "Fetching usage for account $ACCOUNT_ID from $AUTH_FILE"
echo "URL: $URL"
echo ""

curl -sS -w "\n\nHTTP_STATUS:%{http_code}" \
  -X GET "$URL" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -H "User-Agent: codex_cli_rs/0.101.0 (Mac OS 26.0.1; arm64) Apple_Terminal/464" \
  -H "Chatgpt-Account-Id: $ACCOUNT_ID" \
  -o "$OUTPUT.raw"

HTTP_STATUS=$(grep "HTTP_STATUS:" "$OUTPUT.raw" | cut -d: -f2)
head -n -1 "$OUTPUT.raw" | grep -v "HTTP_STATUS:" > "$OUTPUT" 2>/dev/null || true
if [ -s "$OUTPUT" ]; then
  echo "Response (HTTP $HTTP_STATUS) saved to $OUTPUT"
  echo ""
  echo "=== Response body (pretty) ==="
  jq . "$OUTPUT" 2>/dev/null || cat "$OUTPUT"
else
  echo "Response (HTTP $HTTP_STATUS):"
  cat "$OUTPUT.raw"
fi
