#!/bin/bash
# ============================================================================
# reset-printers-mac.sh — 模拟"全新 Mac"的打印环境（测试辅助脚本）
#
# 用途：彻底清理本机打印环境 —— 删除所有打印机队列、端口、驱动、CUPS 持久化
#       配置与缓存，并重启打印相关服务，回到接近新 Mac 的状态，便于测试
#       PrinterInstaller。
#
# 用法：
#   sudo bash Rebuild/scripts/reset-printers-mac.sh
#     （需要 root，脚本内会自动检查）
#
# 可选参数：
#   --no-drivers  仅删打印机队列/配置 + 重启服务，不清驱动
#   --dry-run     只列出将要删除的内容，不实际执行
# ============================================================================
set -u

# ---- 检查 root ----
if [ "$(id -u)" -ne 0 ]; then
  echo "错误：需要 root 权限，请用 sudo 运行。" >&2
  echo "  sudo bash $0" >&2
  exit 1
fi

DRIVERS=1
DRY=0
# 可选：第一个位置参数为真实用户 HOME（双击运行时由 PrinterReset.command 传入，
# 避免提权后 $HOME 变成 /var/root 而漏清当前用户的打印缓存）
USER_HOME="${1:-$HOME}"
for a in "$@"; do
  case "$a" in
    --no-drivers) DRIVERS=0 ;;
    --dry-run) DRY=1 ;;
  esac
done

CUPSSVC="org.cups.cupsd"

run() {
  if [ "$DRY" -eq 0 ]; then
    "$@"
  fi
}

echo "=================================================="
echo " 重置 macOS 打印环境（模拟全新 Mac）"
echo "  dry-run = $DRY  清理驱动 = $DRIVERS"
echo "=================================================="

# ---------------- 1. 删除所有打印机队列 ----------------
echo ""
echo "=== [1/10] 删除所有打印机队列 ==="
# 必须在 cupsd 运行时删除（lpadmin 依赖 CUPS daemon）；
# 若先停服务，lpadmin -x 会失败导致队列残留（PPD 仍被引用 → 后续打印失败）。
QUEUES=$(lpstat -a 2>/dev/null | awk '{print $1}')
if [ -z "$QUEUES" ]; then
  echo "  无打印机队列"
else
  for q in $QUEUES; do
    echo "  删除队列：$q"
    run lpadmin -x "$q" 2>/dev/null
  done
fi

# ---------------- 2. 停打印服务 ----------------
echo ""
echo "=== [2/10] 停止打印服务（${CUPSSVC}）==="
if [ "$DRY" -eq 0 ]; then
  launchctl stop "system/$CUPSSVC" 2>/dev/null
  launchctl stop "$CUPSSVC" 2>/dev/null
  killall cupsd 2>/dev/null
  sleep 1
fi
echo "  已停止 cupsd"

# ---------------- 3. 清理 CUPS 持久化配置 ----------------
echo ""
echo "=== [3/10] 清理 CUPS 持久化配置 ==="
for f in \
  /etc/cups/printers.conf \
  /etc/cups/printers.conf.O \
  /etc/cups/printers.conf.pre-update \
  /etc/cups/classes.conf \
  /etc/cups/classes.conf.O \
  /etc/cups/classes.conf.pre-update; do
  if [ -e "$f" ]; then
    echo "  清理：$f"
    run rm -f "$f"
  fi
done
echo "  （cupsd 重启时会重建默认配置）"

# ---------------- 4. 清理每个打印机的 PPD 副本 ----------------
echo ""
echo "=== [4/10] 清理 /etc/cups/ppd（队列 PPD 副本）==="
if [ -d /etc/cups/ppd ]; then
  CNT=$(ls -A /etc/cups/ppd 2>/dev/null | wc -l | tr -d ' ')
  echo "  共 $CNT 个文件"
  run rm -rf /etc/cups/ppd/* 2>/dev/null
fi

# ---------------- 5. 清理打印作业 / spool ----------------
echo ""
echo "=== [5/10] 清理 /var/spool/cups（作业与临时文件）==="
if [ -d /var/spool/cups ]; then
  CNT=$(ls -A /var/spool/cups 2>/dev/null | wc -l | tr -d ' ')
  echo "  共 $CNT 个文件"
  # 只清文件，保留 cache/ 子目录 —— cupsd 需要它写 job.cache / PID，
  # 删掉会导致作业卡住（之前踩过的坑）。
  run find /var/spool/cups -mindepth 1 ! -name cache -exec rm -rf {} + 2>/dev/null
  run find /var/spool/cups/cache -mindepth 1 -exec rm -rf {} + 2>/dev/null
fi

# ---------------- 6. 清理 CUPS 日志 ----------------
echo ""
echo "=== [6/10] 清理 CUPS 日志（/var/log/cups）==="
if [ -d /var/log/cups ]; then
  CNT=$(ls -A /var/log/cups 2>/dev/null | wc -l | tr -d ' ')
  echo "  共 $CNT 个文件"
  run rm -rf /var/log/cups/* 2>/dev/null
fi

# ---------------- 6. 清理驱动（PPD 与厂商包） ----------------
echo ""
echo "=== [7/10] 清理驱动 ==="
if [ "$DRIVERS" -eq 1 ]; then
  # 6a. PPD 目录中本项目安装的驱动
  DRV_DIR="/Library/Printers/PPDs/Contents/Resources"
  FOUND=0
  for f in \
    "$DRV_DIR/ff-mac-driver.ppd" \
    "$DRV_DIR/FF Print Driver for Mac OS X.ppd" \
    "$DRV_DIR/FF Print Driver for Mac OS X.gz"; do
    if [ -e "$f" ]; then
      echo "  删除驱动：$f"
      FOUND=1
      run rm -f "$f"
    fi
  done

  # 6b. FUJIFILM 厂商驱动包目录（本项目驱动来源）
  if [ -d /Library/Printers/FUJIFILM ]; then
    echo "  删除厂商驱动包：/Library/Printers/FUJIFILM"
    FOUND=1
    run rm -rf /Library/Printers/FUJIFILM
  fi

  # 6c. 已安装打印机记录中的 FF 条目（InstalledPrinters.plist）
  PLIST="/Library/Printers/InstalledPrinters.plist"
  if [ -e "$PLIST" ]; then
    echo "  清理 InstalledPrinters.plist 中的 FF 条目：$PLIST"
    run /usr/libexec/PlistBuddy -c "Delete :InstalledPrinters" "$PLIST" 2>/dev/null
    run /usr/libexec/PlistBuddy -c "Delete :Manufacturers" "$PLIST" 2>/dev/null
    echo "    已清空记录（cupsd 将按需重建）"
  fi

  [ "$FOUND" -eq 0 ] && echo "  未发现本项目安装的驱动"
else
  echo "  --no-drivers 已指定，跳过驱动清理"
fi

# ---------------- 7. 清理用户级打印缓存 ----------------
echo ""
echo "=== [8/10] 清理用户级打印缓存（${USER_HOME}）==="
if [ -d "$USER_HOME/Library/Printers" ]; then
  echo "  清理：$USER_HOME/Library/Printers"
  run rm -rf "$USER_HOME/Library/Printers"
fi
if [ -d "$USER_HOME/Library/Application Support/CUPS" ]; then
  echo "  清理：$USER_HOME/Library/Application Support/CUPS"
  run rm -rf "$USER_HOME/Library/Application Support/CUPS"
fi

# ---------------- 8. 重启打印服务 ----------------
echo ""
echo "=== [9/10] 重启打印服务（${CUPSSVC}）==="
if [ "$DRY" -eq 0 ]; then
  launchctl start "system/$CUPSSVC" 2>/dev/null
  launchctl start "$CUPSSVC" 2>/dev/null
  sleep 3
fi

# ---------------- 9. 验证 ----------------
echo ""
echo "=== [10/10] 验证 ==="
echo "  剩余打印机队列："
if lpstat -a 2>/dev/null | grep -q .; then
  lpstat -a 2>/dev/null
else
  echo "    （空）"
fi
echo "  cupsd 状态："
pgrep -x cupsd >/dev/null && echo "    cupsd 运行中" || echo "    cupsd 未运行（异常）"
echo "  默认打印机："
lpstat -d 2>/dev/null || echo "    （无）"

echo ""
echo "完成。打印环境已重置为接近全新 Mac 的状态。"
