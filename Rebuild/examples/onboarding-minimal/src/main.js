// 入职指引前端：通过 shared-ui 的 createPrinterUI 复用打印机配置界面，
// 底层 4 个 command 在 Rust 侧转调 printer-core（见 src-tauri/src/lib.rs）。
import { createPrinterUI } from "./shared-ui/printer-ui.js";

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
});

ui.init().then(({ S, t }) => {
  console.log("检测位置:", S.detected_location, "| 位置:", S.locations);
});
