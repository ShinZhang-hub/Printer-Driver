// Printer 配置中心 - 试用版 Server（Node.js，无第三方依赖）
// 运行: node server.js
// GET  /api/v1/config  公开，客户端拉配置（含健康检查）
// POST /api/v1/config  公开（无鉴权，仅内网使用，注意收紧防火墙）
// GET  /               可视化编辑页（查看/修改 locations 与打印机，保存即 POST）

"use strict";
const http = require("http");
const https = require("https");
const fs = require("fs");
const path = require("path");

const DIR = __dirname;
const CONF_PATH = path.join(DIR, "server.conf.json");

function loadConf() {
  const raw = fs.readFileSync(CONF_PATH, "utf8");
  return JSON.parse(raw);
}
const conf = loadConf();

const CONFIG_PATH = path.join(DIR, conf.configFile || "config.json");
const LOG_DIR = path.join(DIR, "logs");
const BACKUP_DIR = path.join(DIR, "backups");
fs.mkdirSync(LOG_DIR, { recursive: true });
fs.mkdirSync(BACKUP_DIR, { recursive: true });

// ---- 小工具 ----
function audit(ip, method, url, status, extra = "") {
  const line = `${new Date().toISOString()} ip=${ip} ${method} ${url} -> ${status}${extra ? " " + extra : ""}\n`;
  try { fs.appendFileSync(path.join(LOG_DIR, "audit.log"), line); } catch {}
}

function clientIp(req) {
  const fwd = req.headers["x-forwarded-for"];
  if (typeof fwd === "string" && fwd.length) return fwd.split(",")[0].trim();
  const s = req.socket.remoteAddress || "";
  if (s.startsWith("::ffff:")) return s.slice(7);
  if (s === "::1") return "127.0.0.1";
  return s;
}

function readBody(req, maxBytes = 1024 * 1024) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];
    req.on("data", (c) => {
      size += c.length;
      if (size > maxBytes) { reject(new Error("body too large")); req.destroy(); return; }
      chunks.push(c);
    });
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

function send(res, status, obj) {
  const body = typeof obj === "string" ? obj : JSON.stringify(obj);
  res.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "Access-Control-Allow-Origin": "*",
  });
  res.end(body);
}

function loadConfig() {
  return JSON.parse(fs.readFileSync(CONFIG_PATH, "utf8"));
}

// 与 Rust 端 Config 结构对齐的宽松校验：垃圾直接 422
function validateConfig(c) {
  if (typeof c !== "object" || c === null) return "body must be a JSON object";
  if (!Array.isArray(c.locations)) return "locations must be an array";
  for (let i = 0; i < c.locations.length; i++) {
    const l = c.locations[i];
    if (typeof l.name !== "string" || !l.name.trim()) return `locations[${i}].name required`;
    if (l.printers !== undefined && !Array.isArray(l.printers)) return `locations[${i}].printers must be array`;
    if (Array.isArray(l.printers)) {
      for (let j = 0; j < l.printers.length; j++) {
        const p = l.printers[j];
        if (typeof p.ip !== "string" || !p.ip.trim()) return `locations[${i}].printers[${j}].ip required`;
        if (typeof p.name !== "string" || !p.name.trim()) return `locations[${i}].printers[${j}].name required`;
      }
    }
  }
  return null;
}

function pruneBackups(keep = 20) {
  try {
    const files = fs.readdirSync(BACKUP_DIR).filter((f) => f.startsWith("config-")).sort();
    while (files.length > keep) {
      const f = files.shift();
      try { fs.unlinkSync(path.join(BACKUP_DIR, f)); } catch {}
    }
  } catch {}
}

// ---- 可视化编辑页（GET /）：查看/修改 locations 与打印机，保存即 POST ----
const ADMIN_PAGE = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>打印机配置管理</title>
<style>
body{font-family:-apple-system,"Segoe UI","Microsoft YaHei",sans-serif;background:#f3f5f8;color:#1d2939;margin:0;padding:20px;font-size:14px}
.wrap{max-width:860px;margin:0 auto;background:#fff;border-radius:10px;padding:20px 24px;box-shadow:0 2px 10px rgba(0,0,0,.06)}
h2{margin:0 0 4px}.sub{color:#667085;font-size:12px;margin-bottom:16px}
.loc{border:1px solid #e4e7ec;border-radius:8px;padding:12px;margin-bottom:12px;background:#fafbfd}
.loc-head{display:flex;gap:8px;align-items:center;margin-bottom:8px}
.loc-head input{flex:1;padding:6px 8px;border:1px solid #d5dce6;border-radius:6px;font-size:13px}
.row{display:flex;gap:8px;margin-bottom:6px;align-items:center}
.row input{padding:6px 8px;border:1px solid #d5dce6;border-radius:6px;font-size:13px}
.row input.n{flex:1}.row input.i{flex:1}.row input.m{flex:1}
button{cursor:pointer;border:1px solid #d5dce6;background:#fff;border-radius:6px;padding:6px 12px;font-size:13px}
button.pri{background:#1570ef;border-color:#1570ef;color:#fff;font-weight:600}
button.dan{color:#d92d20;border-color:#fecdca}
button.sm{padding:3px 8px;font-size:12px}
.meta{display:flex;gap:8px;align-items:center;margin:12px 0;color:#667085;font-size:12px}
#msg{margin-top:10px;font-size:13px;min-height:20px}
#msg.ok{color:#067647}#msg.err{color:#d92d20}
.small{font-size:12px;color:#667085}
</style></head><body><div class="wrap">
<h2>打印机配置管理</h2>
<div class="sub">修改后点保存即时生效，客户端 30 秒内自动同步。version / updated_at / config_url 由服务端维护。</div>
<div id="locs"></div>
<div><button id="addLoc">+ 添加办公室</button>
<button class="pri" id="save" style="margin-left:8px">保存</button></div>
<div class="meta"><span id="ver"></span></div>
<div id="msg"></div>
</div><script>
var S=null;
function esc(s){return String(s==null?"":s).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;")}
function msg(t,ok){var e=document.getElementById("msg");e.textContent=t;e.className=ok?"ok":"err"}
function render(){
  var box=document.getElementById("locs");box.innerHTML="";
  document.getElementById("ver").textContent="version "+S.version+" · 更新 "+esc(S.updated_at||"");
  S.locations.forEach(function(l,li){
    var d=document.createElement("div");d.className="loc";
    var h='<div class="loc-head"><input data-k="name" data-i="'+li+'" value="'+esc(l.name)+'" placeholder="办公室名">'
      +'<input data-k="subnets" data-i="'+li+'" value="'+esc((l.subnets||[]).join(", "))+'" placeholder="子网 逗号分隔" style="flex:2">'
      +'<button class="dan sm" data-del-loc="'+li+'">删除</button></div>';
    h+='<div class="row small"><span>端口</span><input data-k="port_number" data-i="'+li+'" value="'+esc(l.port_number||9100)+'" style="width:70px">'
      +'<span>协议</span><input data-k="protocol" data-i="'+li+'" value="'+esc(l.protocol||"raw")+'" style="width:70px"></div>';
    var ph="";
    var plist=l.printers&&l.printers.length?l.printers:[{ip:l.printer_ip||"",name:l.printer_name||"",model:l.printer_model||""}];
    plist.forEach(function(p,pi){
      ph+='<div class="row"><input class="n" data-p="name" data-i="'+li+'" data-j="'+pi+'" value="'+esc(p.name)+'" placeholder="打印机名">'
        +'<input class="i" data-p="ip" data-i="'+li+'" data-j="'+pi+'" value="'+esc(p.ip)+'" placeholder="IP">'
        +'<input class="m" data-p="model" data-i="'+li+'" data-j="'+pi+'" value="'+esc(p.model)+'" placeholder="型号">'
        +'<button class="dan sm" data-del-prn="'+li+':'+pi+'">删</button></div>';
    });
    d.innerHTML=h+ph+'<button class="sm" data-add-prn="'+li+'">+ 打印机</button>';
    box.appendChild(d);
  });
  bind();
}
function collect(){
  var locs=[];
  document.querySelectorAll("#locs .loc").forEach(function(d,li){
    var g=function(k){var e=d.querySelector('[data-k="'+k+'"]');return e?e.value.trim():""};
    var o={name:g("name"),subnets:g("subnets").split(",").map(function(s){return s.trim()}).filter(Boolean)};
    var pn=parseInt(g("port_number"),10);if(pn)o.port_number=pn;
    if(g("protocol"))o.protocol=g("protocol");
    var ps=[];
    d.querySelectorAll(".row").forEach(function(r){
      var q=function(p){var e=r.querySelector('[data-p="'+p+'"]');return e?e.value.trim():""};
      if(q("ip")||q("name"))ps.push({ip:q("ip"),name:q("name"),model:q("model")});
    });
    if(ps.length===1&&!o.subnets.length){o.printer_ip=ps[0].ip;o.printer_name=ps[0].name;o.printer_model=ps[0].model}
    else o.printers=ps;
    locs.push(o);
  });
  return {version:S.version,updated_at:S.updated_at,config_url:S.config_url,port_number:S.port_number||9100,protocol:S.protocol||"raw",locations:locs};
}
function bind(){
  document.querySelectorAll("[data-del-loc]").forEach(function(b){b.onclick=function(){S.locations.splice(+b.getAttribute("data-del-loc"),1);sync();render()}});
  document.querySelectorAll("[data-add-prn]").forEach(function(b){b.onclick=function(){var l=S.locations[+b.getAttribute("data-add-prn")];sync();if(!l.printers)l.printers=[];l.printers.push({ip:"",name:"",model:""});render()}});
  document.querySelectorAll("[data-del-prn]").forEach(function(b){b.onclick=function(){var a=b.getAttribute("data-del-prn").split(":");sync();S.locations[+a[0]].printers.splice(+a[1],1);render()}});
  document.querySelectorAll("#locs input").forEach(function(e){e.onchange=sync});
}
function sync(){
  document.querySelectorAll("#locs .loc").forEach(function(d,li){
    var L=S.locations[li];if(!L)return;
    var g=function(k){var e=d.querySelector('[data-k="'+k+'"]');return e?e.value.trim():""};
    L.name=g("name");L.subnets=g("subnets").split(",").map(function(s){return s.trim()}).filter(Boolean);
    var pn=parseInt(g("port_number"),10);if(pn)L.port_number=pn;L.protocol=g("protocol")||"raw";
    var rows=d.querySelectorAll(".row");var idx=0;
    var getList=function(){if(L.printers&&L.printers.length)return L.printers;return [L]};
    rows.forEach(function(r){
      var q=function(p){var e=r.querySelector('[data-p="'+p+'"]');return e?e.value.trim():""};
      var hasP=r.querySelector('[data-p]');
      if(!hasP)return;
      var lst=getList();var item=lst[idx]||{};
      if(item===L){item.printer_name=q("name");item.printer_ip=q("ip");item.printer_model=q("model")}
      else{item.name=q("name");item.ip=q("ip");item.model=q("model")}
      idx++;
    });
  });
}
document.getElementById("addLoc").onclick=function(){sync();S.locations.push({name:"",subnets:[],printers:[{ip:"",name:"",model:""}]});render()};
document.getElementById("save").onclick=function(){
  msg("保存中...",true);
  var body=collect();
  fetch("/api/v1/config",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)})
    .then(function(r){return r.json().then(function(j){return {st:r.status,j:j}})})
    .then(function(x){if(x.st===200){S=x.j;render();msg("已保存 version "+x.j.version,true)}else{msg("保存失败 "+x.st+": "+(x.j.error||""),false)}})
    .catch(function(e){msg("请求失败: "+e,false)});
};
fetch("/api/v1/config").then(function(r){return r.json()}).then(function(j){S=j;render()}).catch(function(e){msg("加载失败: "+e,false)});
</script></body></html>`;

async function handler(req, res) {
  const ip = clientIp(req);
  const url = req.url.split("?")[0];

  if (req.method === "OPTIONS") {
    res.writeHead(204, {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, Authorization",
    });
    res.end();
    return;
  }

  if (url === "/api/v1/config" && req.method === "GET") {
    try {
      const cfg = loadConfig();
      audit(ip, "GET", url, 200, `v${cfg.version}`);
      send(res, 200, cfg);
    } catch (e) {
      audit(ip, "GET", url, 500);
      send(res, 500, { error: "cannot load config: " + e.message });
    }
    return;
  }

  if (url === "/api/v1/health" && req.method === "GET") {
    try {
      const cfg = loadConfig();
      audit(ip, "GET", url, 200);
      send(res, 200, { ok: true, version: cfg.version });
    } catch (e) {
      send(res, 500, { ok: false, error: e.message });
    }
    return;
  }

  if (url === "/" && req.method === "GET") {
    audit(ip, "GET", url, 200, "admin-page");
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
    res.end(ADMIN_PAGE);
    return;
  }

  if (url === "/api/v1/config" && req.method === "POST") {
    // 无鉴权（仅内网使用）：直接解析 + 校验
    let body;
    try {
      body = await readBody(req);
    } catch (e) {
      audit(ip, "POST", url, 413);
      send(res, 413, { error: String(e.message || e) });
      return;
    }
    let incoming;
    try {
      incoming = JSON.parse(body);
    } catch {
      audit(ip, "POST", url, 422, "invalid json");
      send(res, 422, { error: "invalid JSON" });
      return;
    }
    const err = validateConfig(incoming);
    if (err) {
      audit(ip, "POST", url, 422, err);
      send(res, 422, { error: err });
      return;
    }
    // 4) 备份旧版 -> version+1 / updated_at / config_url 由服务端生成
    try {
      const old = loadConfig();
      const ts = new Date().toISOString().replace(/[:.]/g, "-");
      fs.writeFileSync(path.join(BACKUP_DIR, `config-${ts}-v${old.version}.json`), JSON.stringify(old, null, 2));
      pruneBackups(conf.keepBackups || 20);
      const next = { ...incoming };
      next.version = (Number(old.version) || 0) + 1;
      next.updated_at = new Date().toISOString();
      next.config_url = conf.publicUrl || old.config_url; // 防止改偏
      fs.writeFileSync(CONFIG_PATH, JSON.stringify(next, null, 2));
      audit(ip, "POST", url, 200, `v${old.version}->v${next.version}`);
      send(res, 200, next);
    } catch (e) {
      audit(ip, "POST", url, 500);
      send(res, 500, { error: "save failed: " + e.message });
    }
    return;
  }

  audit(ip, req.method, url, 404);
  send(res, 404, { error: "not found" });
}

function start() {
  const port = conf.port || 443;
  const host = conf.host || "0.0.0.0";
  const useHttps = conf.https && conf.https.enabled;
  let server;
  if (useHttps) {
    const pfxPath = path.join(DIR, conf.https.pfxFile || "server.pfx");
    if (!fs.existsSync(pfxPath)) {
      console.error(`[server] https.enabled=true 但找不到 ${pfxPath}，先按 HTTP 启动或导出 pfx 后再开 HTTPS。`);
      process.exit(1);
    }
    server = https.createServer(
      { pfx: fs.readFileSync(pfxPath), passphrase: conf.https.passphrase || "" },
      handler
    );
  } else {
    server = http.createServer(handler);
  }
  server.on("error", (e) => {
    if (e.code === "EACCES") {
      console.error(`[server] 端口 ${port} 拒绝绑定：请用管理员/SYSTEM 运行，或改 server.conf.json port。`);
    } else if (e.code === "EADDRINUSE") {
      console.error(`[server] 端口 ${port} 已被占用：Get-NetTCPConnection -LocalPort ${port} 查进程。`);
    } else {
      console.error("[server] 启动失败:", e.message);
    }
    process.exit(1);
  });
  server.listen(port, host, () => {
    console.log(`[server] ${useHttps ? "https" : "http"}://${conf.publicHost || "30.61.39.54"}:${port} 已启动`);
    console.log(`[server] GET  /api/v1/config  公开`);
    console.log(`[server] POST /api/v1/config  公开（无鉴权，仅内网）`);
    console.log(`[server] GET  /               可视化编辑页`);
  });
}

start();
