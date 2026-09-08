#!/bin/bash
# Mac 管理端推送配置脚本
# 用法:
#   ./admin-push.sh [config.json]     # 默认推 current.json
# 环境变量:
#   PRINTER_SERVER  默认 https://30.61.39.54:443
#   PRINTER_CACERT  默认 ./server.cer（找不到则用 -k 跳过校验，仅测试）
# Token 来源: Mac 钥匙串 PrinterConfigAdmin（见 README）
set -euo pipefail

SERVER="${PRINTER_SERVER:-https://30.61.39.54:443}"
CACERT="${PRINTER_CACERT:-server.cer}"
FILE="${1:-current.json}"

[ -f "$FILE" ] || { echo "找不到 $FILE，先 GET 存一份：curl --cacert $CACERT $SERVER/api/v1/config -o $FILE"; exit 1; }
python3 -m json.tool "$FILE" > /dev/null || { echo "$FILE 不是合法 JSON"; exit 1; }

TOK="$(security find-generic-password -s "PrinterConfigAdmin" -w)" || { echo "钥匙串无 PrinterConfigAdmin，先添加"; exit 1; }
[ -n "$TOK" ] || { echo "token 为空"; exit 1; }

CURL_FLAGS=(-sS -w "\nHTTP:%{http_code}\n")
if [ -f "$CACERT" ]; then
  CURL_FLAGS+=(--cacert "$CACERT")
else
  echo "警告: 找不到 $CACERT，用 -k 跳过证书校验（仅测试）"
  CURL_FLAGS+=(-k)
fi

echo "-> POST $SERVER/api/v1/config ($FILE)"
curl "${CURL_FLAGS[@]}" -X POST "$SERVER/api/v1/config" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOK" \
  --data-binary "@$FILE"
