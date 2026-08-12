# PrinterInstaller Windows 版设计文档

> 本文描述 Windows 环境下 App 的全部设计：复用面、平台差异、驱动资源、
> 安装引擎、提权方案与交付形态。功能语义遵循 `functional-design.md`，
> 此处只补充 Windows 平台特有的设计决策。

---

## 1. 总体策略：最大化复用 Rebuild（Rust/Tauri）

Windows 版不另起炉灶，沿用现有 `Rebuild/` 工作区结构与分层：

```
Rebuild/
├─ Cargo.toml                 # workspace
├─ app/                       # Tauri 应用（前端 + src-tauri）
│  ├─ index.html / src/main.js / src/style.css   ← 前端，原样复用
│  └─ src-tauri/src/lib.rs    ← Tauri commands，原样复用
└─ printer-core/              # 平台无关业务核心
   ├─ src/config.rs           ← 原样复用（配置解析/远程刷新/缓存）
   ├─ src/flow.rs             ← 原样复用（状态构建/冲突/联动）
   ├─ src/i18n.rs             ← 只加 Windows 语言检测分支
   ├─ src/location.rs         ← 已有 Windows 分支，切换为 PowerShell 实现
   ├─ src/printer.rs          ← 业务层复用，`imp` 模块重写为 Windows
   └─ src/driver.rs           ← 新增：Windows 驱动匹配（移植 Go match.go）
```

**不变的部分（一行不改）**：
- 前端 4 页流程：loading → confirm → progress → result
- 480px 自适应窗口 + 右上角语言菜单（en/ja/ko/zh/zh-Hant）
- Tauri command：`get_initial_state` / `confirm` / `get_strings` /
  `refresh_config` / `quit`
- `flow::initial_state()` 的全部编排逻辑
- `printer::run_install()` 的规划 / 跳过 / 两轮重试 / 结果消息语义
- `parse_batch_output()` 的 `I-OK/I-FAIL/D-OK/D-FAIL` 标签协议（Windows
  安装脚本继续产生这些标签，解析代码完全复用）

---

## 2. 平台差异对照

| 层面 | macOS（现状） | Windows（本设计） |
|---|---|---|
| 驱动资源 | 内嵌 `ff-mac-driver.ppd` | 内嵌 INF 驱动包（见 §3） |
| 本机 IP 枚举 | `ifconfig` | `Get-NetIPAddress`（PowerShell，本地化无关） |
| 已装打印机枚举 | `lpstat -v` 解析 `socket://` | `Get-Printer` + `Get-PrinterPort` → `名=IP` |
| 装驱动 | PPD + `lpadmin -E` | `pnputil /add-driver` |
| 建端口 | `lpadmin -v socket://…` | `Add-PrinterPort -Name "IP_x.x.x.x"` |
| 建队列 | `lpadmin -p` | `rundll32 printui.dll,PrintUIEntry /if` |
| 启/收/默认 | `cupsenable/cupsaccept/lpadmin -d` | `printui /y` 设默认（Windows 无启停概念） |
| 删除 | `lpadmin -x` | `Remove-Printer` + winspool `DeletePrinter` 回退 |
| 删除锁恢复 | 无（CUPS 无此问题） | 杀 splwow64/PrintIsolationHost + 重启 spooler |
| 管理员提权 | `osascript` 一次性授权 | UAC 一次性提权执行批处理（见 §5） |
| 语言检测 | `osascript user locale` | `Get-WinUserLanguageList` |

---

## 3. 驱动资源：内嵌 INF 包

### 3.1 资源来源与构成

复用仓库内已有的 `internal/embeds/drivers/`（已拉取本地）：

- 18 个 `FF*.INF`（每个对应一款富士打印机，含 ModelName/硬件 ID/安装节）
- 194 个配套文件（`DXDT`/`DXDC`/`gp_`/`dl_`/`ic_` 等驱动二进制，
  INF 的 CopyFiles 指令运行时引用它们）约 11.2 MB
- **不含** 28MB 的 `ffopkplw250320w646fml.exe`（InnoSetup 安装器，
  pnputil 不需要它）

### 3.2 内嵌方式

INF 与全部配套文件作为 `include_bytes!` 静态资源编入 `printer-core`，
**离线可用**（与现有"核心流程不依赖外网"设计原则一致，见
functional-design.md §10）。新增 `Rebuild/printer-core/assets/drivers-windows/`
目录，构建时整体内嵌。

### 3.3 运行时解包

- 首次安装时把内嵌 INF 包解包到 `%TEMP%\printer-installer-drv-<pid>\`
- 安装完成后清理
- INI 目录结构必须按 INF 的 CopyFiles 相对路径原样还原，
  `pnputil /add-driver` 才能解析

### 3.4 型号匹配（新增 `driver.rs`）

移植 Go `internal/drvpack/match.go` 与 `inf.go` 的解析/匹配逻辑：

1. **解析 INF**：读取 `[Manufacturer]` / `<model 节>.ntamd64` / `[Strings]`，
   提取 `ModelName` + `InstallSection` + HardwareID，解析 `%STR%` 字符串引用
2. **匹配**：`normalizeModel` 去除品牌前缀（FF/Fujifilm/HP/…）→ 型号数字
   提取 → 相似度打分（精确 100 / 同型号号 80/60 / 包含 50 / 无 0）
3. **优先级**：`printers[].model`（配置）→ 严格匹配 → 模糊回退

与原 Mac 实现的差异仅是"匹配对象是 INF 条目"而非 PPD 条目，算法一致。

---

## 4. Windows 安装引擎（重写 `printer.rs` 的 `imp` 模块）

### 4.1 设计原则（与 Mac 一致）

- **一次 UAC 授权** 完成全部 安装/覆盖/删除（functional-design.md §5 要求
  全程只弹一次授权）
- 保持两轮重试语义：第 1 轮记录失败项，第 2 轮重试并给出最终结论
- 输出 `I-OK / I-FAIL / D-OK / D-FAIL` tab 分隔标签，复用
  `parse_batch_output()`

### 4.2 枚举已装打印机（替代 `lpstat -v`）

```powershell
Get-Printer | ForEach-Object {
  $port = Get-PrinterPort -Name $_.PortName -ErrorAction SilentlyContinue
  if ($port) {
    $ip = if ($port.Name -match '^IP_(\d+\.\d+\.\d+\.\d+)$') { $matches[1] }
           elseif ($port.HostAddress) { $port.HostAddress } else { $null }
    if ($ip) { $_.Name + "=" + $ip }
  }
}
```

输出 `名字=IP` 行，喂给现有 `printers_by_ip()` → 冲突检测 / 删除禁用逻辑
全部复用。

### 4.3 安装单台打印机（install_one）

按序（全部带隐藏窗口 + 失败即置 `LAST_REASON`，同 Mac）：

1. **移除旧同名/同 IP 打印机**（先在机器上确认旧队列已消失）
2. **装驱动**：`pnputil /add-driver <解包后的 INF>`（退出码 5 视为已存在）
3. **建端口**：`Add-PrinterPort -Name "IP_<ip>" -PrinterHostAddress <ip>
   -PortNumber 9100`（存在则先 `Remove-PrinterPort`）
4. **建队列**：`rundll32 printui.dll,PrintUIEntry /if /b "<name>"
   /f "<INF>" /r "IP_<ip>" /m "<ModelName>"`
5. **设默认**：仅 `is_default` 目标 → `rundll32 printui.dll,PrintUIEntry
   /y /n "<name>"`

reason 码沿用现有协议：`lpadmin/verify/enable/accept/default/delete` 保持
枚举不变（i18n 文案无需改），内部含义映射到 Windows 步骤。

### 4.4 删除打印机（delete_one）

1. `Remove-Printer -Name "<name>"`（失败不立即报错）
2. 回退：winspool.drv `OpenPrinterW` + `DeletePrinter`
3. 访问被拒绝（错误码 5）→ 防锁恢复：杀 `splwow64.exe` /
   `PrintIsolationHost.exe` → `sc stop spooler` → `sc start spooler` →
   重试

### 4.5 驱动安装过程的窗口抑制

参考 `api_windows.go` 的 `HideDriverWindowsLoop`：驱动安装时循环隐藏
`ffcomist.exe` / `Launcher.exe` / "Printer Driver Installation" 窗口，
避免安装中弹窗打断用户。Windows 消息钩子在批处理内部由提权进程完成。

---

## 5. 一次 UAC 授权方案

### 5.1 流程

```
confirm click（前端）
   │
   ▼
[Tauri confirm command]
   ├─ 先生成安装计划 + 内嵌驱动解包
   ├─ 生成 PowerShell 批处理脚本（含两轮重试）
   ├─ 检测当前是否已提权？
   │    ├─ 是 → 直接执行（不弹窗）
   │    └─ 否 → UAC 提权执行一次（ShellExecute runas）
   ▼
收集 I-OK/I-FAIL/D-OK/D-FAIL → parse_batch_output
   │
   ▼
返回 messages 给前端 result 页
```

### 5.2 批处理脚本轮廓

```powershell
param($PlanFile, $RetryI, $RetryD)
$LAST_REASON = ""
function InstallOne($d) {
  $name,$ip,$port,$proto,$isdef = $d -split "`t"
  # 删旧 → pnputil → Add-PrinterPort → printui /if → printui /y
  # 任一步失败 → LAST_REASON，return 1
}
function DeleteOne($n) {
  # Remove-Printer → winspool 回退 → return 0/1
}
# Round 1: 记录失败
# Round 2: 重试 + 输出 I-OK/I-FAIL/D-OK/D-FAIL
```

与 Mac 的 bash + 重试文件结构一一对应，只换命令实现。

### 5.3 状态文件与标签协议

- 计划文件：`<pid>.plan`，tab 分隔（`name\tip\tport\tproto\tisdef`），
  PowerShell `-split "\t"` 读回（处理含空格名字）
- 失败重试文件：`<pid>.retry-i` / `<pid>.retry-d`
- 输出标签与 Mac 完全相同，后端解析零改动

---

## 6. 其余平台适配点

| 文件 | 改动 |
|---|---|
| `i18n.rs::detect()` | 加 `#[cfg(windows)]`：`Get-WinUserLanguageList | % { $_.LanguageTag }` 映射到 LANGS（沿用 `map_system_locale`） |
| `location.rs` | Windows 分支由 `ipconfig` 英文前缀解析改为 `Get-NetIPAddress`（中文系统 "IPv4 地址" 前缀问题）。`detected_local_ip` / `cidr_contains` / `match_location` 逻辑不变 |
| 前端 `fitWindow()` | 去掉仅 macOS 的 outer/inner 物理像素补偿；Windows 下 `innerSize`/`setSize` 均为逻辑像素，直接对标即可（保留 `ResizeObserver` 自适应） |
| `tauri.conf.json` | Windows 窗口无边框 + `data-tauri-drag-region` 拖拽沿用；`decorations:false`（与 Mac 一致），语言菜单/字号无需改 |

---

## 7. 交付形态

- `cargo tauri build` → Windows NSIS/MSI 安装包 + 自带内嵌驱动
- 单文件/安装包内嵌全部驱动资源，双击即用，无外网依赖
- WebView2 已预装（本机已验证），无额外运行时依赖

---

## 8. 工作量拆解

| # | 任务 | 说明 |
|---|---|---|
| 1 | 内嵌驱动资源 | assets/drivers-windows 引入 + include_bytes + 解包清理 |
| 2 | `driver.rs` | 移植 INF 解析 + match.go 匹配 |
| 3 | `printer::imp` Windows | 枚举 + 安装 + 删除 + spooler 防锁 |
| 4 | UAC 提权脚本 | 批处理生成 + runas 一次授权 + 标签输出 |
| 5 | `i18n::detect` Windows | Get-WinUserLanguageList |
| 6 | `location.rs` Windows | Get-NetIPAddress 替换 ipconfig |
| 7 | 前端 fitWindow 微调 | Windows 尺寸补偿修正 |
| 8 | Windows 构建验证 | cargo tauri build + 实机安装验证 |

预计复用率：前端/i18n/config/flow/Tauri 层约 80% 零改动，核心增量在
第 3、4 项（约 250–350 行 Rust + 一份 PowerShell 脚本模板）。