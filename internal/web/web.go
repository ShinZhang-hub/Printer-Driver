package web

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"printer-installer/internal/config"
	"printer-installer/internal/log"
)

type installHandler func(ip, name string) error

func StartAdminPanel(cfg *config.Config, embedded []byte, fn installHandler) (string, <-chan struct{}) {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(adminHTML))
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(cfg)
			return
		}
		if r.Method == "POST" {
			var updated config.Config
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			*cfg = updated
			if err := cfg.Save(); err != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	})

	mux.HandleFunc("/api/config/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		if reloaded := config.LoadRemote(embedded); reloaded != nil {
			*cfg = *reloaded
			json.NewEncoder(w).Encode(cfg)
		} else {
			http.Error(w, `{"error":"重新加载失败"}`, 500)
		}
	})

	mux.HandleFunc("/api/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req struct {
			IP   string `json:"ip"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
			return
		}
		if req.IP == "" {
			http.Error(w, `{"error":"IP 地址不能为空"}`, 400)
			return
		}
		err := fn(req.IP, req.Name)
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "安装成功"})

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		stdlog.Fatalf("启动管理面板失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	fmt.Println("管理面板已启动:", url)
	fmt.Println("关闭此窗口即可退出")

	openBrowser(url)
	go func() {
		srv.Serve(ln)
		close(done)
	}()
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	return url, done
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		edgePath := ""
		for _, dir := range []string{
			os.Getenv("PROGRAMFILES(X86)"),
			os.Getenv("PROGRAMFILES"),
			os.Getenv("LOCALAPPDATA"),
		} {
			p := filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe")
			if _, err := os.Stat(p); err == nil {
				edgePath = p
				break
			}
		}
		if edgePath != "" {
			log.Info("Edge --app 模式: %s", edgePath)
			exec.Command(edgePath, "--app="+url).Start()
		} else {
			log.Info("Edge 未找到，使用 url.dll 回退")
			exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

const adminHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>打印机驱动 - 管理面板</title>
<style>
body{font-family:sans-serif;margin:40px;background:#f5f5f5}
.card{background:#fff;padding:24px;border-radius:8px;margin-bottom:16px;box-shadow:0 1px 3px rgba(0,0,0,.1)}
input{width:100%;padding:8px;margin:6px 0 16px;border:1px solid #ccc;border-radius:4px;box-sizing:border-box}
input:disabled{background:#eee;color:#999}
button{background:#007aff;color:#fff;border:none;padding:10px 20px;border-radius:4px;cursor:pointer;font-size:14px}
button:hover{background:#0056b3}
button:disabled{background:#999;cursor:not-allowed}
#result{margin-top:12px;padding:12px;border-radius:4px;display:none}
#result.success{background:#d4edda;color:#155724;display:block}
#result.error{background:#f8d7da;color:#721c24;display:block}
label{font-weight:600;font-size:14px}
h2{margin-top:0}
</style>
</head>
<body>
<h2>打印机驱动安装器</h2>

<div class="card">
<h3>手动安装</h3>
<label>打印机 IP 地址</label>
<input type="text" id="printerIP" placeholder="例如: 30.61.40.40" oninput="onIPChange()">
<label>打印机名称</label>
<input type="text" id="printerName" placeholder="留空自动使用配置名称">
<button onclick="startInstall()" id="installBtn">开始安装</button>
<div id="result"></div>
</div>

<div class="card">
<h3>当前配置</h3>
		<pre id="configDisplay" style="white-space:pre-wrap;word-break:break-all">加载中...</pre>
		<button onclick="saveConfig()" id="saveBtn">保存配置</button>
		<button onclick="reloadConfig()" id="reloadBtn" style="background:#555;margin-left:8px">刷新配置</button>
		<div id="saveResult"></div>
	</div>

<script>
let currentConfig = null
fetch('/api/config').then(r=>r.json()).then(cfg => {
  currentConfig = cfg
  document.getElementById('configDisplay').textContent = JSON.stringify(cfg, null, 2)
  document.getElementById('configDisplay').contentEditable = true
  onIPChange()
})

function getDefaultName() {
  if (!currentConfig) return ''
  if (currentConfig.locations && currentConfig.locations.length > 0) {
    return currentConfig.locations[0].printer_name || currentConfig.locations[0].printer_model || ''
  }
  return ''
}

function onIPChange() {
  const ip = document.getElementById('printerIP').value.trim()
  const nameInput = document.getElementById('printerName')
  if (!ip) {
    nameInput.disabled = true
    nameInput.value = getDefaultName()
  } else {
    nameInput.disabled = false
  }
}

function reloadConfig() {
  const btn = document.getElementById('reloadBtn')
  const display = document.getElementById('configDisplay')
  btn.disabled = true
  btn.textContent = '刷新中...'
  fetch('/api/config/reload', {method:'POST'}).then(r=>{
    if (!r.ok) throw new Error(r.statusText)
    return r.json()
  }).then(cfg => {
    currentConfig = cfg
    display.textContent = JSON.stringify(cfg, null, 2)
    document.getElementById('saveResult').textContent = '配置已刷新'
  }).catch(e => {
    document.getElementById('saveResult').textContent = '刷新失败: '+e.message
  }).finally(() => {
    btn.disabled = false
    btn.textContent = '刷新配置'
  })
}

function saveConfig() {
  const display = document.getElementById('configDisplay')
  const btn = document.getElementById('saveBtn')
  const result = document.getElementById('saveResult')
  btn.disabled = true
  btn.textContent = '保存中...'
  result.className = ''
  result.textContent = ''
  try {
    const updated = JSON.parse(display.textContent)
    fetch('/api/config', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(updated)
    }).then(r => r.json()).then(d => {
      if (d.error) {
        result.style.color = 'red'
        result.textContent = '保存失败: ' + d.error
      } else {
        result.style.color = 'green'
        result.textContent = '保存成功（本地 + 远端）'
        currentConfig = updated
      }
    }).catch(e => {
      result.style.color = 'red'
      result.textContent = '请求失败: ' + e
    }).finally(() => {
      btn.disabled = false
      btn.textContent = '保存配置'
    })
  } catch (e) {
    result.style.color = 'red'
    result.textContent = 'JSON 格式错误: ' + e.message
    btn.disabled = false
    btn.textContent = '保存配置'
  }
}

function startInstall() {
  let ip = document.getElementById('printerIP').value.trim()
  const name = document.getElementById('printerName').value.trim()
  const btn = document.getElementById('installBtn')
  const result = document.getElementById('result')
  if (!ip && currentConfig) {
    if (currentConfig.printer_ips && currentConfig.printer_ips.length > 0) {
      ip = currentConfig.printer_ips[0]
    } else if (currentConfig.locations && currentConfig.locations.length > 0) {
      ip = currentConfig.locations[0].printer_ip
    }
  }
  btn.disabled = true
  btn.textContent = '安装中...'
  result.className = ''
  result.style.display = 'none'
  fetch('/api/install', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ip, name})
  }).then(r => r.json()).then(d => {
    if (d.error) {
      result.className = 'error'
      result.textContent = '安装失败: ' + d.error
      btn.disabled = false
      btn.textContent = '开始安装'
    } else {
      result.className = 'success'
      result.textContent = '安装成功，即将关闭'
      btn.textContent = '已完成'
      document.body.innerHTML = '<div style="text-align:center;margin-top:100px"><h2>安装完成</h2><p>此页面即将关闭</p></div>'
      setTimeout(() => window.close(), 2000)
    }
  }).catch(e => {
    result.className = 'error'
    result.textContent = '请求失败: ' + e
    btn.disabled = false
    btn.textContent = '开始安装'
  })
}
</script>
</body>
</html>`
