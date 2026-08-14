// 入职指引前端：通过 shared-ui 的 createPrinterUI 复用打印机配置界面，
// 底层 4 个 command 在 Rust 侧转调 printer-core（见 src-tauri/src/lib.rs）。
import { createPrinterUI } from "./shared-ui/printer-ui.js";
import { listen } from "@tauri-apps/api/event";

const invoke = window.__TAURI__.core.invoke;

// 未检测到位置时，强制默认选第一个（onboarding 的 UI 策略）
async function getState() {
  const S = await invoke("get_printer_state");
  if (!S.detected_location && S.locations.length) {
    S.detected_location = S.locations[0];
  }
  return S;
}

const ui = createPrinterUI({
  getState,
  runInstall: (req) => invoke("run_printer_install", { req }),
  getStrings: (lang) => invoke("get_printer_strings", { lang }),
  // simple: true = 只做「选位置 + 安装」，不显示冲突/删除，不覆盖不删除。
  // 【完整版】恢复：删除此行，并取消 index.html 中「②冲突 / ③删除」注释块。
  simple: true,
});

// 主动刷新远端配置：进入打印机步骤时拉一次最新配置；
// 若有更新，Rust 侧会发 "printer-config-updated" 事件 → 重新加载状态并重渲染。
async function refreshAndReload() {
  await invoke("refresh_printer_config");
  // 立即刷新一次状态，让位置列表尽量新（即使事件因时序未触发）。
  await ui.reloadState();
}

listen("printer-config-updated", () => {
  ui.reloadState();
});

ui.init().then(({ S, t }) => {
  console.log("检测位置:", S.detected_location, "| 位置:", S.locations);
  refreshAndReload(); // 主动刷新（fire-and-forget）
});
