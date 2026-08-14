# shared-ui — 打印机配置 UI 资产（onboarding 内嵌）

从独立安装器（`app/`）抽取出的**与窗口壳无关**的可复用 UI 部分，
位于 onboarding 项目内部（`src/shared-ui/`），直接引用，保持观感与交互一致。

## 组成

| 文件 | 内容 |
|---|---|
| `style.css` | 通用样式：变量、卡片分组、摘要行、勾选行、下拉框、分隔线、删除列表、关键词高亮 `.hl`、按钮、结果区。**不含** titlebar / fitWindow / 静默启动等窗口壳样式。 |
| `printer-ui.js` | 渲染 + 交互逻辑模块：`createPrinterUI(opts)` 返回一个实例，内部包含多语言（`t`/`tHTML`）、确认界面渲染、位置切换联动、删除勾选防误删、结果分组展示。 |

## 用法（onboarding）

**1. HTML**：link 样式，并按约定的 DOM id 放置 confirm 界面骨架
（可复用独立 app `index.html` 的 confirm 段结构）。

```html
<link rel="stylesheet" href="../../shared-ui/style.css" />

<div id="summary" class="summary"></div>
<label class="row" id="confirm-row"><input type="checkbox" id="chk-confirm" checked /><span id="confirm-label"></span></label>
<div id="picker-wrap" hidden>
  <label id="picker-label"></label>
  <select id="picker"></select>
</div>
<p id="conflict-label"></p>
<select id="conflict"></select>
<p id="existing-label"></p>
<div id="delete-list"></div>
<div class="btns"><button id="btn-cancel" class="secondary"></button><button id="btn-ok"></button></div>
<div id="result-body"></div>
```

**2. JS**：把 4 个 command 包装成 opts 传入即可。

```js
import { createPrinterUI } from "../../shared-ui/printer-ui.js";

const invoke = window.__TAURI__.core.invoke;

const ui = createPrinterUI({
  getState: () => invoke("get_printer_state"),
  runInstall: (req) => invoke("run_printer_install", { req }),
  getStrings: (lang) => invoke("get_printer_strings", { lang }),
  langBtn: "lang-btn",   // 可选：右上角语言按钮 id
  langDrop: "lang-drop", // 可选：语言下拉 id
});

ui.init();  // 加载状态 + 渲染确认界面 + 绑定事件
// "好"按钮默认调用 runInstall 并展示 result；
// 如需接管：ui.onConfirm = async (req) => { ... };
```

## 与独立安装器的关系

- **shared-ui** = 独立 app 的 `main.js` 渲染部分 + `style.css` 通用样式，
  去掉窗口壳（fitWindow / 静默启动 / 四屏切换 / titlebar 拖拽）。
- 独立 app 保持原样，**不强制**改为引用 shared-ui（避免回归风险）。
- 若未来希望单源维护，可将独立 app 的渲染函数改为 import shared-ui，
  二者共享同一份 UI 逻辑。

## DOM id 约定

`printer-ui.js` 默认按独立 app 的 id 查找元素；宿主 DOM id 不同时，
通过 `opts.ids` 覆盖：

```js
createPrinterUI({ ..., ids: { picker: "my-location-select", btnOk: "submit-btn" } });
```
