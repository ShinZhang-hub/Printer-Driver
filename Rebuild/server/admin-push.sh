#!/bin/bash
# Mac 管理端推送配置脚本
# 用法:
#   ./admin-push.sh [config.json]     # 默认推 current.json
# 环境变量:
#   PRINTER_SERVER  默认 https://30.61.39.54:443
#   PRINTER_CACERT  默认 ./server.cer（必须存在，否则直接报错退出，绝不 -k）
# Token 来源: Mac 钥匙串 PrinterConfigAdmin
set -euo pipefail
cd "$(dirname "$0")"

SERVER="${PRINTER_SERVER:-https://30.61.39.54:443}"
CACERT="${PRINTER_CACERT:-server.cer}"
FILE="${1:-current.json}"

[ -f "$FILE" ] || { echo "找不到 ${FILE}，先 GET 存一份：curl --cacert $CACERT $SERVER/api/v1/config -o $FILE"; exit 1; }
python3 -m json.tool "$FILE" > /dev/null || { echo "$FILE 不是合法 JSON"; exit 1; }

# 最小 schema 校验：顶层必须有 version(整数) 和 locations(数组)
python3 - "$FILE" <<'EOF' || { echo "$FILE 缺少必要字段 (version/locations)"; exit 1; }
import json, sys
c = json.load(open(sys.argv[1]))
assert isinstance(c.get("version"), int), "version 必须为整数"
assert isinstance(c.get("locations"), list), "locations 必须为数组"
EOF

# 证书必须存在，绝不 -k 跳过校验
[ -f "$CACERT" ] || { echo "错误: 找不到 ${CACERT}（可用 PRINTER_CACERT 指定），拒绝不校验推送"; exit 1; }

TOK="$(security find-generic-password -s "PrinterConfigAdmin" -w 2>/dev/null)" || { echo "钥匙串无 PrinterConfigAdmin，先添加"; exit 1; }
[ -n "$TOK" ] || { echo "token 为空"; exit 1; }

# 推送前备份远端现行配置，方便回滚
BACKUP="backup-$(date +%Y%m%d-%H%M%S).json"
echo "-> 备份远端现行配置到 $BACKUP"
curl -sS --fail --cacert "$CACERT" "$SERVER/api/v1/config" -o "$BACKUP" \
  || { echo "备份失败（远端不可达或证书/认证有问题），停止推送"; exit 1; }

# token 经临时 header 文件传入，避免 ps 可见
HDR="$(mktemp)"; RESP="$(mktemp)"; trap 'rm -f "$HDR" "$RESP"' EXIT
printf 'Authorization: Bearer %s' "$TOK" > "$HDR"

echo "-> POST $SERVER/api/v1/config ($FILE)"
set +e
HTTP_CODE="$(curl -sS -o "$RESP" -w "%{http_code}" --cacert "$CACERT" -X POST "$SERVER/api/v1/config" \
  -H "Content-Type: application/json" \
  -H @"$HDR" \
  --data-binary "@$FILE")"
RC=$?
set -e
if [ "$RC" -ne 0 ]; then
  echo "推送请求失败（curl exit ${RC}），远端备份在 $BACKUP"
  exit 1
fi
case "$HTTP_CODE" in
  2*) echo "推送成功：HTTP ${HTTP_CODE}（远端备份在 $BACKUP）" ;;
  *) echo "推送被拒绝：HTTP ${HTTP_CODE}，服务端返回："; cat "$RESP"; echo; echo "远端备份在 $BACKUP"; exit 1 ;;
esac
