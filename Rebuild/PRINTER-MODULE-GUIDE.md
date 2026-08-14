# 🖨️ PrinterInstaller 模块接入指南

> **一句话**：入职指引程序想用打印机安装功能，只需要「引依赖 + 抄 3 个转发函数 + 放一段 HTML + 写几行 JS」。
> 打印机怎么装、授权、重试、翻译——全在 printer-core 里，宿主不用管。
>
> 本指南是**完整接入教程**，从零到跑通。已配套可运行示例：
> `examples/onboarding-minimal/`（printer-core + shared-ui 都接好了）。

---

## 你需要的东西

| 素材 | 位置 | 作用 |
|---|---|---|
| `printer-core/` | Rebuild 根目录 | Rust 业务库：安装、覆盖、删除、授权、重试、多语言 |
| `shared-ui/` | onboarding 项目内 `src/shared-ui/` | 前端 UI 资产：`style.css` + `printer-ui.js`（渲染/交互逻辑） |

> 独立安装器 `app/` 是另一个完整程序，**不需要**引入，仅作参考。

---

## 完整接入步骤（共 6 步）

### 第 1 步：`src-tauri/Cargo.toml` — 加一行依赖

```toml
[dependencies]
tauri = { version = "2", features = [] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
printer-core = { path = "../../../printer-core" }   # ← 这一行
```

> 路径按实际位置调整；同 workspace 可直接相对路径。

### 第 2 步：`src-tauri/src/lib.rs` — 抄这 3 个转发函数（一次性）

```rust
use serde::Deserialize;
use tauri::Emitter;

// 前端传参结构（字段名与 JS 一致）
#[derive(Deserialize)]
struct InstallRequestDto {
    location: String,
    overwrite: bool,
    delete: Vec<String>,
}

// ① 初始状态：位置识别 / 冲突 / 删除列表 / 文案
#[tauri::command]
fn get_printer_state() -> printer_core::InitialState {
    printer_core::initial_state()
}

// ② 执行安装 / 覆盖 / 删除（内部处理授权 + 重试）
#[tauri::command]
fn run_printer_install(
    req: InstallRequestDto,
) -> Result<printer_core::printer::InstallOutcome, String> {
    let cfg = printer_core::load_config();
    let targets = printer_core::printer::targets_for_location(&cfg, &req.location);
    if targets.is_empty() {
        return Err(format!("location '{}' not found in config", req.location));
    }
    let plan = printer_core::printer::InstallRequest {
        location: req.location,
        targets,
        overwrite: req.overwrite,
        delete: req.delete,
    };
    let lang = printer_core::i18n::detect();
    printer_core::printer::run_install(&cfg, &lang, &plan)
}

// ③ 某语言界面文案
#[tauri::command]
fn get_printer_strings(lang: Option<String>) -> std::collections::HashMap<String, String> {
    let lang = lang.unwrap_or_else(printer_core::i18n::detect);
    printer_core::i18n::strings(&lang)
}

// 登记（在 run() 里）
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            get_printer_state,
            run_printer_install,
            get_printer_strings,
            // refresh_printer_config（可选，见第 6 节扩展）
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

### 第 3 步：HTML — 放一段约定 id 的骨架

```html
<link rel="stylesheet" href="/src/shared-ui/style.css" />

<div id="summary" class="summary"></div>

<label class="row" id="confirm-row">
  <input type="checkbox" id="chk-confirm" checked />
  <span id="confirm-label"></span>
</label>
<div id="picker-wrap" hidden>
  <label class="row muted" id="picker-label"></label>
  <select id="picker"></select>
</div>

<p class="muted" id="conflict-label"></p>
<select id="conflict"></select>

<p class="muted" id="existing-label"></p>
<div id="delete-list"></div>

<div id="result-body"></div>
<div class="btns"><button id="btn-ok"></button></div>
```

### 第 4 步：前端 JS — 就这几行

```js
import { createPrinterUI } from "./shared-ui/printer-ui.js";

const invoke = window.__TAURI__.core.invoke;

const ui = createPrinterUI({
  getState: () => invoke("get_printer_state"),
  runInstall: (req) => invoke("run_printer_install", { req }),
  getStrings: (lang) => invoke("get_printer_strings", { lang }),
});

ui.init();   // 渲染整个打印机配置步骤 + 绑定"好"按钮
```

> 可选：没检测到位置时强制默认第一个位置
> ```js
> const S0 = await invoke("get_printer_state");
> if (!S0.detected_location && S0.locations.length) S0.detected_location = S0.locations[0];
> ```

### 第 5 步：`vite.config.js` — 无需额外配置

```js
import { defineConfig } from "vite";

export default defineConfig({
  clearScreen: false,
  server: { port: 1421, strictPort: true },
  build: { outDir: "dist", emptyOutDir: true },
});
```

> shared-ui 已放在 onboarding 项目内（`src/shared-ui/`），import 不越界，
> **无需** `server.fs.allow` 配置。

### 第 6 步：运行

```bash
npm install
npm run tauri dev     # 开发调试
npm run tauri build   # 打包
```

---

## 可选扩展

### 配置刷新（远端配置更新）

```rust
#[tauri::command]
fn refresh_printer_config(app: tauri::AppHandle) {
    std::thread::spawn(move || {
        if printer_core::config::refresh_config() {
            let _ = app.emit_to("main", "printer-config-updated", ());
        }
    });
}
```

前端监听更新事件后可重新 `ui.init()` 刷新界面。

### 自定义 DOM id

printer-ui 默认按独立 app 的 id 查找元素；宿主 id 不同时通过 `ids` 覆盖：

```js
createPrinterUI({
  ...,
  ids: { picker: "my-loc-select", btnOk: "submit-btn" },
});
```

### 接管"好"按钮

```js
ui.onConfirm = async (req) => {
  // 例如：先切到宿主自己的进度页，再执行
  await invoke("run_printer_install", { req });
  // 自己处理 result
};
```

### simple 模式（只需「选位置 + 安装」）

如果入职指引**不需要**覆盖重装、也不需要删除打印机，启用 `simple: true`：

```js
const ui = createPrinterUI({
  getState,
  runInstall: (req) => invoke("run_printer_install", { req }),
  getStrings: (lang) => invoke("get_printer_strings", { lang }),
  simple: true,   // 只做「选位置 + 安装」
});
```

**效果**：
- 界面只显示位置选择 + 「好」按钮，隐藏冲突/删除区域
- 执行时固定传 `overwrite: false, delete: []`
- printer-core 零改动：位置打印机已存在则自动按「跳过」处理

**完整版（含覆盖/删除）恢复**，仅两处改动，shared-ui 无需动：
1. `index.html`：取消「②冲突处理 / ③删除列表」注释块
2. `src/main.js`：删除 `simple: true` 一行

> shared-ui 的完整版逻辑已保留在 `if (!simple)` 分支内，随时可切换。

---

## 常见问题

| 问题 | 解决 |
|---|---|
| `Failed to resolve import .../shared-ui/printer-ui.js` | import 路径写错；shared-ui 在 `src/shared-ui/`，从 `src/main.js` 引用为 `./shared-ui/printer-ui.js` |
| 窗口出现但白屏 | 检查 HTML 骨架的 DOM id 是否齐全、CSS link 路径是否为 `/src/shared-ui/style.css` |
| `location not found in config` | 前端没传位置（`detected_location` 为空且没选下拉） |
| 授权后无反应 | `run_install` 返回 `cancelled`，前端应保持当前页（示例已处理） |
| 想删打印机但勾选框灰色 | 目标位置的打印机默认禁用（防误删），这是预期行为 |
