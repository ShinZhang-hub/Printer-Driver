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
  conflictWrap: "conflict-wrap",
  conflictLabel: "conflict-label",
  conflict: "conflict",
  defaultWrap: "default-wrap",
  defaultLabel: "default-label",
  chkDefault: "chk-default",
  defaultPickerWrap: "default-picker-wrap",
  defaultPicker: "default-picker",
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
  const api = opts; // { getState, runInstall, getStrings, ids?, langBtn?, simple? }
  const ids = { ...DEFAULT_IDS, ...(opts.ids || {}) };
  const $ = (id) => document.getElementById(id);
  let _onConfirm = null; // 宿主可通过 ui.onConfirm 覆盖
  // simple 模式：只做「选位置 + 安装」，不显示冲突/覆盖/删除列表，
  // runInstall 固定传 overwrite:false, delete:[]（printer-core 无需改动）。
  const simple = !!opts.simple;

  let S = null;      // 初始状态（含 strings）
  let lang = "en";

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
  // 变更：去掉 checkbox，改为单一下拉占位符“检测到您在 X，点击可选其他办公室”
  function renderConfirm() {
    $(ids.summary).textContent = [
      S.detected_location ?? t("NO_LOCATION"),
      S.detected_name,
      S.detected_ip ? "IP: " + S.detected_ip : "",
    ]
      .filter(Boolean)
      .join("  |  ");

    // 隐藏旧的 checkbox 行（保留 DOM 兼容）
    if ($(ids.confirmRow)) $(ids.confirmRow).hidden = true;
    if ($(ids.pickerLabel)) $(ids.pickerLabel).textContent = "";
    if ($(ids.pickerWrap)) $(ids.pickerWrap).hidden = false;

    $(ids.picker).innerHTML = "";
    if (S.detected_location) {
      const detOpt = document.createElement("option");
      detOpt.value = S.detected_location;
      detOpt.textContent = t("CONFIRM_FMT", S.detected_location);
      $(ids.picker).appendChild(detOpt);
      // 第一个选项（检测到的位置）锁定不可删除不可切换为非检测位置
      // 用户只能在"检测位置"和其他位置之间切换
      for (const l of S.locations.filter((l) => l !== S.detected_location)) {
        const opt = document.createElement("option");
        opt.value = l;
        opt.textContent = l;
        $(ids.picker).appendChild(opt);
      }
      $(ids.picker).value = S.detected_location;
    } else {
      const ph = document.createElement("option");
      ph.value = "";
      ph.textContent = t("PICKER_PROMPT");
      ph.disabled = true;
      ph.selected = true;
      $(ids.picker).appendChild(ph);
      for (const l of S.locations) {
        const opt = document.createElement("option");
        opt.value = l;
        opt.textContent = l;
        $(ids.picker).appendChild(opt);
      }
    }

    // — 设为默认打印机（安装页新增，默认勾选；多台时显示选择框，默认第一台）—
    const defaultWrap = $(ids.defaultWrap);
    const chkDef = $(ids.chkDefault);
    const defLabel = $(ids.defaultLabel);
    const defPickerWrap = $(ids.defaultPickerWrap);
    const defPicker = $(ids.defaultPicker);
    if (defaultWrap && chkDef) {
      if (defLabel) defLabel.textContent = t("SET_DEFAULT_LABEL");
      chkDef.checked = true;
      defaultWrap.hidden = false;
      // 多台时显示选择框
      const loc = currentLoc() || S.detected_location || S.locations[0] || "";
      const names = S.loc_names[loc] ?? [];
      if (defPicker && defPickerWrap) {
        defPicker.innerHTML = "";
        if (names.length > 1) {
          defPickerWrap.hidden = !chkDef.checked;
          for (const n of names) {
            const opt = document.createElement("option");
            opt.value = n;
            opt.textContent = n;
            defPicker.appendChild(opt);
          }
          defPicker.value = names[0];
          // 可选：显示“选择默认打印机：”提示
          const hint = defPickerWrap.querySelector("span");
          if (hint) hint.textContent = t("DEFAULT_CHOICE_LABEL");
        } else {
          defPickerWrap.hidden = true;
        }
      }
    }

    if (!simple) {
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
    }

    if ($(ids.btnOk)) $(ids.btnOk).textContent = t("OK_LABEL");
    if ($(ids.btnCancel)) $(ids.btnCancel).textContent = t("CANCEL_LABEL");
    if ($(ids.btnClose)) $(ids.btnClose).textContent = t("OK_LABEL");

    updateChosenState();
    updateDefaultState();
  }

  function currentLoc() {
    return $(ids.picker).value || "";
  }

  function updateChosenState() {
    const loc = currentLoc();
    if (simple) return;
    const hasConflict = !!(S.conflict[loc] ?? false);
    const wrap = $(ids.conflictWrap);
    if (wrap) wrap.hidden = !hasConflict;
    updateDefaultState();
  }

  function updateDefaultState() {
    const chk = $(ids.chkDefault);
    const wrap = $(ids.defaultPickerWrap);
    const picker = $(ids.defaultPicker);
    if (!chk || !wrap || !picker) return;
    const loc = currentLoc();
    const names = S.loc_names[loc] ?? [];
    // 仅多台时显示选择框，且勾选“设为默认”时才显示
    if (names.length > 1) {
      wrap.hidden = !chk.checked;
      if (!wrap.hidden) {
        // 若 picker 为空（首次或语言切换），重新填充
        if (picker.options.length !== names.length) {
          picker.innerHTML = "";
          for (const n of names) {
            const opt = document.createElement("option");
            opt.value = n;
            opt.textContent = n;
            picker.appendChild(opt);
          }
          picker.value = names[0];
        }
      }
    } else {
      wrap.hidden = true;
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
    const chkDef = $(ids.chkDefault);
    const defPicker = $(ids.defaultPicker);
    const loc = currentLoc();
    const names = S.loc_names[loc] ?? [];
    const setDefault = chkDef ? chkDef.checked : true;
    const defaultPrinter = setDefault ? (names.length > 1 && defPicker ? defPicker.value : names[0] || "") : "";
    if (simple) {
      return { location: loc, overwrite: false, delete: [], setDefault, defaultPrinter };
    }
    const hasConflict = !!(S.conflict[loc] ?? false);
    return {
      location: loc,
      overwrite: hasConflict && $(ids.conflict).value === t("OVERWRITE_LABEL"),
      delete: [],
      setDefault,
      defaultPrinter,
    };
  }

  async function doConfirm() {
    const req = collectRequest();
    if (!req.location) {
      await showResult([{ kind: "install-failed", text: t("FAIL_PREFIX") + " no location" }]);
      return;
    }
    if (_onConfirm) {
      // 宿主可接管（如先切换到自己的进度页）
      await _onConfirm(req);
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

    if ($(ids.picker)) {
      $(ids.picker).addEventListener("change", () => {
        updateChosenState();
        updateDefaultState();
        updateSummary();
        // 位置切换时重绘默认选择框（多台→单台切换）
        const loc = currentLoc();
        const names = S.loc_names[loc] ?? [];
        const chk = $(ids.chkDefault);
        const wrap = $(ids.defaultPickerWrap);
        const picker = $(ids.defaultPicker);
        if (picker && wrap && names.length > 1 && chk && chk.checked) {
          picker.innerHTML = "";
          for (const n of names) {
            const opt = document.createElement("option");
            opt.value = n;
            opt.textContent = n;
            picker.appendChild(opt);
          }
          picker.value = names[0];
        }
      });
    }
    if ($(ids.chkDefault)) {
      $(ids.chkDefault).addEventListener("change", () => {
        updateDefaultState();
      });
    }
    if ($(ids.btnOk)) {
      $(ids.btnOk).addEventListener("click", () => doConfirm());
    }
    return { S, t, tHTML };
  }

  // 重新拉取状态并重渲染界面（供宿主在配置刷新/事件后调用）。
  async function reloadState() {
    S = await api.getState();
    lang = S.lang || lang;
    renderConfirm();
    return S;
  }

  const instance = { init, doConfirm, showResult, t, tHTML, reloadState };
  Object.defineProperty(instance, "onConfirm", {
    get: () => _onConfirm,
    set: (fn) => { _onConfirm = fn; },
  });
  return instance;
}
