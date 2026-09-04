import { createMockApi } from "./mock.js";

const $=(id)=>document.getElementById(id);
const LOCATIONS=[
  {name:"Osaka - JP Tower",ips:["30.61.40.40"],names:["Printer-Osaka"]},
  {name:"Tokyo - Business Tower",ips:["30.61.30.30"],names:["Printer-Tencent"]},
  {name:"Tokyo - Mori Tower",ips:["30.61.34.29","30.61.34.30"],names:["Printer-BG","Printer-Game"]}
];

let lang="zh",currentTab="install",added=[];
let installState={pickerVal:"Osaka - JP Tower",conflictVal:"跳过",defChecked:true,defPickerVal:""};
let removeChecked=new Set();
let currentTabStyle="segmented";
const stateRef={current:{lang:"zh",locations:LOCATIONS.map(l=>l.name),existing:[]}};

function t(k){
  const STRINGS={en:{TITLE:"Printer Driver Installer",CONFIRM_FMT:"Detected at %s, click to choose another office",PICKER_PROMPT:"Select the correct location:",CONFLICT_LABEL:"Default printer already exists — **overwrite** or **skip** ?",SKIP_BTN:"Skip",OVERWRITE_LABEL:"Overwrite",SET_DEFAULT_LABEL:"Set as default printer",DEFAULT_CHOICE_LABEL:"Default printer:",EXISTING_PRINTERS:"**%d** printers found, check to remove:",OK_LABEL:"OK",CANCEL_LABEL:"Cancel",INSTALLED_LABEL:"✅ %s installed successfully",REMOVED_MSG:"🗑️ %s removed successfully",REVIEW_TITLE:"Confirm",REVIEW_INSTALL:"Install:",REVIEW_ADD_INSTALL:"Additional install:",REVIEW_CONFLICT:"Conflict:",REVIEW_DEFAULT_PRINTER:"Default printer:",REVIEW_REMOVE:"Remove:",REVIEW_NONE:"None",REVIEW_SKIPPED_ADDED:"Skipped (duplicate):",REVIEW_FILTERED_REMOVE:"Filtered:",BTN_ADD_MORE:"＋ Add more",BTN_ADD:"Add",BTN_CANCEL:"Cancel",SELECT_ALL:"Select all",NO_MORE_TO_ADD:"No more to add",TAB_INSTALL:"Install",TAB_REMOVE:"Remove"},zh:{TITLE:"打印机驱动安装",CONFIRM_FMT:"检测到您在 %s，点击可选其他办公室",PICKER_PROMPT:"请选择正确的位置：",CONFLICT_LABEL:"所选办公室的默认打印机已存在，**覆盖** 或 **跳过** ？",SKIP_BTN:"跳过",OVERWRITE_LABEL:"覆盖安装",SET_DEFAULT_LABEL:"设为默认打印机",DEFAULT_CHOICE_LABEL:"选择默认打印机：",EXISTING_PRINTERS:"本机已存在 **%d** 台打印机，勾选可移除：",OK_LABEL:"好",CANCEL_LABEL:"取消",INSTALLED_LABEL:"✅ %s 已成功安装",REMOVED_MSG:"🗑️ %s 已成功移除",REVIEW_TITLE:"确认操作",REVIEW_INSTALL:"安装：",REVIEW_ADD_INSTALL:"追加安装：",REVIEW_CONFLICT:"冲突处理：",REVIEW_DEFAULT_PRINTER:"默认打印机：",REVIEW_REMOVE:"移除：",REVIEW_NONE:"无",REVIEW_SKIPPED_ADDED:"跳过（重复）：",REVIEW_FILTERED_REMOVE:"过滤：",BTN_ADD_MORE:"＋ 继续添加",BTN_ADD:"添加",BTN_CANCEL:"取消",SELECT_ALL:"全选",NO_MORE_TO_ADD:"无更多可添加",TAB_INSTALL:"安装",TAB_REMOVE:"移除"},ja:{TITLE:"プリンタードライバーインストーラー",CONFIRM_FMT:"%s を検出、クリックで他オフィスを選択",PICKER_PROMPT:"正しい場所を選択してください：",CONFLICT_LABEL:"既定プリンターが既存。**上書き** か **スキップ** ？",SKIP_BTN:"スキップ",OVERWRITE_LABEL:"上書きインストール",SET_DEFAULT_LABEL:"既定のプリンターに設定",DEFAULT_CHOICE_LABEL:"既定のプリンター：",EXISTING_PRINTERS:"既存プリンター **%d** 台、削除するにはチェック：",OK_LABEL:"OK",CANCEL_LABEL:"キャンセル",INSTALLED_LABEL:"✅ %s をインストールしました",REMOVED_MSG:"🗑️ %s を削除しました",REVIEW_TITLE:"確認",REVIEW_INSTALL:"インストール：",REVIEW_ADD_INSTALL:"追加インストール：",REVIEW_CONFLICT:"競合：",REVIEW_DEFAULT_PRINTER:"既定プリンター：",REVIEW_REMOVE:"削除：",REVIEW_NONE:"なし",REVIEW_SKIPPED_ADDED:"スキップ（重複）：",REVIEW_FILTERED_REMOVE:"フィルター済：",BTN_ADD_MORE:"＋ 追加",BTN_ADD:"追加",BTN_CANCEL:"キャンセル",SELECT_ALL:"すべて選択",NO_MORE_TO_ADD:"追加なし",TAB_INSTALL:"インストール",TAB_REMOVE:"削除"},ko:{TITLE:"프린터 드라이버 설치",CONFIRM_FMT:"%s 감지, 클릭하여 다른 오피스 선택",PICKER_PROMPT:"올바른 위치를 선택하세요：",CONFLICT_LABEL:"기본 프린터가 이미 있음. **덮어쓰기** / **건너뛰기** ？",SKIP_BTN:"건너뛰기",OVERWRITE_LABEL:"덮어쓰기",SET_DEFAULT_LABEL:"기본 프린터로 설정",DEFAULT_CHOICE_LABEL:"기본 프린터:",EXISTING_PRINTERS:"기존 프린터 **%d** 대, 제거하려면 선택：",OK_LABEL:"확인",CANCEL_LABEL:"취소",INSTALLED_LABEL:"✅ %s 설치 완료",REMOVED_MSG:"🗑️ %s 제거 완료",REVIEW_TITLE:"확인",REVIEW_INSTALL:"설치：",REVIEW_ADD_INSTALL:"추가 설치：",REVIEW_CONFLICT:"충돌：",REVIEW_DEFAULT_PRINTER:"기본 프린터:",REVIEW_REMOVE:"제거：",REVIEW_NONE:"없음",REVIEW_SKIPPED_ADDED:"건너뜀 (중복):",REVIEW_FILTERED_REMOVE:"필터됨：",BTN_ADD_MORE:"＋ 추가",BTN_ADD:"추가",BTN_CANCEL:"취소",SELECT_ALL:"전체 선택",NO_MORE_TO_ADD:"추가 없음",TAB_INSTALL:"설치",TAB_REMOVE:"제거"},"zh-Hant":{TITLE:"印表機驅動程式安裝程式",CONFIRM_FMT:"偵測到您位於 %s，點擊可選其他辦公室",PICKER_PROMPT:"請選擇正確的位置：",CONFLICT_LABEL:"預設印表機已存在，**覆蓋** 或 **跳過** ？",SKIP_BTN:"跳過",OVERWRITE_LABEL:"覆蓋安裝",SET_DEFAULT_LABEL:"設為預設印表機",DEFAULT_CHOICE_LABEL:"選擇預設印表機：",EXISTING_PRINTERS:"本機已存在 **%d** 台印表機，勾選可移除：",OK_LABEL:"好",CANCEL_LABEL:"取消",INSTALLED_LABEL:"✅ %s 已成功安裝",REMOVED_MSG:"🗑️ %s 已成功移除",REVIEW_TITLE:"確認操作",REVIEW_INSTALL:"安裝：",REVIEW_ADD_INSTALL:"追加安裝：",REVIEW_CONFLICT:"衝突處理：",REVIEW_DEFAULT_PRINTER:"預設印表機：",REVIEW_REMOVE:"移除：",REVIEW_NONE:"無",REVIEW_SKIPPED_ADDED:"跳過（重複）：",REVIEW_FILTERED_REMOVE:"過濾：",BTN_ADD_MORE:"＋ 繼續新增",BTN_ADD:"新增",BTN_CANCEL:"取消",SELECT_ALL:"全選",NO_MORE_TO_ADD:"無更多可新增",TAB_INSTALL:"安裝",TAB_REMOVE:"移除"}};
  return STRINGS[lang]?.[k]||STRINGS.en[k]||k;
}

function updateSummary(){
  const picker=$("picker");
  const v=picker?.value||"";
  const l=LOCATIONS.find(x=>x.name===v);
  const el=$("summary-line");
  if(el)el.textContent=l?[v,l.names.join(", "),"IP: "+l.ips.join(", ")].filter(Boolean).join(" | "):v||"";
  const defPicker=$("def-picker"),defWrap=$("def-picker-wrap"),chkDef2=$("chk-default");
  if(defPicker&&l){defPicker.innerHTML=l.names.map(n=>`<option>${n}</option>`).join("");if(!defPicker.value&&l.names.length)defPicker.value=l.names[0];if(defWrap&&chkDef2)defWrap.style.display=(l.names.length>1&&chkDef2.checked)?"block":"none";}
  const cw=document.getElementById("conflict-wrap");
  if(cw)cw.style.display=(v==="Osaka - JP Tower")?"":"none";
}

function refreshAddPicker(avail){
  const used=new Set([$("picker")?.value||"",...added.map(a=>a.loc)]);
  const opts=LOCATIONS.filter(l=>!used.has(l.name));
  const p=$("add-picker");if(!p)return;p.innerHTML="";
  if(!opts.length){p.innerHTML=`<option disabled selected>${t("NO_MORE_TO_ADD")}</option>`;}else opts.forEach(l=>{const o=document.createElement("option");o.value=l.name;o.textContent=l.name;p.appendChild(o);});
}

function renderAdded(){
  const el=$("added-list");if(!el)return;el.innerHTML="";
  added.forEach((a,i)=>{const d=document.createElement("div");d.className="added-item";d.innerHTML=`<div class="left">${a.loc} — ${a.names.join(", ")} (${a.ips.join(", ")})</div><button class="del" data-i="${i}">×</button>`;el.appendChild(d);});
  el.querySelectorAll(".del").forEach(b=>b.addEventListener("click",()=>{added.splice(+b.dataset.i,1);render();}));
}

function render(){
  const existing=stateRef.current.existing;
  const detectedLoc=LOCATIONS[0].name;
  const allUsed=new Set([detectedLoc,...added.map(a=>a.loc)]);
  const avail=LOCATIONS.filter(l=>!allUsed.has(l.name));
  const defNames=LOCATIONS.find(l=>l.name===detectedLoc)?.names||[];
  const app=$("app");
  app.innerHTML=`
    <div class="header" style="position:relative">
      <div class="title">${t("TITLE")}</div>
      <button class="lang-btn" id="lang-btn">🌐</button>
      <div class="lang-drop" id="lang-drop">
        ${[{c:"zh",n:"简体中文"},{c:"zh-Hant",n:"繁體中文"},{c:"en",n:"English"},{c:"ja",n:"日本語"},{c:"ko",n:"한국어"}].map(x=>`<button data-lang="${x.c}" class="${x.c===lang?'active':''}">${x.n}</button>`).join("")}
      </div>
    </div>
    <nav class="tabs"><div class="tabs-inner">
      <button class="tab ${currentTab==='install'?'active':''}" data-tab="install">${t("TAB_INSTALL")}</button>
      <button class="tab ${currentTab==='remove'?'active':''}" data-tab="remove">${t("TAB_REMOVE")}</button>
    </div></nav>
    <section id="pane-install" class="tab-pane ${currentTab==='install'?'active':''}">
      <p class="summary" id="summary-line"></p>
      <hr>
      <select id="picker" onchange="window._updateSummary()"><option value="${detectedLoc}">${t("CONFIRM_FMT").replace("%s",detectedLoc)}</option>${LOCATIONS.filter(l=>l.name!==detectedLoc).map(l=>{const dis=added.some(a=>a.loc===l.name);return `<option value="${l.name}" ${dis?'disabled style="color:#c7c7cc"':''}>${l.name}</option>`;}).join("")}</select>
      <div id="conflict-wrap"><hr><p class="muted">${t("CONFLICT_LABEL").replace(/\*\*(.+?)\*\*/g,'<b style="color:#1d1d1f;font-weight:700">$1</b>')}</p><select id="conflict"><option>${t("SKIP_BTN")}</option><option>${t("OVERWRITE_LABEL")}</option></select></div>
      <label class="row"><input type="checkbox" id="chk-default" checked><span>${t("SET_DEFAULT_LABEL")}</span></label>
      <div id="def-picker-wrap"><span class="muted" style="font-size:12px">${t("DEFAULT_CHOICE_LABEL")}</span><select id="def-picker" style="flex:1;width:100%;margin-left:8px">${defNames.map(n=>`<option>${n}</option>`).join("")}</select></div>
      <div id="added-list"></div>
      <button class="sec" id="btn-add-more" style="width:100%;border-style:dashed;margin-top:8px">${t("BTN_ADD_MORE")}</button>
      <div id="add-picker-wrap"><select id="add-picker"></select><button id="btn-add-ok" style="padding:4px 10px">${t("BTN_ADD")}</button><button id="btn-add-cancel" class="sec" style="padding:4px 10px">${t("BTN_CANCEL")}</button></div>
      <div class="btns"><button class="sec" id="btn-cancel">${t("CANCEL_LABEL")}</button><button id="btn-ok">${t("OK_LABEL")}</button></div>
    </section>
    <section id="pane-remove" class="tab-pane ${currentTab==='remove'?'active':''}">
      <p class="muted" style="margin-bottom:4px">${t("EXISTING_PRINTERS").replace(/\*\*(.+?)\*\*/g,'<b style="color:#1d1d1f;font-weight:700">$1</b>').replace("%d",existing.length)}</p>
      <label class="row" style="margin:6px 0 4px"><input type="checkbox" id="chk-all"><span>${t("SELECT_ALL")}</span></label>
      <div id="delete-list">${existing.map(p=>`<label><input type="checkbox" data-name="${p.name}" data-ip="${p.ip}"><span>${p.name} (${p.ip})</span></label>`).join("")}</div>
      <div class="btns"><button class="sec" id="btn-r-cancel">${t("CANCEL_LABEL")}</button><button id="btn-r-ok">${t("OK_LABEL")}</button></div>
    </section>
    <section id="pane-review" class="tab-pane"><p style="font-size:14px;font-weight:600;margin:0 0 12px">${t("REVIEW_TITLE")}</p><div class="review-body" id="review-body"></div><div class="btns"><button class="sec" id="btn-rb">${t("BTN_CANCEL")}</button><button id="btn-re">${t("OK_LABEL")}</button></div></section>
    <section id="pane-result" class="tab-pane"><div id="result-body" style="padding:6px 0"></div><div class="btns"><button id="btn-rc">${t("OK_LABEL")}</button></div></section>
  `;
  wireEvents(existing,detectedLoc,defNames,avail);
}

function wireEvents(existing,loc,defNames,avail){
  const picker=$("picker");
  // Tab 切换
  document.querySelector(".tabs").onclick=e=>{const b=e.target.closest(".tab");if(b){
    if(currentTab==="install"){const p=$("picker"),c=$("conflict"),d=$("chk-default"),dp=$("def-picker");if(p)installState.pickerVal=p.value;if(c)installState.conflictVal=c.value;if(d)installState.defChecked=d.checked;if(dp)installState.defPickerVal=dp.value;}
    else{removeChecked=new Set([...document.querySelectorAll("#delete-list input:checked")].map(cb=>cb.dataset.ip));}
    currentTab=b.dataset.tab;render();
  }};
  // 语言
  $("lang-btn").onclick=e=>{e.stopPropagation();$("lang-drop").classList.toggle("show");};
  document.onclick=e=>{if(!e.target.closest("#lang-menu"))$("lang-drop").classList.remove("show");};
  const ld=$("lang-drop");
  if(ld)ld.onclick=e=>{const b=e.target.closest("button");if(b){lang=b.dataset.lang;render();}};
  // 默认打印机
  const chkDef=$("chk-default"),defWrap=$("def-picker-wrap"),defPicker=$("def-picker");
  function updateDefWrap(){const v=picker?.value||"";const l=LOCATIONS.find(x=>x.name===v);const names=l?.names||[];if(defPicker){defPicker.innerHTML=names.map(n=>`<option>${n}</option>`).join("");if(!defPicker.value&&names.length)defPicker.value=names[0];}if(defWrap&&chkDef)defWrap.style.display=(names.length>1&&chkDef.checked)?"block":"none";}
  if(chkDef)chkDef.onchange=updateDefWrap;
  updateDefWrap();
  // 继续添加
  const btnMore=$("btn-add-more"),addWrap=$("add-picker-wrap"),addPicker=$("add-picker"),btnAddOk=$("btn-add-ok"),btnAddCancel=$("btn-add-cancel"),btnOk=$("btn-ok"),btnROk=$("btn-r-ok");
  const lockOk=v=>{if(btnOk)btnOk.disabled=v;if(btnROk)btnROk.disabled=v;};
  if(btnMore)btnMore.onclick=()=>{addWrap.classList.add("show");btnMore.style.display="none";refreshAddPicker(avail);lockOk(true);};
  if(btnAddCancel)btnAddCancel.onclick=()=>{addWrap.classList.remove("show");btnMore.style.display="";lockOk(false);};
  if(btnAddOk)btnAddOk.onclick=()=>{const v=addPicker?.value;if(v){const l=LOCATIONS.find(x=>x.name===v);if(l)added.push({loc:l.name,names:l.names,ips:l.ips});}addWrap.classList.remove("show");btnMore.style.display="";lockOk(false);render();};
  renderAdded();
  // picker disabled
  if(picker){
    const addedLocs=new Set(added.map(a=>a.loc));
    [...picker.options].forEach(o=>{if(addedLocs.has(o.value)){o.disabled=true;o.style.color="#c7c7cc";}});
    const curOpt=[...picker.options].find(o=>o.value===picker.value);
    if(curOpt?.disabled){const first=[...picker.options].find(o=>!o.disabled);if(first)picker.value=first.value;}
  }
  updateSummary();
  // 恢复安装页状态
  if(currentTab==="install"){
    const p=$("picker"),c=$("conflict"),d=$("chk-default"),dp=$("def-picker");
    if(p&&installState.pickerVal)p.value=installState.pickerVal;
    if(c&&installState.conflictVal)c.value=installState.conflictVal;
    if(d)d.checked=installState.defChecked;
    if(dp&&installState.defPickerVal)dp.value=installState.defPickerVal;
    updateSummary();
  }
  if(currentTab==="remove"){document.querySelectorAll("#delete-list input").forEach(cb=>{if(removeChecked.has(cb.dataset.ip))cb.checked=true;});}
  // 已添加项删除
  $("added-list").onclick=e=>{const del=e.target.closest(".del");if(del){added.splice(+del.dataset.i,1);render();}};
  // 全选
  const chkAll=$("chk-all");
  if(chkAll)chkAll.onchange=function(){document.querySelectorAll("#delete-list input").forEach(cb=>cb.checked=this.checked);};
  const dl=document.getElementById("delete-list");
  if(dl)dl.onchange=e=>{if(e.target.type==="checkbox"){const cbs=[...document.querySelectorAll("#delete-list input")];if(chkAll)chkAll.checked=cbs.length>0&&cbs.every(c=>c.checked);}};
  // 按钮
  function collectAllData(){
    let pv,cv,dc,dpv;
    if(currentTab==="install"){pv=$("picker")?.value||"";cv=$("conflict")?.value||"";dc=$("chk-default")?.checked;dpv=$("def-picker")?.value||"";}
    else{pv=installState.pickerVal;cv=installState.conflictVal;dc=installState.defChecked;dpv=installState.defPickerVal;}
    let cip; if(currentTab==="remove"){cip=new Set([...document.querySelectorAll("#delete-list input:checked")].map(cb=>cb.dataset.ip));}else{cip=removeChecked;}
    return {pv,cv,dc,dpv,cip};
  }
  function onOkClick(){
    const{pv,cv,dc,dpv,cip}=collectAllData();
    const l=LOCATIONS.find(x=>x.name===pv);
    showReview(existing,pv,l?.names||[],cv,dc,dpv,cip);
  }
  if(btnOk)btnOk.onclick=onOkClick;
  if(btnROk)btnROk.onclick=onOkClick;
  const btnCancel=$("btn-cancel");
  if(btnCancel)btnCancel.onclick=()=>{};
  const btnRCancel=$("btn-r-cancel");
  if(btnRCancel)btnRCancel.onclick=()=>{currentTab="install";render();};
  // 结果页好
  $("btn-rc").onclick=()=>{currentTab="install";render();};
}

function showReview(existing,loc,defNames,cv,dc,dpv,cip){
  const hasConflict=(loc==="Osaka - JP Tower");
  const overwrite=hasConflict&&cv===t("OVERWRITE_LABEL");
  const checked=[...document.querySelectorAll("#delete-list input")].filter(cb=>cip.has(cb.dataset.ip));
  const installIps=new Set([...(overwrite?LOCATIONS.find(l=>l.name===loc)?.ips:[]),...added.flatMap(a=>a.ips)]);
  const filtered=checked.filter(cb=>!installIps.has(cb.dataset.ip));
  const filteredNames=filtered.map(cb=>cb.dataset.name);
  const skippedRemove=checked.length-filtered.length;
  let defPrinter="";if(dc&&defNames.length){defPrinter=(defNames.length>1&&dpv)?dpv:defNames[0];}
  const skippedAdded=[];const filteredAdded=added.filter(a=>{if(a.ips.some(ip=>existing.some(e=>e.ip===ip))){skippedAdded.push(a.loc);return false;}return true;});
  const body=$("review-body");body.innerHTML="";
  const addL=(k,items)=>{if(!items.length)return;const p=document.createElement("p");p.innerHTML=`<b>${t(k)}</b> ${items.join(", ")}`;body.appendChild(p);};
  addL("REVIEW_INSTALL",[loc+" — "+defNames.join(", ")]);
  if(hasConflict)addL("REVIEW_CONFLICT",[overwrite?t("OVERWRITE_LABEL"):t("SKIP_BTN")]);
  addL("REVIEW_ADD_INSTALL",filteredAdded.map(a=>`${a.loc} — ${a.names.join(", ")}`));
  if(skippedAdded.length)addL("REVIEW_SKIPPED_ADDED",skippedAdded);
  if(defPrinter)addL("REVIEW_DEFAULT_PRINTER",[defPrinter]);
  addL("REVIEW_REMOVE",filteredNames.length?filteredNames:(checked.length?checked.map(cb=>cb.dataset.name):[t("REVIEW_NONE")]));
  if(skippedRemove)addL("REVIEW_FILTERED_REMOVE",[skippedRemove]);
  document.querySelectorAll(".tab-pane").forEach(p=>p.classList.remove("active"));
  $("pane-review").classList.add("active");
  document.querySelectorAll(".tabs .tab").forEach(b=>{b.disabled=true;b.style.opacity=".4";});
  $("btn-rb").onclick=()=>{document.querySelectorAll(".tab-pane").forEach(p=>p.classList.remove("active"));$(currentTab==="install"?"pane-install":"pane-remove").classList.add("active");document.querySelectorAll(".tabs .tab").forEach(b=>{b.disabled=false;b.style.opacity="";});};
  $("btn-re").onclick=()=>showResult(filteredNames,filteredAdded,skippedAdded,skippedRemove,defPrinter,overwrite,loc,existing);
}

function showResult(deleted,filteredAdded,skippedAdded,skippedRemove,defPrinter,overwrite,loc,existing){
  const msgs=[];
  const locNames=LOCATIONS.find(l=>l.name===loc)?.names||[];
  if(overwrite){msgs.push({ok:true,text:t("REMOVED_MSG").replace("%s",loc)+" ("+locNames.join(", ")+")"});}
  else{msgs.push({ok:true,text:t("INSTALLED_LABEL").replace("%s",loc)+" ("+locNames.join(", ")+")"});}
  if(filteredAdded.length)msgs.push({ok:true,text:"＋ "+filteredAdded.map(a=>a.loc).join(", ")});
  if(defPrinter)msgs.push({ok:true,text:t("REVIEW_DEFAULT_PRINTER")+" "+defPrinter});
  if(deleted.length)msgs.push({ok:true,text:t("REMOVED_MSG").replace("%s",deleted.join(", "))});
  if(skippedAdded.length)msgs.push({ok:true,text:t("REVIEW_SKIPPED_ADDED")+" "+skippedAdded.join(", ")});
  if(skippedRemove)msgs.push({ok:true,text:t("REVIEW_FILTERED_REMOVE")+" "+skippedRemove});
  const el=$("result-body");el.innerHTML="";
  msgs.forEach(m=>{const p=document.createElement("p");p.textContent=m.text;if(!m.ok)p.className="result-fail";el.appendChild(p);});
  document.querySelectorAll(".tab-pane").forEach(p=>p.classList.remove("active"));
  $("pane-result").classList.add("active");
}

// 全局绑定 updateSummary 给 inline onchange 用
window._updateSummary=updateSummary;

// 控制面板
function wireControls(){
  $("ctrl-lang").addEventListener("change",()=>{lang=$("ctrl-lang").value;render();});
  $("ctrl-detected").addEventListener("change",rebuildState);
  $("ctrl-existing").addEventListener("change",rebuildState);
  document.querySelectorAll('input[name="tabStyle"]').forEach(r=>r.addEventListener("change",()=>{currentTabStyle=r.value;render();}));
  $("btn-reset").addEventListener("click",()=>{$("ctrl-lang").value="zh";$("ctrl-detected").value="Osaka - JP Tower";$("ctrl-existing").value="with-conflict";lang="zh";added=[];installState={pickerVal:"Osaka - JP Tower",conflictVal:"跳过",defChecked:true,defPickerVal:""};removeChecked=new Set();rebuildState();});
}

function rebuildState(){
  const mode=$("ctrl-existing").value;
  const existing=mode==="empty"?[]:mode==="no-conflict"?[{name:"Printer-Old-A",ip:"30.61.40.99"},{name:"My HP Printer",ip:"192.168.1.10"}]:mode==="many"?[{name:"Printer-Old-A",ip:"30.61.40.99"},{name:"Printer-Old-B",ip:"30.61.40.98"},{name:"Printer-Osaka",ip:"30.61.40.40"},{name:"Printer-Tencent",ip:"30.61.30.30"},{name:"My HP Printer",ip:"192.168.1.10"}]:[{name:"Printer-Old-A",ip:"30.61.40.99"},{name:"Printer-Osaka",ip:"30.61.40.40"},{name:"My HP Printer",ip:"192.168.1.10"}];
  stateRef.current.existing=existing;
  added=[];
  installState={pickerVal:LOCATIONS[0].name,conflictVal:"跳过",defChecked:true,defPickerVal:""};
  removeChecked=new Set();
  render();
}

wireControls();
render();
