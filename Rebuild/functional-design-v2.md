# PrinterInstaller 功能设计（macOS + Windows 双平台）

> 本文档为 macOS 与 Windows 双平台的统一功能设计，覆盖两版的实际实现。
> 结构沿用原 `functional-design.md`，在涉及平台差异的章节补充对照表与
> 实现细节，确保设计、代码与交付形态一致。

---

## 1. 项目目标

打印机驱动一键安装工具。用户启动程序即可：

1. 自动识别用户当前所在位置
2. 弹窗让用户确认（可换位置、可覆盖重装、可删除其它打印机）
3. 一次授权完成「安装 / 覆盖 / 删除」

核心原则：**零命令行、零手动配置、按位置自动安装正确的打印机**。

双平台（macOS + Windows）共享同一套前端与业务逻辑，仅安装引擎、
驱动资源、授权方式按平台分叉。

---

## 2. 用户与角色

<table>
  <tr><th>角色</th><th>场景</th></tr>
  <tr><td>普通用户</td><td>双击运行 → 勾选+确认 → 完成安装/删除</td></tr>
  <tr><td>管理员</td><td>查看安装 log；安装时系统授权</td></tr>
</table>

---

## 3. 用户主流程

### 3.1 正常安装（双击即用）

启动后依次执行：

- **步骤一 启动（静默）**：窗口以 `visible:false` 创建，**不显示**，避免白屏 /
  loading 闪现；后台开始定位与加载。
- **步骤二 定位**：定位 + 交互界面渲染完成后，才 `show()` 窗口，弹出确认窗
  （见第 4 节）。
- **步骤三 确认**：用户确认后进入分支：
  - 若选择「跳过」且该位置打印机均已存在 → 直接提示「无需操作」，流程结束
    （不请求授权）。
  - 否则显示「正在安装/删除...」进度提示，随后请求系统授权（Mac: `osascript`；
    Windows: UAC）。
  - 授权通过后执行「安装 / 覆盖 + 删除」。
- **步骤四 收尾**：
  - 成功 → 汇总结果（安装 / 覆盖 / 跳过 / 删除）并提示。
  - 失败 → 展示可读错误信息。

**静默启动**：窗口初始 `visible:false`；`get_initial_state` 返回 + confirm 界面
渲染完成后前端调用 `win.show()`。加载失败分支同样先 `show()` 再显示错误。

### 3.2 手动选择位置

- 未自动识别到位置，或用户取消「自动识别」勾选时，从位置下拉框手动选择。
- 切换位置后，冲突与可删除列表即时联动更新。

### 3.3 管理后台

- 本地服务器设置管理后台，可：
  - 查看 / 修改 / 保存 / 刷新全局配置
- 客户端启动时从配置中心拉取最新配置（见第 7 节）。

---

## 4. 确认弹窗设计

### 4.1 弹窗布局（自上而下）

```
[titlebar] 标题（可拖拽区，右移避开红绿灯/关闭按钮，垂直居中）

摘要行（位置 | 打印机名 | IP）

──────────────────────────────

☑ 自动识别到 <位置>，取消勾选可手动选择其他位置

    [位置下拉 ▼]          ← 勾选时隐藏

──────────────────────────────

该位置 IP 下已有打印机，请选择：

    [跳过 ▼]              ← 无冲突时置灰

──────────────────────────────

本机已有打印机 (N)，如需移除请勾选：

  ☐ 打印机A (IP)

  ☐ 打印机B (IP)          ← 数量多时进入滚动区

──────────────────────────────

              [取消]  [好]
```

### 4.2 视觉规范

- 窗口固定宽 **480px**，高度按内容自适应（fitWindow 仅紧贴高度，不重设宽度）。
- **titlebar**：高 28px，`padding: 3px 0 0 78px`（标题右移避开红绿灯按钮，
  标题中心与按钮中心垂直对齐）；`data-tauri-drag-region` 支持整条拖拽，
  与焦点状态无关，可连续拖拽。
- **关键词高亮**：冲突文案中的「覆盖 / 跳过」、已有打印机数量数字用
  `**…**` 标记 → 渲染为近黑（`#1d1d1f`）加粗，两侧留 1 个空格。
- **result 屏**：消息按内容收缩窗口高度，在 titlebar 下沿与「好」按钮上沿
  之间垂直居中（`flex + justify-content:center`）。
- 语言切换菜单（🌐）固定在右上角。

### 4.3 交互规则

<table>
  <tr><th>控件</th><th>规则</th></tr>
  <tr><td>自动识别勾选</td><td>勾选：使用检测到的位置，位置下拉隐藏。取消勾选：展开位置下拉，由用户手动选择。</td></tr>
  <tr><td>位置下拉</td><td>只列出除检测位置外的其它位置。切换位置时，联动刷新「冲突选项」与「删除勾选」状态。</td></tr>
  <tr><td>冲突选项</td><td>仅当目标位置中任一打印机的 IP 已在本机存在时可用（否则置灰）。</td></tr>
  <tr><td>删除勾选</td><td>列出本机全部已有打印机。命中目标位置 IP 的打印机禁用且不可勾选（防止误删自己要装的打印机）。位置切换时重新计算启用/禁用。</td></tr>
  <tr><td>摘要行</td><td>随当前选择的位置实时更新（位置、打印机名、IP）。</td></tr>
</table>

---

## 5. 对话框时序

<table>
  <tr><th>阶段</th><th>提示</th><th>用户操作</th></tr>
  <tr><td>启动</td><td>窗口隐藏（静默），无提示</td><td>无</td></tr>
  <tr><td>定位完成</td><td>确认窗（显示窗口）</td><td>确认 / 取消</td></tr>
  <tr><td>确认后</td><td>「正在安装/删除...」</td><td>可取消，取消即退出（不报成功）</td></tr>
  <tr><td>执行前</td><td>系统授权请求（Mac: 密码 / Windows: UAC）</td><td>允许 / 拒绝</td></tr>
  <tr><td>完成</td><td>结果汇总（成功/覆盖/跳过/删除）</td><td>自动关闭</td></tr>
</table>

关键语义：

- 任何阶段取消都视为**正常退出**，不误报成功或失败。
- 授权只在有实际安装 / 删除任务时才请求（纯「跳过」不打扰用户）。
- 授权流程全程只出现**一次**（安装与删除合并执行）。

---

## 6. 核心业务规则

### 6.1 位置识别

- 依据**本机网络环境**（所在网段）判断位置，与打印机是否在线无关。
- 匹配规则（按顺序）：
  1. 若本机 IP 命中某位置的网段 → 该位置命中。
  2. 本机存在多块网卡时，优先选择命中网段的那个（忽略虚拟网卡 / 本地回环 /
     自动获取异常的地址，如 169.254.*）。
  3. 全部未命中 → 视为未识别，交由用户手动选择。
- 每个位置可对应**多台打印机**，按配置顺序安装，第一台设为默认。

平台差异：

<table>
  <tr><th>环节</th><th>macOS</th><th>Windows</th></tr>
  <tr><td>本机 IP 枚举</td><td><code>ifconfig</code> 解析 <code>inet </code></td><td><code>Get-NetIPAddress -AddressFamily IPv4</code>（PowerShell，规避中文系统前缀问题）</td></tr>
  <tr><td>位置匹配</td><td><code>cidr_contains()</code> 子网包含判断</td><td>相同，平台无关</td></tr>
</table>

### 6.2 安装 / 覆盖 / 跳过

- **跳过**：目标位置所有打印机均已存在 → 不执行任何操作，提示「已存在，无需操作」。
- **覆盖**：目标位置有打印机已存在，且用户选择「覆盖」→ 先移除旧队列再重新安装。
- **正常安装**：其余情况直接安装缺失的打印机。
- 位置含多台打印机时，按顺序执行，单任务失败后延后重试，单台设备最多尝试 2 次
  （两轮制：Round 1 记录失败 → Round 2 重试并给出最终结论）。

### 6.3 删除

- 用户勾选要删除的本机打印机（目标位置的打印机不可选）。
- 删除与安装合并为一次授权执行。
- 空列表 / 哨兵项（如无打印机时的「none」）必须被过滤，不得出现在删除列表。
- 覆盖重装时，目标 IP 上的已有打印机自动并入删除队列。

### 6.4 驱动匹配

- 根据打印机型号（可从设备探测或配置读取）匹配对应驱动。
- 精确匹配优先；无精确匹配时允许按相似度回退；仍无匹配时（限特定平台）可回退
  通用驱动。
- 驱动资源来源优先级：外部驱动包 → 内置驱动包。内置包保证离线 / 新机器可用。

平台差异：

<table>
  <tr><th>环节</th><th>macOS</th><th>Windows</th></tr>
  <tr><td>驱动资源</td><td>内嵌单个 PPD（<code>ff-mac-driver.ppd</code>）</td><td>内嵌完整 INF 驱动包（<code>assets/drivers-windows/</code>，18 个 INF + 194 个配套文件）</td></tr>
  <tr><td>型号匹配</td><td>按 PPD 条目匹配</td><td><code>driver.rs</code> 解析 INF（Manufacturer/Model/Strings），<code>normalize_model</code> + 相似度打分（精确 100 / 同型号数字 80/60 / 包含 50 / 无 0）</td></tr>
  <tr><td>运行时</td><td>PPD 直接写临时文件</td><td><code>build.rs</code> 以字节数组内嵌，运行解包到 <code>%TEMP%\printer-installer-drv-&lt;pid&gt;\</code></td></tr>
</table>

---

## 7. 配置结构

```
全局配置
 ├─ 配置来源地址（config_url，可选的远程配置中心）
 ├─ 默认端口 / 协议
 ├─ 位置列表
 │    ├─ 名称
 │    ├─ 网段（可多个）           ← 用于位置识别
 │    ├─ 打印机列表（一台或多台）
 │    │    └─ IP / 名称 / 型号（可选，端口/协议可覆盖全局）
 └─ 驱动清单（品牌 / 型号 / 启用状态）
```

规则：

- 读取优先级：**远程配置中心（在线）→ 本地缓存（若实现）→ 内置默认配置
  （离线兜底）**。
- 支持向后兼容：旧结构（单个打印机字段）与新结构（打印机数组）等价，
  新结构优先。
- 配置变更自动升版本号并记录更新时间。

### 7.1 配置来源与更新机制

- **内嵌配置**：`printer-core/assets/config.json` 经 `include_str!` 编译期内嵌，
  作为装机时的初始快照与离线兜底。更新它 = 重新发版。
- **远端拉取（静默更新）**：
  - 启动后后台调用 `${config_url}/api/v1/config`（当前 `http://30.61.40.61:9527`）。
  - 2 秒超时、失败静默保留当前配置，**启动不被网络阻塞**。
  - 拉取到新配置后替换内存缓存，并发出 `config-updated` 事件，前端重新加载
    状态并重渲染界面。
  - **持久性**：目前仅进程内生效，进程退出即恢复内嵌旧配置；若需跨启动持久，
    需加本地缓存落盘（启动时本地缓存优先于内嵌）。
- 双平台共用同一 `config.rs`，机制一致。

### 7.2 配置中心（可选）

- 集中存放最新配置，客户端启动时拉取。
- 提供读取接口与受保护的更新接口。
- 不可用时客户端自动使用内置配置，不影响使用。

---

## 8. 多语言

- 界面文案支持**英文 / 日文 / 韩文 / 简体中文 / 繁体中文**，随系统语言自动切换，
  未知语言回退英文（右上角语言切换入口手动选择界面语言）。
- 所有界面文案集中管理，一处维护；逻辑代码不内嵌文案。
- 位置名、打印机名等业务数据来自配置，不参与翻译。
- 各平台（无论实现方式）共用同一套文案，保证多语言行为一致。

语言检测平台差异：

<table>
  <tr><th>平台</th><th>检测方式</th></tr>
  <tr><td>macOS</td><td><code>osascript -e "user locale of (system info)"</code></td></tr>
  <tr><td>Windows</td><td><code>Get-WinUserLanguageList | Select -First 1 .LanguageTag</code>（PowerShell）</td></tr>
</table>

映射规则：`zh` 开头的 locale 中，含 `hant`/`_tw`/`_hk`/`_mo` → `zh-Hant`，
否则 → `zh`。支持 `PRINTER_INSTALLER_LANG` 环境变量覆盖。

---

## 9. 错误处理与边界情况

<table>
  <tr><th>场景</th><th>行为</th></tr>
  <tr><td>无打印机（新机器）</td><td>删除列表为空，不显示幽灵项</td></tr>
  <tr><td>无法识别位置</td><td>隐藏自动识别、显示位置下拉供用户手动选择</td></tr>
  <tr><td>离线（拉不到远程配置）</td><td>使用内置配置继续</td></tr>
  <tr><td>目标位置打印机已全存在</td><td>直接提示「无需操作」（不请求授权）</td></tr>
  <tr><td>执行中被取消</td><td>正常退出，不报成功或失败</td></tr>
  <tr><td>删除列表含目标打印机</td><td>自动禁用，防止误删</td></tr>
  <tr><td>驱动不匹配</td><td>回退相似匹配或通用驱动；仍失败则给出明确错误</td></tr>
  <tr><td>执行失败</td><td>展示可读错误摘要（过滤无关日志噪音），含失败步骤原因码（<code>lpadmin/verify/enable/accept/default/delete</code>）</td></tr>
  <tr><td>初始状态加载失败</td><td>仍显示窗口并展示错误（静默启动的失败分支）</td></tr>
</table>

---

## 10. 平台实现对照

### 10.1 安装引擎

<table>
  <tr><th>步骤</th><th>macOS（CUPS）</th><th>Windows（spooler）</th></tr>
  <tr><td>已装打印机枚举</td><td><code>lpstat -v</code> 解析 <code>socket://</code>（含本地化前缀处理）</td><td><code>Get-Printer</code> + <code>Get-PrinterPort</code> → <code>名字=IP</code></td></tr>
  <tr><td>装驱动</td><td>PPD + <code>lpadmin -E</code></td><td><code>pnputil /add-driver</code>（退出码 5 = 已存在）</td></tr>
  <tr><td>建端口</td><td><code>lpadmin -v socket://ip:port/proto</code></td><td><code>Add-PrinterPort -Name "IP_&lt;ip&gt;"</code></td></tr>
  <tr><td>建队列</td><td><code>lpadmin -p</code></td><td><code>rundll32 printui.dll,PrintUIEntry /if</code></td></tr>
  <tr><td>设默认</td><td><code>cupsenable / cupsaccept / lpadmin -d</code></td><td><code>printui /y</code>（Windows 无启停概念）</td></tr>
  <tr><td>删除</td><td><code>lpadmin -x</code></td><td><code>Remove-Printer</code>（回退 winspool <code>DeletePrinter</code>）</td></tr>
  <tr><td>防锁恢复</td><td>无（CUPS 无此问题）</td><td>杀 <code>splwow64</code>/<code>PrintIsolationHost</code> + 重启 spooler</td></tr>
  <tr><td>授权</td><td><code>osascript</code> 一次性授权（<code>with administrator privileges</code>）</td><td>UAC 一次性提权执行批处理（<code>Start-Process -Verb RunAs</code>）</td></tr>
  <tr><td>结果协议</td><td><code>I-OK/I-FAIL/D-OK/D-FAIL\t&lt;name&gt;[\t&lt;reason&gt;]</code></td><td>完全一致，<code>parse_batch_output</code> 复用</td></tr>
</table>

### 10.2 授权流程对比

- **macOS**：`osascript "do shell script bash <script> with administrator privileges
  with prompt <ADMIN_PROMPT>"`。用户取消密码框（`-128`）→ 视为取消，不报错误。
- **Windows**：生成 PowerShell 批处理 → 若已提权则直接执行；否则 wrapper 脚本
  `Start-Process -Verb RunAs -Wait -PassThru` 触发一次 UAC，退出码编码结果
  （0=成功 / 1=失败 / 2=取消）。全程隐藏控制台窗口（`CREATE_NO_WINDOW` /
  `-WindowStyle Hidden`）。

### 10.3 前端 fitWindow 差异

- macOS：`innerSize/outerSize/setSize` 为物理像素，`getBoundingClientRect()`
  为逻辑像素，经 `scaleFactor` 换算并补回标题栏差值。
- Windows：`innerSize/setSize` 已是逻辑像素，无外框差值，同一公式无害复用。

---

## 11. 分发与安装形态

- 以**单文件应用 / 安装包**形式交付，双击即可用：
  - 自带完整驱动资源，无需用户额外下载或手动操作。
  - 无外网依赖（WebView2 已预装；macOS 系统自带 WebKit）。

平台产物：

<table>
  <tr><th>平台</th><th>交付形态</th></tr>
  <tr><td>macOS</td><td><code>PrinterInstaller.app</code> + <code>.dmg</code>（拖拽安装）</td></tr>
  <tr><td>Windows</td><td><code>PrinterInstaller.exe</code> + <code>.msi</code></td></tr>
</table>

---

## 12. 非功能需求

- **启动快**：窗口静默创建（`visible:false`），交互界面渲染完成才显示，
  避免白屏 / loading 闪现；定位阶段轻量路径。
- **自动化程度高**：默认全自动，绝大多数用户无需任何选择。
- **强防误操作**：不允许删除自己要装的打印机。
- **离线可用**：核心安装流程不依赖外网 / 远程服务（内嵌驱动 + 内嵌配置兜底）。
- **可维护**：文案、配置、驱动资源与逻辑代码解耦，便于重构与本地化扩展。
- **跨平台复用**：前端、i18n、配置、流程编排约 80% 零改动，仅安装引擎与
  授权按平台实现。
