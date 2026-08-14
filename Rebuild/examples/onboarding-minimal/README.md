# 入职指引接入 printer-core —— 最小示例

本目录是一个完整的最小 Tauri 应用，演示入职指引程序如何复用 `printer-core`
的打印机业务逻辑（位置识别、安装、覆盖、删除、多语言、配置刷新），
**不依赖**独立安装器那套窗口/静默启动/布局代码。

## 目录结构

```
onboarding-minimal/
├─ index.html                 # 演示页面（位置选择 + 删除勾选 + 执行）
├─ package.json / vite.config.js
├─ src/main.js                # 前端调用 4 个 command
└─ src-tauri/
   ├─ tauri.conf.json
   ├─ Cargo.toml              # 通过 path 依赖 printer-core
   ├─ capabilities/default.json
   └─ src/lib.rs              # 4 个 Tauri command，转调 printer-core
```

## printer-core 是怎么被调用的

关键就一步：**在 Cargo.toml 加 path 依赖**，然后直接调用其公开 API。

```toml
# src-tauri/Cargo.toml
[dependencies]
printer-core = { path = "../../../printer-core" }
```

`src-tauri/src/lib.rs` 里 4 个 command 直接转调 printer-core：

| 前端 command | printer-core 调用 | 用途 |
|---|---|---|
| `get_printer_state` | `printer_core::initial_state()` | 位置识别、冲突、删除列表、文案 |
| `run_printer_install` | `printer_core::printer::run_install(&cfg, &lang, &plan)` | 安装/覆盖/删除（内部含两平台授权 + 两轮重试） |
| `get_printer_strings` | `printer_core::i18n::strings(&lang)` | 当前语言界面文案 |
| `refresh_printer_config` | `printer_core::config::refresh_config()` | 后台刷新远端配置 |

## 前端怎么联动（src/main.js）

```js
const invoke = window.__TAURI__.core.invoke;

// 1. 拿初始状态（返回 flow::InitialState 的 JSON）
const S = await invoke("get_printer_state");
//    S.detected_location / S.locations / S.existing / S.conflict ...

// 2. 执行安装（Rust 侧组装 InstallRequest 调 run_install）
const out = await invoke("run_printer_install", {
  req: { location: "Osaka - JP Tower", overwrite: false, delete: ["Printer-BG"] },
});
//    out.messages = [{ kind, text }, ...]（本地化结果，含失败原因）
```

## 运行

```bash
cd examples/onboarding-minimal
npm install
npm run tauri dev
```

## 与独立安装器的复用/差异

| 项 | 复用自 printer-core | 入职指引自己的 |
|---|---|---|
| 位置识别 / 冲突 / 删除列表 | `flow::initial_state()` | 无 |
| 安装 / 覆盖 / 删除 / 重试 | `printer::run_install()` | 无 |
| 多语言文案 / 语言检测 | `i18n::strings()` / `i18n::detect()` | 无 |
| 授权弹窗（osascript / UAC） | 内嵌在 run_install 内部 | 无 |
| 窗口布局 / 静默启动 / fitWindow | **不引入** | 宿主自己的 UI |
| Tauri command 桥 | 见 lib.rs | 按宿主前端需要增减 |

## 本次为让示例可编译所做的 printer-core 小改动

- `printer::InstallOutcome` 增加 `serde::Serialize` 派生
  （使其可作为 Tauri command 返回值直接传给前端；主安装器不受影响）
