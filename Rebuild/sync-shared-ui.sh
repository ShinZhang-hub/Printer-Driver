#!/usr/bin/env bash
# 同步 shared-ui：以 src 为唯一源头，同步到 Rebuild 根目录。
#
# 源：Rebuild/examples/onboarding-minimal/src/shared-ui/（唯一真源，改这里）
# 目标：Rebuild/shared-ui/（镜像，勿直接改，会被覆盖）
#
# 用法：bash Rebuild/sync-shared-ui.sh
# 建议：改完 src/shared-ui 后执行一次，或在 CI / 发布前执行。

set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
SRC="$ROOT/examples/onboarding-minimal/src/shared-ui"
DST="$ROOT/shared-ui"

if [ ! -d "$SRC" ]; then
  echo "错误：源目录不存在 $SRC" >&2
  exit 1
fi

mkdir -p "$DST"

# 同步文件（保留 DST 结构 = 仅包含 SRC 中的文件；旧的镜像文件会被清除）
rm -rf "$DST"/*
cp -R "$SRC"/. "$DST"/

echo "已同步：$SRC  →  $DST"
echo "文件列表："
ls -1 "$DST"
