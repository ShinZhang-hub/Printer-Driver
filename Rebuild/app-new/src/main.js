import { getInitialState, getStrings, refreshConfig, confirm, quit, getInstalledPrinters, checkServerHealth, getUsername } from "./api.js";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { PhysicalSize } from "@tauri-apps/api/dpi";

const $ = (id) => document.getElementById(id);
const $$ = (sel) => [...document.querySelectorAll(sel)];

const WIN_W = 440;
let S = null;
let lang = "zh";
let officeKey = "auto";
let anywhereSelected = new Set();
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
function esc(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

// —— 异地打印 ——
function getTodayStr(){const d=new Date();return d.getFullYear()+'-'+String(d.getMonth()+1).padStart(2,'0')+'-'+String(d.getDate()).padStart(2,'0');}
function isBannerDismissed(){try{const v=localStorage.getItem('anywhere_banner_dismissed');if(!v) return false;const ts=parseInt(v,10);if(isNaN(ts)) return localStorage.getItem('anywhere_banner_dismissed')===getTodayStr();return Date.now()-ts < 24*60*60*1000;}catch(e){return false;}}
function isJumpDismissed(){try{const v=localStorage.getItem('anywhere_jump_dismissed');if(!v) return false;const ts=parseInt(v,10);if(isNaN(ts)) return localStorage.getItem('anywhere_jump_dismissed')===getTodayStr();return Date.now()-ts < 24*60*60*1000;}catch(e){return false;}}
function isRemoteScenario(){
  const localIp = S?.local_ip || "";
  const defName = defaultPrinterName;
  if(!localIp || !defName) return false;
  // find default printer IP
  let defIp = "";
  for(const [loc, ips] of Object.entries(S?.loc_ips || {})){
    const names = S?.loc_names?.[loc] || [];
    const idx = names.indexOf(defName);
    if(idx>=0) {defIp = ips[idx] || ""; break;}
  }
  if(!defIp) {
    // fallback: find in installed cache
    const found = installedPrintersCache.find(pr=>pr.name===defName);
    defIp = found?.ip || "";
  }
  if(!defIp) return false;
  return localIp.split('.').slice(0,3).join('.') !== defIp.split('.').slice(0,3).join('.');
}
function shouldShowBanner(){return isRemoteScenario() && !isBannerDismissed();}
function shouldShowHighlight(){return isRemoteScenario() && !isBannerDismissed();}
function shouldShowJump(){return isRemoteScenario() && !isJumpDismissed() && !isBannerDismissed();}
function updateAnywhereHighlight(){
  const tab=document.getElementById('tab-anywhere');
  const banner=document.getElementById('anywhere-banner');
  const showBanner=shouldShowBanner();
  const showHighlight=shouldShowHighlight();
  const showJump=shouldShowJump();
  if(tab){tab.classList.toggle('highlight',showHighlight);tab.classList.toggle('jump',showJump);}
  if(banner)banner.style.display=showBanner?'flex':'none';
}
function anywhereCurrentOffice(){
  // 复用安装页办公室逻辑：共享 officeKey
  return currentOffice();
}
function renderAnywhere(){
  if(!S) return;
  // 复用安装页办公室逻辑：办公室卡片（与安装页共享 officeKey）
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  const isAuto = officeKey==="auto";
  const cardNameEl=document.getElementById('anywhere-office-name');
  const cardDetailEl=document.getElementById('anywhere-office-detail');
  const cardStatusEl=document.getElementById('anywhere-office-status');
  const cardLabelEl=document.getElementById('anywhere-office-label');
  if(cardNameEl) cardNameEl.textContent=loc||"--";
  if(cardDetailEl) cardDetailEl.textContent=t('LOCAL_IP')+(S?.local_ip||"--");
  if(cardStatusEl){cardStatusEl.textContent=isAuto?t('AUTO_DETECT'):t('MANUAL_SELECT'); cardStatusEl.className='office-status '+(isAuto?'auto':'manual');}
  if(cardLabelEl) cardLabelEl.textContent=t('OFFICE');
  // 办公室菜单
  const menu=document.getElementById('anywhere-office-menu');
  if(menu){
    menu.innerHTML="";
    const autoBtn=document.createElement('button');
    autoBtn.dataset.office="auto";
    autoBtn.textContent=t('AUTO_DETECT_MENU');
    menu.appendChild(autoBtn);
    for(const l of (S.locations||[])){
      const b=document.createElement('button');
      b.dataset.office=l;
      b.textContent=l;
      menu.appendChild(b);
    }
  }
  // 打印机列表：单台强制勾选，多台手动最多2台
  const names = S.loc_names?.[loc] || [];
  const ips = S.loc_ips?.[loc] || [];
  const byIp = new Map((S.existing||[]).map(pr=>[pr.ip, pr.name]));
  // 单台强制勾选
  if(names.length===1 && !byIp.has(ips[0]||"")){
    anywhereSelected.clear();
    anywhereSelected.add(loc+"::"+(ips[0]||""));
  }
  const container=document.getElementById('anywhere-printer-list');
  if(container){
    let html="";
    for(let i=0;i<names.length;i++){
      const name=names[i];
      const ip=ips[i]||"";
      const id=loc+"::"+ip;
      const installed=byIp.has(ip);
      const isForced = names.length===1 && !installed;
      html+=`<label class="printer" style="cursor:${(installed||isForced)?'default':'pointer'}"><input type="checkbox" data-id="${id}" ${anywhereSelected.has(id)?'checked':''} ${(installed||isForced)?'disabled':''}><span class="printer-icon">${printerIcon}</span><span><span class="printer-name">${name}</span><span class="printer-detail">IP ${ip}</span></span><span class="printer-action"><span class="tag ${installed?'installed':'available'}">${installed?t('INSTALLED_TAG'):t('AVAILABLE_TAG')}</span>${isForced?'<span style="font-size:10px;color:var(--sub)">'+ (t('REQUIRED')||'必选') +'</span>':''}</span></label>`;
    }
    if(names.length===0) html=`<div style="padding:12px;color:var(--sub);font-size:12px;text-align:center">${t('NONE')}</div>`;
    container.innerHTML=html;
    // 绑定（多台手动，最多2台；单台locked）
    container.querySelectorAll('input[type="checkbox"]').forEach(cb=>{
      cb.addEventListener('change', ()=>{
        const id=cb.dataset.id;
        if(cb.checked){
          if(anywhereSelected.size>=2){ cb.checked=false; toast(t('MAX_TWO')||'最多选择2台'); return; }
          anywhereSelected.add(id);
        } else {
          const sole = names.length===1;
          if(sole){ cb.checked=true; return; }
          anywhereSelected.delete(id);
        }
        renderAnywhere();
      });
    });
  }
  updateAnywhereSelectAllText();
  // email preview
  const userEl=document.getElementById('anywhere-email-user');
  const domainEl=document.getElementById('anywhere-email-domain');
  const preview=document.getElementById('anywhere-email-preview');
  if(userEl && domainEl && preview){
    const u=(userEl.value||'zhxsdxin').trim()||'zhxsdxin';
    const d=domainEl.value||'global.xx.com';
    preview.textContent=u+'@'+d;
  }
}
// —— 异地申请次数：每日最多2次；每次提交成功→隐藏表单显示成功详情（当前会话），
//    重开软件后若当日未满2次→重新显示申请界面；满2次→重开也显示成功详情；次日重置 ——
function getSubmitMeta(){
  try{
    const v=localStorage.getItem('anywhere_submit_meta');
    return v?JSON.parse(v):null;
  }catch(e){ return null; }
}
function isQuotaExhaustedToday(){
  const m=getSubmitMeta();
  return !!(m && m.date===getTodayStr() && m.count>=2);
}
// 成功详情渲染
function renderSuccess(record, quota){
  const succ=document.getElementById('anywhere-success');
  if(!succ) return;
  succ.innerHTML=`<b style="font-size:14px">🎉 ${t('ANYWHERE_STATUS')}</b><br>`+
    (record.req_no?`<div style="margin-top:8px;color:var(--ink)">${t('ANYWHERE_REQ')||'申请单号'}：<b style="color:#1849a9">${esc(record.req_no)}</b></div>`:'')+
    `<div style="margin-top:8px;color:var(--ink)">${t('ANYWHERE_FORM_OFFICE_LABEL')}: <b>${esc(record.office||'')}</b></div>`+
    `<div style="color:var(--ink)">${t('ANYWHERE_DETAIL_PRINTERS')||'打印机'}: <b>${esc(record.printers||'')}</b></div>`+
    `<div style="color:var(--ink)">邮箱: <b>${esc(record.email||'')}</b></div>`+
    `<div style="color:var(--ink)">${t('ANYWHERE_FORM_REASON_LABEL')}: ${esc(record.reason||'')}</div>`+
    `<div style="font-size:11px;color:#667085;margin-top:8px">${(t('ANYWHERE_SUCCESS_QUOTA')||'今日申请次数：%d/2').replace('%d', quota||1)}</div>`+
    `<div style="font-size:11px;color:#667085">${t('ANYWHERE_SUCCESS_HINT')||'明天将重新开放申请'}</div>`;
  succ.style.display='block';
}
// 提交成功后：总是隐藏表单、显示成功详情（当前会话）
function forceAnywhereSuccess(record, meta){
  const apply=document.getElementById('anywhere-apply');
  const status=document.getElementById('anywhere-status');
  if(apply) apply.style.display='none';
  if(status) status.style.display='none';
  renderSuccess(record, meta?meta.count:1);
}
// 载入/刷新时按当日剩余次数决定显示表单或成功详情
function renderAnywhereState(){
  const apply=document.getElementById('anywhere-apply');
  const succ=document.getElementById('anywhere-success');
  const status=document.getElementById('anywhere-status');
  const m=getSubmitMeta();
  if(m && m.date===getTodayStr() && m.count>=2){
    // 今日已用满2次：显示成功详情，隐藏申请界面
    if(apply) apply.style.display='none';
    if(status) status.style.display='none';
    renderSuccess(m.last||{}, m.count);
  } else {
    // 未满2次（或次日）：显示申请界面
    if(apply) apply.style.display='';
    if(succ) succ.style.display='none';
  }
}
function bumpSubmitCount(record){
  const today=getTodayStr();
  const m=getSubmitMeta();
  // 每日最多2次
  const count=Math.min((m && m.date===today ? m.count : 0)+1, 2);
  const meta={date:today, count, last:record};
  try{ localStorage.setItem('anywhere_submit_meta', JSON.stringify(meta)); }catch(e){}
  return meta;
}
function clearSubmitMeta(){
  try{ localStorage.removeItem('anywhere_submit_meta'); }catch(e){}
}
function bindAnywhereIcons(){
  const pairs=[['anywhere-info-wrap','anywhere-info-pop'],['anywhere-tip-wrap','anywhere-tip-pop']];
  const hideAll=()=>{pairs.forEach(([,pid])=>{const el=document.getElementById(pid);if(el)el.classList.remove('show');});};
  pairs.forEach(([wid,pid])=>{
    const wrap=document.getElementById(wid),pop=document.getElementById(pid);
    if(!wrap||!pop) return;
    wrap.addEventListener('mouseenter',()=>{hideAll();pop.classList.add('show');});
    wrap.addEventListener('mouseleave',()=>{pop.classList.remove('show');});
    const btn=wrap.querySelector('button');
    if(btn) btn.addEventListener('click',(e)=>{e.stopPropagation();const was=pop.classList.contains('show');hideAll();if(!was)pop.classList.add('show');});
  });
  document.addEventListener('click',(e)=>{if(!e.target.closest('.anywhere-icons'))hideAll();});
}
function updateAnywhereSelectAllText(){
  const selAll=document.getElementById('anywhere-select-all');
  if(!selAll) return;
  const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
  const names = S.loc_names?.[loc] || [];
  const ips = S.loc_ips?.[loc] || [];
  const byIp = new Map((S.existing||[]).map(pr=>[pr.ip, pr.name]));
  const available = names.filter((_,i)=>!byIp.has(ips[i]||""));
  const selCount = Array.from(anywhereSelected).filter(id=>id.startsWith(loc+"::")).length;
  // 单台强制勾选时隐藏全选
  if(names.length===1){ selAll.style.display='none'; return; }
  selAll.style.display='';
  // 多台时已选=已选数
  selAll.textContent = (available.length>0 && selCount>=available.length) ? t('CANCEL_SELECT_ALL') : t('SELECT_ALL');
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
          :`<label class="default-choice"><input type="checkbox" data-default data-id="${id}" value="${name}" ${pendingDefault===name?'checked':''}> ${t('SET_DEFAULT')}</label>`}
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
  // 安装勾选；取消安装时顺带取消该打印机的设为默认（避免默认指向未安装项）
  $$(".install-choice").forEach(cb => {
    cb.addEventListener("change", () => {
      if (cb.checked) {
        selectedInstall.add(cb.dataset.id);
      } else {
        selectedInstall.delete(cb.dataset.id);
        const def = $$('input[data-default]').find(x => x.dataset.id === cb.dataset.id);
        if (def && def.checked) def.checked = false;
        if (def && pendingDefault === def.value) pendingDefault = null;
      }
      updateInstallSummary();
    });
  });
  // 设为默认：全局单选（最多 1 个），勾选时顺带勾选左侧安装框
  $$('input[data-default]').forEach(cb => {
    cb.addEventListener("change", () => {
      if (cb.checked) {
        $$('input[data-default]').forEach(c => { if (c !== cb) c.checked = false; });
        pendingDefault = cb.value;
        const id = cb.dataset.id;
        if (id && !selectedInstall.has(id)) {
          selectedInstall.add(id);
          const inst = $$(".install-choice").find(x => x.dataset.id === id);
          if (inst && !inst.disabled) inst.checked = true;
          updateInstallSummary();
        }
      } else {
        if (pendingDefault === cb.value) pendingDefault = null;
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
  renderAnywhere();
  renderAnywhereState();
  updateAnywhereHighlight();
  updateHealthUI();
}

// —— 全局绑定 ——
function bindGlobal() {
  // office menu (install)
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
  // office menu (anywhere) - 复用安装页逻辑
  const awChangeBtn=document.getElementById('anywhere-change-office');
  const awMenu=document.getElementById('anywhere-office-menu');
  if(awChangeBtn && awMenu){
    awChangeBtn.addEventListener('click', (e)=>{e.stopPropagation(); awMenu.hidden=!awMenu.hidden;});
    awMenu.addEventListener('click', (e)=>{
      const btn=e.target.closest('button[data-office]');
      if(!btn) return;
      const v=btn.dataset.office;
      anywhereSelected.clear();
      awMenu.hidden=true;
      // 复用安装页办公室切换：共享 officeKey，安装页与异地页联动
      selectOffice(v);
      renderAnywhere();
      updateAnywhereHighlight();
    });
    document.addEventListener('click', (e)=>{
      if(!e.target.closest('#anywhere-office-card')) awMenu.hidden=true;
    });
  }
  // anywhere 说明/提示图标：悬浮显示，离开隐藏（点按可切换，点空白关闭）
  bindAnywhereIcons();
  const awSelAll=document.getElementById('anywhere-select-all');
  if(awSelAll){
    awSelAll.addEventListener('click', ()=>{
      const loc = currentOffice() ?? S?.detected_location ?? S?.locations?.[0] ?? "";
      const names=S.loc_names?.[loc]||[];
      const ips=S.loc_ips?.[loc]||[];
      const byIp=new Map((S.existing||[]).map(pr=>[pr.ip,pr.name]));
      // 单台强制勾选时全选无效
      if(names.length===1) return;
      const ids=[];
      for(let i=0;i<names.length;i++){ if(!byIp.has(ips[i]||"")) ids.push(loc+"::"+(ips[i]||"")); }
      const allSelected=ids.length>0 && ids.every(id=>anywhereSelected.has(id));
      if(allSelected) ids.forEach(id=>anywhereSelected.delete(id));
      else {
        anywhereSelected.clear();
        ids.slice(0,2).forEach(id=>anywhereSelected.add(id)); // 最多2台
      }
      renderAnywhere();
    });
  }

  // tab 切换
  $$(".tab").forEach(tab => {
    tab.addEventListener("click", () => {
      if (tab.disabled) return;
      const key = tab.dataset.tab;
      if(key==='anywhere' && shouldShowJump()){
        try{localStorage.setItem('anywhere_jump_dismissed',String(Date.now()));}catch(e){}
        tab.classList.remove('jump');
      }
      $$(".tab").forEach(t => t.classList.toggle("active", t === tab));
      $("install-panel").classList.toggle("active", key === "install");
      $("remove-panel").classList.toggle("active", key === "remove");
      $("anywhere-panel").classList.toggle("active", key === "anywhere");
      $("repair-panel").classList.toggle("active", key === "repair");
      if (key === "remove") renderRemoveList();
      if (key === "anywhere") {renderAnywhere(); renderAnywhereState(); updateAnywhereHighlight();}
      if (key === "repair") {/* 修复面板文案已在 renderAll 中润色 */}
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
  // anywhere - email 通过账户名自动检测
  const awUser=document.getElementById('anywhere-email-user');
  const awDomain=document.getElementById('anywhere-email-domain');
  const awPreview=document.getElementById('anywhere-email-preview');
  // 优先使用系统账户名，其次 localStorage，最后默认 zhxsdxin
  (async ()=>{
    let account="zhxsdxin";
    try{ const u=await getUsername(); if(u && u.trim() && u!=="root") account=u.trim().split(/[.\\\/]/).pop()||u; }catch(e){}
    try{
      const savedU=localStorage.getItem('anywhere_email_user');
      const savedD=localStorage.getItem('anywhere_email_domain');
      if(savedU && awUser) awUser.value=savedU;
      else if(awUser) awUser.value=account;
      if(savedD && awDomain) awDomain.value=savedD;
      refreshEmailPreview();
    }catch(e){ if(awUser) awUser.value=account; refreshEmailPreview();}
  })();
  function refreshEmailPreview(){if(awUser && awDomain && awPreview){const u=(awUser.value||'zhxsdxin').trim()||'zhxsdxin';const d=awDomain.value||'global.xx.com';awPreview.textContent=u+'@'+d;}}
  if(awUser) awUser.addEventListener('input', ()=>{refreshEmailPreview();try{localStorage.setItem('anywhere_email_user',awUser.value.trim());}catch(e){} awUser.style.borderColor='var(--line)';});
  if(awDomain) awDomain.addEventListener('change', ()=>{refreshEmailPreview();try{localStorage.setItem('anywhere_email_domain',awDomain.value);}catch(e){}});
  refreshEmailPreview();
  // anywhere - banner dismiss & submit
  const awBannerDismiss=document.getElementById('anywhere-banner-dismiss');
  if(awBannerDismiss) awBannerDismiss.addEventListener('click', ()=>{
    try{const now=String(Date.now());localStorage.setItem('anywhere_banner_dismissed',now);localStorage.setItem('anywhere_jump_dismissed',now);}catch(e){}
    updateAnywhereHighlight();
    toast(t('TOAST_CANCEL'));
  });
  const awSubmit=document.getElementById('anywhere-submit');
  const awCancel=document.getElementById('anywhere-cancel');
  const awReason=document.getElementById('anywhere-reason');
  const awStatus=document.getElementById('anywhere-status');
  if(awReason) awReason.addEventListener('input', ()=>{awReason.style.borderColor='var(--line)';const er=document.getElementById('anywhere-reason-error');if(er)er.style.display='none';});
  if(awSubmit) awSubmit.addEventListener('click', async ()=>{
    const officeObj=currentOffice();
    const office=officeObj || S?.detected_location || S?.locations?.[0] || "";
    const reason=(awReason?.value||'').trim();
    const errEl=document.getElementById('anywhere-reason-error');
    const emailUser=(awUser?.value||'zhxsdxin').trim()||'zhxsdxin';
    const emailDomain=awDomain?.value||'global.xx.com';
    const email=emailUser+'@'+emailDomain;
    if(anywhereSelected.size===0){
      toast(t('SELECTION')+': '+t('NONE'));
      return;
    }
    if(!reason){
      if(awReason){awReason.style.borderColor='var(--red)';awReason.focus();}
      if(errEl){errEl.textContent=t('ANYWHERE_REASON_REQUIRED');errEl.style.display='block';}
      toast(t('ANYWHERE_REASON_REQUIRED'));
      return;
    }
    if(awReason)awReason.style.borderColor='var(--line)';
    if(errEl)errEl.style.display='none';
    if(awUser)awUser.style.borderColor='var(--line)';
    try{localStorage.setItem('anywhere_email_user',emailUser);localStorage.setItem('anywhere_email_domain',emailDomain);}catch(e){}
    const selectedNames=Array.from(anywhereSelected).map(id=>{
      const sep=id.indexOf('::');
      if(sep===-1) return id;
      const loc=id.slice(0,sep);
      const ip=id.slice(sep+2);
      const ips=S.loc_ips?.[loc]||[];
      const names=S.loc_names?.[loc]||[];
      const idx=ips.indexOf(ip);
      return idx>=0?names[idx]:id;
    }).join(', ');
    const selectedArr=Array.from(anywhereSelected).map(id=>{
      const sep=id.indexOf('::');
      if(sep===-1) return id;
      const loc=id.slice(0,sep);
      const ip=id.slice(sep+2);
      const ips=S.loc_ips?.[loc]||[];
      const names=S.loc_names?.[loc]||[];
      const idx=ips.indexOf(ip);
      return idx>=0?names[idx]:id;
    });
    // 直接发给后台 8000
    const BACKEND="http://127.0.0.1:8000";
    const payload={
      user_id: emailUser,
      ip: S?.local_ip||"",
      email: email,
      default_printer: defaultPrinterName||"",
      target_location: office,
      target_printer: selectedArr[0]||"",
      target_printers: selectedArr,
      reason: reason
    };
    try{
      const r=await fetch(`${BACKEND}/api/apply`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
      const j=await r.json();
      console.log('apply',j);
      const req_no=j.req_no||j.reqNo||'';
      // 提交成功：累计当日次数（上限2），隐藏表单显示成功详情；重开软件未满2次则重新显示申请界面
      const meta=bumpSubmitCount({ req_no, office, printers:selectedNames, email, reason });
      forceAnywhereSuccess(meta.last, meta);
      toast(office+' — '+selectedNames+' — '+t('ANYWHERE_STATUS')+' — '+email);
    }catch(e){
      console.error(e);
      if(awStatus){
        awStatus.innerHTML=`<b>🛠</b><br><span style="color:var(--red)">`+e+`</span>`;
        awStatus.style.display='block';
        awStatus.classList.add('show');
      }
      toast('❌ '+e);
    }
    if(awReason) awReason.value='';
  });
  if(awCancel) awCancel.addEventListener('click', ()=>{
    if(awReason){awReason.value='';awReason.style.borderColor='var(--line)';}
    const st=document.getElementById('anywhere-status');
    if(st){st.style.display='none';st.classList.remove('show');}
    const er=document.getElementById('anywhere-reason-error');
    if(er)er.style.display='none';
    renderAnywhereState();
    toast(t('TOAST_CANCEL'));
  });
  const repairBtn=document.getElementById('repair-btn');
  if(repairBtn) repairBtn.addEventListener('click', ()=>{toast(t('REPAIR_DEV')||'诊断功能开发中...');});
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
    else if (key === "anywhere") tab.textContent = t("TAB_ANYWHERE");
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
  // anywhere & repair i18n (文案润色)
  const awMap = {
    "anywhere-banner-title":"ANYWHERE_BANNER_TITLE",
    "anywhere-banner-desc":"ANYWHERE_BANNER_DESC",
    "anywhere-desc-title":"ANYWHERE_DESC_TITLE",
    "anywhere-desc":"ANYWHERE_DESC",
    "anywhere-tip-title":"ANYWHERE_TIP_TITLE",
    "anywhere-tip-body":"ANYWHERE_TIP_BODY",
    "anywhere-form-office-label":"ANYWHERE_FORM_OFFICE_LABEL",
    "anywhere-form-email-label":"ANYWHERE_FORM_EMAIL_LABEL",
    "anywhere-form-reason-label":"ANYWHERE_FORM_REASON_LABEL",
    "anywhere-submit":"ANYWHERE_SUBMIT",
    "anywhere-cancel":"ANYWHERE_CANCEL",
    "anywhere-footer":"ANYWHERE_FOOTER",
    "anywhere-banner-dismiss":"ANYWHERE_BANNER_DISMISS",
    "repair-title":"REPAIR_TITLE",
    "repair-hint":"REPAIR_HINT",
    "repair-item1":"REPAIR_ITEM1",
    "repair-item1-detail":"REPAIR_ITEM1_DETAIL",
    "repair-item2":"REPAIR_ITEM2",
    "repair-item2-detail":"REPAIR_ITEM2_DETAIL",
    "repair-item3":"REPAIR_ITEM3",
    "repair-item3-detail":"REPAIR_ITEM3_DETAIL",
    "repair-status":"REPAIR_STATUS",
    "repair-footer":"REPAIR_FOOTER",
    "repair-btn":"REPAIR_BTN"
  };
  for(const [id,key] of Object.entries(awMap)){
    const el=document.getElementById(id);
    if(el){
      if(id==="anywhere-form-reason-label") el.innerHTML=t(key)+' <span style="color:var(--red)">*</span>';
      else el.textContent=t(key);
    }
  }
  const reasonEl=document.getElementById('anywhere-reason');
  if(reasonEl) reasonEl.placeholder=t('ANYWHERE_FORM_REASON_PLACEHOLDER');
  const errEl=document.getElementById('anywhere-reason-error');
  if(errEl) errEl.textContent=t('ANYWHERE_REASON_REQUIRED');
  renderOffice();
  renderInstallList();
  renderRemoveList();
  renderAnywhere();
  renderAnywhereState();
  updateAnywhereHighlight();
  updateHealthUI();
  // repair tag
  document.querySelectorAll('#repair-panel .tag.available').forEach(el=>el.textContent=t('REPAIR_STATUS'));
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
