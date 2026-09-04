import { getInitialState, getStrings, refreshConfig, confirm, quit, getInstalledPrinters, checkServerHealth } from "./api.js";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { PhysicalSize } from "@tauri-apps/api/dpi";

const $ = (id) => document.getElementById(id);
const $$ = (sel) => [...document.querySelectorAll(sel)];

const WIN_W = 440;
let S = null;
let lang = "zh";
let officeKey = "auto";
let detectedOfficeKey = "auto";
let defaultPrinterName = "";
let pendingDefault = null;
let selectedInstall = new Set();
let removeSelected = new Set();
let addedLocations = [];
let serverHealthy = false;
let installedPrintersCache = [];

const printerIcon = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M6 9V4h12v5M6 18H4v-8h16v8h-2M6 14h12v6H6z"/></svg>';

// —— i18n ——
function t(k) {
  const v = S?.strings?.[k];
  return v !== undefined ? v : (STRINGSFallback[k] ?? k);
}
const STRINGSFallback = {
  OFFICE:"办公室",AUTO_DETECT:"自动检测",MANUAL_SELECT:"手动选择",CHANGE:"更换",
  AUTO_DETECT_MENU:"自动检测（推荐）",LOCAL_IP:"本机 IP：",
  CAPTION_INSTALL:"可用打印机",CAPTION_INSTALL_HINT:"勾选安装；右侧可设为默认",
  CURRENT_DEFAULT:"当前默认打印机：",NONE:"未设置",
  INSTALLED_TAG:"已安装",AVAILABLE_TAG:"可安装",SET_DEFAULT:"设为默认",CURRENT_DEFAULT_TAG:"当前默认",
  SELECTION:"已选择",UNIT:"台",CANCEL:"取消",INSTALL_BTN:"安装",
  INSTALLED_PRINTERS:"已安装的打印机",SELECT_ALL:"全选",CANCEL_SELECT_ALL:"取消全选",
  REMOVE_NOTE:"移除当前默认设备后，系统将自动选择其他可用打印机。",REMOVE_BTN:"移除",
  REVIEW_TITLE:"确认操作",REVIEW_INSTALL:"安装：",REVIEW_ADD_INSTALL:"追加安装：",
  REVIEW_CONFLICT:"冲突处理：",REVIEW_DEFAULT_PRINTER:"默认打印机：",
  REVIEW_REMOVE:"移除：",REVIEW_NONE:"无",REVIEW_SKIPPED_ADDED:"跳过（重复）：",REVIEW_FILTERED_REMOVE:"过滤：",
  TOAST_INSTALL:"已安装 %d 台打印机",TOAST_REMOVE:"已移除 %d 台打印机",
  TOAST_CANCEL:"已取消本次操作",TOAST_SWITCH:"已切换到 %s",TOAST_AUTO:"已自动识别为 %s",
  SERVER_OK:"服务连接正常",SERVER_ERR:"服务连接失败",
  INSTALLING:"安装中，请稍候...",
};

function toast(text) {
  const el = $("toast");
  el.textContent = text;
  // 确保在最新画面已渲染后再显示，避免被窗口收缩截断
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      el.classList.add("show");
      clearTimeout(toast.t);
      toast.t = setTimeout(() => el.classList.remove("show"), 2500);
    });
  });
}
async function toastAfterRefresh(text) {
  // 等待 refresh 后的 DOM 完整绘制 + 窗口自适应完成
  await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
  await new Promise(r => setTimeout(r, 120));
  toast(text);
}

// —— 窗口适配 ——
async function fitWindow() {
  try {
    await document.fonts.ready;
    const win = getCurrentWindow();
    const app = $("app");
    const rect = app.getBoundingClientRect();
    const scale = await win.scaleFactor();
    const wantW = Math.round(WIN_W * scale);
    const wantH = Math.round(rect.height * scale);
    const [inner, outer] = await Promise.all([win.innerSize(), win.outerSize()]);
    const decoW = outer.width - inner.width;
    const decoH = outer.height - inner.height;
    const targetW = wantW + decoW;
    const targetH = wantH + decoH;
    if (Math.abs(outer.height - targetH) < 4 && Math.abs(outer.width - targetW) < 4) return;
    await win.setSize(new PhysicalSize(targetW, targetH));
  } catch (_) {}
}
function scheduleFit() {
  for (const ms of [0, 120, 350, 700]) setTimeout(fitWindow, ms);
}
new ResizeObserver(() => requestAnimationFrame(fitWindow)).observe($("app"));

// —— 健康检测 ——
async function checkHealth() {
  try {
    const res = await checkServerHealth();
    serverHealthy = res?.healthy ?? false;
  } catch (_) {
    serverHealthy = false;
  }
  updateHealthUI();
}
function updateHealthUI() {
  const el = $("health-dot");
  if (!el) return;
  el.style.cursor = "pointer";
  el.onclick = () => toast(t(serverHealthy ? "SERVER_OK" : "SERVER_ERR"));
  if (serverHealthy) {
    el.classList.remove("err");
    el.title = t("SERVER_OK");
  } else {
    el.classList.add("err");
    el.title = t("SERVER_ERR");
  }
}

// —— 数据获取 ——
function currentOffice() {
  if (officeKey === "auto" || !S) return null;
  return S.locations?.find(l => l === officeKey) || null;
}
function currentLocNames() {
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  return S?.loc_names?.[loc] ?? [];
}
function currentLocIPs() {
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  return S?.loc_ips?.[loc] ?? [];
}
function officeName(loc) {
  return loc || "--";
}

// —— 渲染办公室卡片 ——
function renderOffice() {
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  const auto = officeKey === "auto";
  $("office-name").textContent = officeName(loc);
  $("office-detail").textContent = t("LOCAL_IP") + (S?.local_ip || "--");
  const status = $("office-status");
  status.textContent = auto ? t("AUTO_DETECT") : t("MANUAL_SELECT");
  status.className = "office-status " + (auto ? "auto" : "manual");
  // office menu
  const menu = $("office-menu");
  menu.innerHTML = "";
  const autoBtn = document.createElement("button");
  autoBtn.dataset.office = "auto";
  autoBtn.textContent = t("AUTO_DETECT_MENU");
  menu.appendChild(autoBtn);
  for (const l of (S?.locations ?? [])) {
    const b = document.createElement("button");
    b.dataset.office = l;
    b.textContent = l;
    if (addedLocations.some(a => a.loc === l)) b.style.opacity = "0.4";
    menu.appendChild(b);
  }
}

// —— 安装列表 ——
function installedIds() {
  const byIp = new Map((S?.existing ?? []).map(p => [p.ip, p.name]));
  const ids = new Set();
  for (const loc of (S?.locations ?? [])) {
    const ips = S?.loc_ips?.[loc] ?? [];
    for (const ip of ips) {
      if (byIp.has(ip)) ids.add(loc + "::" + ip);
    }
  }
  return ids;
}
function renderInstallList() {
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  const names = S?.loc_names?.[loc] ?? [];
  const ips = S?.loc_ips?.[loc] ?? [];
  const byIp = new Map((S?.existing ?? []).map(p => [p.ip, p.name]));
  const defName = defaultPrinterName;
  let html = "";
  for (let i = 0; i < names.length; i++) {
    const name = names[i];
    const ip = ips[i] || "";
    const installed = byIp.has(ip);
    const id = loc + "::" + ip;
    // 设为默认独立于安装勾选：只要未安装就可选，全局单选
    html += `<div class="printer">
      <input class="install-choice" type="checkbox" data-id="${id}" data-ip="${ip}" data-name="${name}" ${selectedInstall.has(id)?'checked':''} ${installed?'disabled':''}>
      <span class="printer-icon">${printerIcon}</span>
      <span><span class="printer-name">${name}</span><span class="printer-detail">IP ${ip}</span></span>
      <span class="printer-action">
        <span class="tag ${installed?'installed':'available'}">${installed?t('INSTALLED_TAG'):t('AVAILABLE_TAG')}</span>
        ${installed?(name===defName?`<span class="default-current">${t('CURRENT_DEFAULT_TAG')}</span>`:'')
          :`<label class="default-choice"><input type="checkbox" data-default value="${name}" ${pendingDefault===name?'checked':''}> ${t('SET_DEFAULT')}</label>`}
      </span>
    </div>`;
  }
  // added locations
  for (const a of addedLocations) {
    for (let i = 0; i < a.names.length; i++) {
      const name = a.names[i];
      const ip = a.ips[i] || "";
      const id = a.loc + "::" + ip;
      html += `<div class="printer">
        <input class="install-choice" type="checkbox" data-id="${id}" data-ip="${ip}" data-name="${name}" ${selectedInstall.has(id)?'checked':''}>
        <span class="printer-icon">${printerIcon}</span>
        <span><span class="printer-name">${name}</span><span class="printer-detail">IP ${ip} · ${a.loc}</span></span>
        <span class="printer-action"><span class="tag available">${t('AVAILABLE_TAG')}</span></span>
      </div>`;
    }
  }
  $("install-list").innerHTML = html;
  // default printer: show whatever lpstat -d reports, regardless of config
  $("current-default").textContent = defName || t("NONE");
  updateInstallSummary();
  bindInstallEvents();
}
function updateInstallSummary() {
  const n = selectedInstall.size;
  const unit = t("UNIT");
  $("install-summary").innerHTML = `${t("SELECTION")} <b>${n}</b>${unit ? " " + unit : ""}`;
  $("install-button").disabled = n === 0;
  // 同移除界面一致的全选/取消全选（便于同位置3台以上场景）
  const selBtn = $("install-select-all");
  if (selBtn) {
    const available = $$(".install-choice:not(:disabled)");
    const total = available.length;
    selBtn.textContent = (total > 0 && n === total) ? t("CANCEL_SELECT_ALL") : t("SELECT_ALL");
  }
}
function bindInstallEvents() {
  // 安装勾选
  $$(".install-choice").forEach(cb => {
    cb.addEventListener("change", () => {
      if (cb.checked) selectedInstall.add(cb.dataset.id);
      else selectedInstall.delete(cb.dataset.id);
      updateInstallSummary();
    });
  });
  // 设为默认：独立于安装，全局单选（最多 1 个）
  $$('input[data-default]').forEach(cb => {
    cb.addEventListener("change", () => {
      if (cb.checked) {
        $$('input[data-default]').forEach(c => { if (c !== cb) c.checked = false; });
        pendingDefault = cb.value;
      } else {
        pendingDefault = null;
      }
    });
  });
}

// —— 移除列表 ——
function renderRemoveList() {
  const printers = installedPrintersCache;
  let html = "";
  for (const p of printers) {
    html += `<label class="printer">
      <input type="checkbox" data-id="${p.name}" data-ip="${p.ip}" ${removeSelected.has(p.name)?'checked':''}>
      <span class="printer-icon">${printerIcon}</span>
      <span><span class="printer-name">${p.name}</span><span class="printer-detail">IP ${p.ip}</span></span>
      ${p.is_default?`<span class="default-current">${t('CURRENT_DEFAULT_TAG')}</span>`:'<span></span>'}
    </label>`;
  }
  $("remove-list").innerHTML = html;
  $("installed-count").textContent = printers.length;
  updateRemoveSummary();
  bindRemoveEvents();
}
function updateRemoveSummary() {
  const n = removeSelected.size;
  const unit = t("UNIT");
  $("remove-summary").innerHTML = `${t("SELECTION")} <b>${n}</b>${unit ? " " + unit : ""}`;
  $("remove-button").disabled = n === 0;
  const total = installedPrintersCache.length;
  $("select-all").textContent = n === total && n ? t("CANCEL_SELECT_ALL") : t("SELECT_ALL");
}
function bindRemoveEvents() {
  $$("#remove-list input[type=checkbox]").forEach(cb => {
    cb.addEventListener("change", () => {
      if (cb.checked) removeSelected.add(cb.dataset.id);
      else removeSelected.delete(cb.dataset.id);
      updateRemoveSummary();
    });
  });
}

// —— 办公室切换 ——
function selectOffice(value) {
  const auto = value === "auto";
  officeKey = value;
  const loc = auto ? (S?.detected_location ?? S?.locations?.[0] ?? "") : value;
  $("office-name").textContent = officeName(loc);
  $("office-detail").textContent = t("LOCAL_IP") + (S?.local_ip || "--");
  const status = $("office-status");
  status.textContent = auto ? t("AUTO_DETECT") : t("MANUAL_SELECT");
  status.className = "office-status " + (auto ? "auto" : "manual");
  $("office-menu").hidden = true;
  renderInstallList();
  toast(auto ? t("TOAST_AUTO").replace("%s", officeName(loc)) : t("TOAST_SWITCH").replace("%s", officeName(loc)));
}

// —— 刷新数据 ——
async function refreshAll() {
  try {
    S = await getInitialState();
  } catch (_) {}
  try {
    installedPrintersCache = await getInstalledPrinters();
  } catch (_) {}
  // default printer: read from InitialState first, fallback to is_default flag
  defaultPrinterName = S?.default_printer || installedPrintersCache.find(p => p.is_default)?.name || "";
  renderOffice();
  renderInstallList();
  renderRemoveList();
  updateHealthUI();
}

// —— 全局绑定 ——
function bindGlobal() {
  // office menu
  $("change-office").addEventListener("click", () => {
    $("office-menu").hidden = !$("office-menu").hidden;
  });
  $("office-menu").addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-office]");
    if (btn) selectOffice(btn.dataset.office);
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest(".office")) $("office-menu").hidden = true;
  });

  // tab 切换
  $$(".tab").forEach(tab => {
    tab.addEventListener("click", () => {
      if (tab.disabled) return;
      const key = tab.dataset.tab;
      $$(".tab").forEach(t => t.classList.toggle("active", t === tab));
      $("install-panel").classList.toggle("active", key === "install");
      $("remove-panel").classList.toggle("active", key === "remove");
      if (key === "remove") renderRemoveList();
      scheduleFit();
    });
  });

  // 取消
  $("exit-button").addEventListener("click", () => {
    pendingDefault = null;
    selectedInstall.clear();
    $("office-menu").hidden = true;
    renderInstallList();
    toast(t("TOAST_CANCEL"));
  });

  // 安装：支持跨位置勾选（切换 office 后总数正确且全部安装），Mori 部分安装
  $("install-button").addEventListener("click", async () => {
    const ids = [...selectedInstall];
    if (!ids.length) return;
    const chosenDefault = pendingDefault || "";
    // 从 id 解析打印机名，不依赖 DOM（切换 office 后 DOM 已变）
    const selectedNames = ids.map(id => {
      const sep = id.indexOf("::");
      if (sep === -1) return "";
      const loc = id.slice(0, sep);
      const ip = id.slice(sep + 2);
      const ips = S.loc_ips[loc] || [];
      const names = S.loc_names[loc] || [];
      const idx = ips.indexOf(ip);
      return idx >= 0 ? names[idx] : "";
    }).filter(Boolean);
    // 跨位置：从勾选 id 中提取所有涉及的位置
    const locsFromIds = [...new Set(ids.map(id => id.split("::")[0]))];
    const primaryLoc = locsFromIds[0] || currentOffice() || S?.detected_location || S?.locations?.[0] || "";
    const added = [...new Set([...locsFromIds.slice(1), ...addedLocations.map(a => a.loc)])];
    const btn = $("install-button");
    btn.disabled = true;
    try {
      const merged = { location: primaryLoc, overwrite: false, delete: [], added, selected: selectedNames, lang, defaultPrinter: chosenDefault };
      const hasInstall = selectedNames.length > 0 || added.length > 0;
      const res = hasInstall ? await confirm(merged) : { messages: [], cancelled: false };
      if (res.cancelled) return;
      await refreshAll();
      // current-default 必须来自系统真实值（refreshAll 已通过 lpstat -d 更新），不在此乐观写入
      selectedInstall.clear();
      pendingDefault = null;
      renderInstallList();
      renderRemoveList();
      scheduleFit();
      await toastAfterRefresh(t("TOAST_INSTALL").replace("%d", selectedNames.length));
    } catch (e) {
      await refreshAll();
      renderInstallList();
      renderRemoveList();
      scheduleFit();
      await toastAfterRefresh("❌ " + e);
    } finally {
      btn.disabled = false;
      updateInstallSummary();
    }
  });

  // select all (install) — 与移除界面一致，3台以上时便于全选/取消
  const installSelBtn = $("install-select-all");
  if (installSelBtn) {
    installSelBtn.addEventListener("click", () => {
      const available = $$(".install-choice:not(:disabled)");
      const ids = available.map(cb => cb.dataset.id);
      const allSelected = ids.length > 0 && ids.every(id => selectedInstall.has(id));
      if (allSelected) {
        ids.forEach(id => selectedInstall.delete(id));
      } else {
        ids.forEach(id => selectedInstall.add(id));
      }
      renderInstallList();
    });
  }
  // select all (remove)
  $("select-all").addEventListener("click", () => {
    const ps = installedPrintersCache;
    if (removeSelected.size === ps.length) removeSelected.clear();
    else removeSelected = new Set(ps.map(p => p.name));
    renderRemoveList();
  });

  // 移除：仅移除，不退出，先刷新再 toast
  $("remove-button").addEventListener("click", async () => {
    const names = [...removeSelected];
    if (!names.length) return;
    const n = names.length;
    const btn = $("remove-button");
    btn.disabled = true;
    try {
      const res = await confirm({ location: "", overwrite: false, delete: names, added: [], selected: [], lang, defaultPrinter: "" });
      if (res.cancelled) return;
      await refreshAll();
      removeSelected.clear();
      renderRemoveList();
      $("current-default").textContent = defaultPrinterName || t("NONE");
      scheduleFit();
      await toastAfterRefresh(t("TOAST_REMOVE").replace("%d", n));
    } catch (e) {
      await refreshAll();
      renderRemoveList();
      scheduleFit();
      await toastAfterRefresh("❌ " + e);
    } finally {
      btn.disabled = false;
      updateRemoveSummary();
    }
  });

  // language
  const LANGS = [
    { code: "zh", name: "简体中文" }, { code: "zh-Hant", name: "繁體中文" },
    { code: "en", name: "English" }, { code: "ja", name: "日本語" },
    { code: "ko", name: "한국어" },
  ];
  function buildLangMenu() {
    const drop = $("lang-drop");
    drop.innerHTML = "";
    for (const l of LANGS) {
      const b = document.createElement("button");
      b.dataset.lang = l.code;
      b.textContent = l.name;
      if (l.code === lang) b.classList.add("active");
      b.addEventListener("click", async () => {
        lang = l.code;
        $("lang-drop").hidden = true;
        try { S.strings = await getStrings(lang); } catch (_) {}
        renderAll();
      });
      drop.appendChild(b);
    }
  }
  $("lang-btn").addEventListener("click", (e) => { e.stopPropagation(); $("lang-drop").hidden = !$("lang-drop").hidden; });
  document.addEventListener("click", (e) => { if (!e.target.closest("#lang-menu")) $("lang-drop").hidden = true; });
  buildLangMenu();
}

function renderAll() {
  $("title-text").textContent = S?.strings?.TITLE || "打印机管理";
  $$(".tab").forEach(tab => {
    const key = tab.dataset.tab;
    if (key === "install") tab.childNodes[0].textContent = t("TAB_INSTALL");
    else if (key === "remove") tab.childNodes[0].textContent = t("TAB_REMOVE");
    else if (key === "repair") tab.textContent = t("TAB_REPAIR");
  });
  $("office-label").textContent = t("OFFICE");
  $("change-office").textContent = t("CHANGE");
  $("caption-install-b").textContent = t("CAPTION_INSTALL");
  $("current-default-label").textContent = t("CURRENT_DEFAULT");
  $("caption-remove-b").textContent = t("INSTALLED_PRINTERS");
  $("remove-note").textContent = t("REMOVE_NOTE");
  $("exit-button").textContent = t("CANCEL");
  $("install-button").textContent = t("INSTALL_BTN");
  $("remove-button").textContent = t("REMOVE_BTN");
  renderOffice();
  renderInstallList();
  renderRemoveList();
  updateHealthUI();
}

// —— 初始化 ——
(async () => {
  try {
    S = await getInitialState();
  } catch (e) {
    $("title-text").textContent = "❌ " + e;
    try { await getCurrentWindow().show(); } catch (_) {}
    return;
  }
  lang = S.lang || "zh";
  defaultPrinterName = S.default_printer || "";
  try { installedPrintersCache = await getInstalledPrinters(); } catch (_) {}

  renderAll();
  buildLangMenu?.();
  bindGlobal();
  checkHealth();
  setInterval(checkHealth, 30000);

  try { await getCurrentWindow().show(); } catch (_) {}
  scheduleFit();

  // 配置刷新
  refreshConfig().catch(() => {});
  window.__TAURI__?.event?.listen("config-updated", async () => {
    try { S = await getInitialState(); } catch (_) { return; }
    lang = S.lang || lang;
    defaultPrinterName = S.default_printer || "";
    renderAll();
  });
})();

function buildLangMenu() {
  const LANGS = [
    { code: "zh", name: "简体中文" }, { code: "zh-Hant", name: "繁體中文" },
    { code: "en", name: "English" }, { code: "ja", name: "日本語" },
    { code: "ko", name: "한국어" },
  ];
  const drop = $("lang-drop");
  drop.innerHTML = "";
  for (const l of LANGS) {
    const b = document.createElement("button");
    b.dataset.lang = l.code;
    b.textContent = l.name;
    if (l.code === lang) b.classList.add("active");
    b.addEventListener("click", async () => {
      lang = l.code;
      $("lang-drop").hidden = true;
      try { S.strings = await getStrings(lang); } catch (_) {}
      renderAll();
    });
    drop.appendChild(b);
  }
}
