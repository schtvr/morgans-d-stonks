#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
KEY="${INTERNAL_API_KEY:-changeme}"
SCHEMA="${MCP_SCHEMA_VERSION:-v1}"

payload='{
  "schema_version":"'"$SCHEMA"'",
  "idempotency_key":"smoke-'"$(date +%s)"'",
  "order":{
    "product_id":"BTC-USD",
    "side":"buy",
    "type":"market",
    "quote_size":25
  }
}'

echo "[1/3] health"
curl -fsS "$API_URL/api/health" >/dev/null

echo "[2/3] validate"
curl -fsS -X POST "$API_URL/mcp/v1/trades/validate" \
  -H "X-Internal-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d "$payload" >/dev/null

echo "[3/3] create"
curl -fsS -X POST "$API_URL/mcp/v1/trades/create" \
  -H "X-Internal-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d "$payload" >/dev/null

echo "smoke_mcp_flow: ok"
