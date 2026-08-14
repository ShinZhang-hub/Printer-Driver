# reset-ff.ps1 - 一键清理 Fujifilm 打印机残留，回到"未安装状态"
# 覆盖：打印机队列 / 幽灵打印机 / IP 端口 / 驱动存储包 / 后台服务重启
# 用法：powershell -ExecutionPolicy Bypass -File .\reset-ff.ps1
#       或右键 "使用 PowerShell 运行"（自动提权，会弹 UAC）

# ---- 自动提权 ----
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Host "需要管理员权限，正在提权..." -ForegroundColor Yellow
  Start-Process powershell.exe -Verb RunAs -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File',($MyInvocation.MyCommand.Path)
  exit
}

$ErrorActionPreference = 'Continue'
# 配置里 Fujifilm 打印机的 IP（用于定向清理端口，避免误删 Brother 等）
$ffIps = @('30.61.40.40','30.61.30.30','30.61.34.29','30.61.34.30')

Write-Host ""
Write-Host "===== 1) 删除 Fujifilm 打印机队列 =====" -ForegroundColor Cyan
$ffQueues = @(Get-Printer -ErrorAction SilentlyContinue |
  Where-Object { $_.DriverName -match 'Apeos|FUJIFILM|Fuji' })
if ($ffQueues.Count -eq 0) {
  Write-Host "  无 Fujifilm 队列" -ForegroundColor Gray
} else {
  foreach ($p in $ffQueues) {
    Remove-Printer -Name $p.Name -Confirm:$false -ErrorAction SilentlyContinue
    if (Get-Printer -Name $p.Name -ErrorAction SilentlyContinue) {
      Write-Host "  [X] 删除失败: $($p.Name)" -ForegroundColor Red
    } else {
      Write-Host "  [OK] 已删除: $($p.Name)" -ForegroundColor Green
    }
  }
}

Write-Host ""
Write-Host "===== 2) 清理幽灵打印机(注册表残留) =====" -ForegroundColor Cyan
$realQueues = @(Get-Printer -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name)
$ghostKey = 'HKLM:\SYSTEM\CurrentControlSet\Control\Print\Printers'
$ghostFound = $false
Get-ChildItem $ghostKey -ErrorAction SilentlyContinue | ForEach-Object {
  $n = $_.PSChildName
  if ($realQueues -notcontains $n) {
    Remove-Item "$ghostKey\$n" -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path "$ghostKey\$n") {
      Write-Host "  [X] 删除失败: $n" -ForegroundColor Red
    } else {
      Write-Host "  [OK] 已删除幽灵打印机: $n" -ForegroundColor Green
      $ghostFound = $true
    }
  }
}
if (-not $ghostFound) { Write-Host "  无幽灵打印机" -ForegroundColor Gray }

Write-Host ""
Write-Host "===== 3) 删除 Fujifilm 的 IP 端口 =====" -ForegroundColor Cyan
$used = @(Get-Printer -ErrorAction SilentlyContinue |
  Where-Object { $_.PortName -like 'IP_*' -or $_.PortName -in $ffIps } |
  Select-Object -ExpandProperty PortName)
$portKey = 'HKLM:\SYSTEM\CurrentControlSet\Control\Print\Monitors\Standard TCP/IP Port\Ports'
$del = @()
foreach ($ip in $ffIps) {
  if ($used -notcontains "IP_$ip") { $del += "IP_$ip" }
  if ($used -notcontains $ip) { $del += $ip }
}
foreach ($port in $del) {
  if (Get-PrinterPort -Name $port -ErrorAction SilentlyContinue) {
    Remove-PrinterPort -Name $port -ErrorAction SilentlyContinue
    if (-not (Get-PrinterPort -Name $port -ErrorAction SilentlyContinue)) {
      Write-Host "  [OK] 已删除端口: $port" -ForegroundColor Green
    } else {
      # cmdlet 删不掉 -> 直接清注册表
      Remove-Item "$portKey\$port" -Recurse -Force -ErrorAction SilentlyContinue
      if (Test-Path "$portKey\$port") {
        Write-Host "  [X] 删除失败: $port" -ForegroundColor Red
      } else {
        Write-Host "  [OK] 已删除端口(注册表): $port" -ForegroundColor Green
      }
    }
  }
}

Write-Host ""
Write-Host "===== 4) 重启打印后台服务(释放引用) =====" -ForegroundColor Cyan
Restart-Service Spooler -Force -ErrorAction SilentlyContinue
Write-Host "  [OK] Spooler 已重启" -ForegroundColor Green

Write-Host ""
Write-Host "===== 5) 删除驱动存储里的 Fujifilm 驱动包 =====" -ForegroundColor Cyan
# pnputil 输出字段是本地化的，按「发布名称 oem##.inf + 原始 inf 名」关联定位，语言无关
$current = $null
$deleted = 0
foreach ($ln in (pnputil /enum-drivers 2>$null)) {
  if ($ln -match 'oem\d+\.inf') { $current = ($ln.Trim() -replace '.*(oem\d+\.inf).*', '$1') }
  elseif ($ln -match 'ffsb2plwj\.inf|ffsobplwj\.inf') {
    if ($current -match '^oem\d+\.inf$') {
      Write-Host "  -> 删除 $current" -ForegroundColor Yellow
      pnputil /delete-driver $current /force 2>&1 | ForEach-Object { Write-Host "     $_" }
      $deleted++
      $current = $null
    }
  }
}
if ($deleted -eq 0) { Write-Host "  驱动存储中无 FF 驱动包" -ForegroundColor Gray }

Write-Host ""
Write-Host "===== 6) 验证 =====" -ForegroundColor Cyan
$ffLeft = @(Get-Printer -ErrorAction SilentlyContinue |
  Where-Object { $_.DriverName -match 'Apeos|FUJIFILM|Fuji' })
Write-Host "  Fujifilm 队列剩余: $($ffLeft.Count) 个"
$ffPorts = @(Get-PrinterPort -ErrorAction SilentlyContinue |
  Where-Object { $_.Name -match 'IP_(30\.61\.40\.40|30\.61\.30\.30|30\.61\.34\.29|30\.61\.34\.30)|^(30\.61\.40\.40|30\.61\.30\.30|30\.61\.34\.29|30\.61\.34\.30)$' })
Write-Host "  Fujifilm 端口剩余: $($ffPorts.Count) 个"
$ffDrv = @(pnputil /enum-drivers 2>$null | Select-String 'ffsb2plwj|ffsobplwj')
Write-Host "  驱动存储 FF 剩余: $($ffDrv.Count) 条"
if ($ffLeft.Count -eq 0 -and $ffPorts.Count -eq 0 -and $ffDrv.Count -eq 0) {
  Write-Host "  完成：已回到未安装状态。" -ForegroundColor Green
} else {
  Write-Host "  仍有残留，请检查上面的失败项。" -ForegroundColor Red
}
Write-Host ""
Read-Host "按回车键退出"
