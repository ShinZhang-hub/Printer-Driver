/**
 * printer-ui.js — 可复用的打印机配置界面逻辑（shared-ui）
 *
 * 从独立安装器 app/src/main.js 抽取，去掉窗口壳（fitWindow/静默启动/四屏），
 * 只保留"渲染初始状态 + 收集用户选择 + 展示结果"的纯 UI 逻辑。
 *
 * 用法（onboarding）：
 *   import { createPrinterUI } from "../../shared-ui/printer-ui.js";
 *   const ui = createPrinterUI({
 *     getState: () => invoke("get_printer_state"),
 *     runInstall: (req) => invoke("run_printer_install", { req }),
 *     getStrings: (lang) => invoke("get_printer_strings", { lang }),
 *     ids: {...},   // DOM id 映射，默认同独立 app
 *   });
 *   await ui.init();          // 加载状态 + 渲染确认界面
 *   ui.onConfirm = async (req) => { ... };  // 可选：接管"好"按钮
 */

// 默认 DOM id 约定（与独立 app index.html 一致）
const DEFAULT_IDS = {
  summary: "summary",
  confirmRow: "confirm-row",
  confirmLabel: "confirm-label",
  chkConfirm: "chk-confirm",
  pickerWrap: "picker-wrap",
  pickerLabel: "picker-label",
  picker: "picker",
  conflictLabel: "conflict-label",
  conflict: "conflict",
  existingLabel: "existing-label",
  deleteList: "delete-list",
  btnOk: "btn-ok",
  btnCancel: "btn-cancel",
  btnClose: "btn-close",
  resultBody: "result-body",
};

// 语言菜单（可选）
const LANGS = [
  { code: "en", name: "English" },
  { code: "ja", name: "日本語" },
  { code: "ko", name: "한국어" },
  { code: "zh", name: "简体中文" },
  { code: "zh-Hant", name: "繁體中文" },
];

export function createPrinterUI(opts) {
  const api = opts; // { getState, runInstall, getStrings, ids?, langBtn? }
  const ids = { ...DEFAULT_IDS, ...(opts.ids || {}) };
  const $ = (id) => document.getElementById(id);

  let S = null;      // 初始状态（含 strings）
  let lang = "en";
  let chosenLoc = "";

  // ---- 多语言 ----
  function t(key, ...args) {
    let s = S.strings[key] ?? "";
    for (const a of args) {
      s = s.replace("%s", a);
      s = s.replace("%d", a);
    }
    return s;
  }

  function esc(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  // 渲染 **加粗** 标记为高亮 <b>
  function tHTML(key, ...args) {
    let s = S.strings[key] ?? "";
    for (const a of args) {
      s = s.replace("%s", esc(a));
      s = s.replace("%d", esc(a));
    }
    return s.replace(/\*\*(.+?)\*\*/g, '<b class="hl">$1</b>');
  }

  function locIPs(loc) {
    return S.loc_ips[loc] ?? [];
  }

  // ---- 渲染确认界面 ----
  function renderConfirm() {
    $(ids.summary).textContent = [
      S.detected_location ?? t("NO_LOCATION"),
      S.detected_name,
      S.detected_ip ? "IP: " + S.detected_ip : "",
    ]
      .filter(Boolean)
      .join("  |  ");

    $(ids.confirmRow).hidden = !S.detected_location;
    $(ids.confirmLabel).textContent = t("CONFIRM_FMT", S.detected_location ?? "");
    $(ids.chkConfirm).checked = !!S.detected_location;
    updatePickerVisibility();

    $(ids.pickerLabel).textContent = t("PICKER_PROMPT");
    $(ids.picker).innerHTML = "";
    const others = S.locations.filter((l) => l !== S.detected_location);
    for (const l of others) {
      const opt = document.createElement("option");
      opt.value = l;
      opt.textContent = l;
      $(ids.picker).appendChild(opt);
    }

    $(ids.conflictLabel).innerHTML = tHTML("CONFLICT_LABEL");
    $(ids.conflict).innerHTML = "";
    for (const v of [t("SKIP_BTN"), t("OVERWRITE_LABEL")]) {
      const opt = document.createElement("option");
      opt.value = v;
      opt.textContent = v;
      $(ids.conflict).appendChild(opt);
    }

    $(ids.existingLabel).innerHTML = tHTML("EXISTING_PRINTERS", S.existing.length);
    $(ids.deleteList).innerHTML = "";
    for (const p of S.existing) {
      const label = document.createElement("label");
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.dataset.name = p.name;
      cb.dataset.ip = p.ip;
      const span = document.createElement("span");
      span.textContent = `${p.name} (${p.ip || "?"})`;
      label.append(cb, span);
      $(ids.deleteList).appendChild(label);
    }

    if ($(ids.btnOk)) $(ids.btnOk).textContent = t("OK_LABEL");
    if ($(ids.btnCancel)) $(ids.btnCancel).textContent = t("CANCEL_LABEL");
    if ($(ids.btnClose)) $(ids.btnClose).textContent = t("OK_LABEL");

    chosenLoc = S.detected_location ?? $(ids.picker).options[0]?.value ?? "";
    updateChosenState();
  }

  function currentLoc() {
    if ($(ids.chkConfirm).checked && S.detected_location) {
      return S.detected_location;
    }
    return $(ids.picker).value || "";
  }

  function updatePickerVisibility() {
    $(ids.pickerWrap).hidden = $(ids.chkConfirm).checked;
  }

  function updateChosenState() {
    chosenLoc = currentLoc();
    const ips = locIPs(chosenLoc);
    $(ids.conflict).disabled = !(S.conflict[chosenLoc] ?? false);
    for (const label of $(ids.deleteList).querySelectorAll("label")) {
      const cb = label.querySelector("input");
      const disabled = ips.includes(cb.dataset.ip);
      cb.disabled = disabled;
      if (disabled) cb.checked = false;
    }
  }

  function updateSummary() {
    const loc = currentLoc();
    const names = S.loc_names[loc] ?? [];
    const ips = locIPs(loc);
    $(ids.summary).textContent = [
      loc || t("NO_LOCATION"),
      names.length ? names.join(", ") : "",
      ips.length ? "IP: " + ips.join(", ") : "",
    ]
      .filter(Boolean)
      .join("  |  ");
  }

  // ---- 收集选择并执行 ----
  function collectRequest() {
    const checked = [];
    for (const cb of $(ids.deleteList).querySelectorAll("input")) {
      if (cb.checked) checked.push(cb.dataset.name);
    }
    return {
      location: currentLoc(),
      overwrite: $(ids.conflict).value === t("OVERWRITE_LABEL"),
      delete: checked,
    };
  }

  async function doConfirm() {
    const req = collectRequest();
    if (!req.location) {
      await showResult([{ kind: "install-failed", text: t("FAIL_PREFIX") + " no location" }]);
      return;
    }
    if (api.onConfirm) {
      // 宿主可接管（如先切换到自己的进度页）
      await api.onConfirm(req);
      return;
    }
    try {
      const res = await api.runInstall(req);
      if (res.cancelled) return; // 授权取消 → 留在当前页
      await showResult(res.messages.length ? res.messages : ["ok"]);
    } catch (e) {
      await showResult([{ kind: "install-failed", text: t("FAIL_PREFIX") + " " + e }]);
    }
  }

  // ---- 结果展示 ----
  async function showResult(raw) {
    const el = $(ids.resultBody);
    if (!el) return; // 宿主若不用结果区则忽略
    el.innerHTML = "";
    const messages = raw.map((m) =>
      typeof m === "string" ? { kind: "install-failed", text: m } : m
    );
    const installMsgs = messages.filter(
      (m) => m.kind === "installed" || m.kind === "skipped" || m.kind === "install-failed"
    );
    const removeMsgs = messages.filter(
      (m) => m.kind === "removed" || m.kind === "remove-failed"
    );
    const blocks = [installMsgs, removeMsgs].filter((b) => b.length);
    blocks.forEach((block, bi) => {
      if (bi > 0) {
        const hr = document.createElement("hr");
        hr.className = "result-divider";
        el.appendChild(hr);
      }
      const group = document.createElement("div");
      group.className = "result-group";
      for (const msg of block) {
        const p = document.createElement("p");
        p.textContent = msg.text;
        if (msg.text.includes("❌")) p.className = "fail";
        group.appendChild(p);
      }
      el.appendChild(group);
    });
  }

  // ---- 语言菜单（可选）----
  function buildLangMenu() {
    const drop = $(api.langDrop);
    if (!drop) return;
    drop.innerHTML = "";
    for (const l of LANGS) {
      const b = document.createElement("button");
      b.type = "button";
      b.dataset.lang = l.code;
      b.textContent = l.name;
      b.addEventListener("click", () => switchLang(l.code));
      drop.appendChild(b);
    }
  }

  async function switchLang(code) {
    if (!S) return;
    lang = code;
    refreshLangMenu();
    try {
      S.strings = await api.getStrings(code);
    } catch (_) {
      /* keep current on failure */
    }
    renderConfirm();
  }

  function refreshLangMenu() {
    const drop = $(api.langDrop);
    if (!drop) return;
    for (const b of drop.querySelectorAll("button")) {
      b.classList.toggle("active", b.dataset.lang === lang);
    }
  }

  // ---- 初始化 ----
  async function init() {
    S = await api.getState();
    lang = S.lang || "en";
    if (api.langBtn && api.langDrop) {
      $(api.langBtn).addEventListener("click", (e) => {
        e.stopPropagation();
        $(api.langDrop).hidden = !$(api.langDrop).hidden;
      });
      document.addEventListener("click", (e) => {
        if (!e.target.closest("#lang-menu")) $(api.langDrop).hidden = true;
      });
      buildLangMenu();
      refreshLangMenu();
    }
    renderConfirm();

    if ($(ids.chkConfirm)) {
      $(ids.chkConfirm).addEventListener("change", () => {
        updatePickerVisibility();
        updateChosenState();
        updateSummary();
      });
    }
    if ($(ids.picker)) {
      $(ids.picker).addEventListener("change", () => {
        updateChosenState();
        updateSummary();
      });
    }
    if ($(ids.btnOk)) {
      $(ids.btnOk).addEventListener("click", () => doConfirm());
    }
    return { S, t, tHTML };
  }

  return { init, doConfirm, showResult, t, tHTML };
}
