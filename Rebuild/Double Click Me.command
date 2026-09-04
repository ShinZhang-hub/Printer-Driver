#!/bin/bash
# PrinterReset.command — 双击运行：重置 macOS 打印环境（模拟全新 Mac）
#
# 双击本文件会打开一个终端窗口，提示输入管理员密码后执行
# reset-printers-mac.sh 的完整清理。错误信息直接显示在终端中。

# 本文件所在目录
DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$DIR/scripts/reset-printers-mac.sh"

if [ ! -f "$SCRIPT" ]; then
  echo "找不到脚本：$SCRIPT"
  echo "请确认 PrinterReset.command 位于 Rebuild 目录下。"
  read -r -p "按回车键关闭窗口..." _
  exit 1
fi

echo "即将重置打印环境（需要管理员密码）..."
echo "在下面的提示中输入你的 Mac 登录密码。"
echo ""

# sudo 交互提权执行（错误直接显示在终端，便于排查）
sudo -v 2>/dev/null || {
  echo "无法获取管理员权限。"
  read -r -p "按回车键关闭窗口..." _
  exit 1
}
sudo bash "$SCRIPT" "$HOME"

echo ""
echo "清理完成，窗口即将关闭..."
sleep 3
