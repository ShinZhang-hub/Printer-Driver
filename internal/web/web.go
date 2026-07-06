package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"printer-installer/internal/config"
)

type installHandler func(ip, name string) error

func StartAdminPanel(cfg *config.Config, fn installHandler) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(adminHTML))
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(cfg)
			return
		}
		if r.Method == "POST" {
			var req struct {
				Config     json.RawMessage `json:"config"`
				SyncRemote bool            `json:"sync_remote"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			var updated config.Config
			if err := json.Unmarshal(req.Config, &updated); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			*cfg = updated
			if req.SyncRemote {
				if err := cfg.Save(); err != nil {
					w.WriteHeader(500)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
			} else {
				if err := cfg.SaveLocal(); err != nil {
					w.WriteHeader(500)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "sync_remote": fmt.Sprintf("%v", req.SyncRemote)})
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
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("启动管理面板失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	fmt.Println("管理面板已启动:", url)
	fmt.Println("关闭此窗口即可退出")

	openBrowser(url)
	http.Serve(ln, mux)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", url).Start()
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
<input type="text" id="printerIP" placeholder="例如: 30.61.40.40">
<label>打印机名称（可选）</label>
<input type="text" id="printerName" placeholder="例如: Printer-Osaka，留空自动使用型号名">
<button onclick="startInstall()">开始安装</button>
<div id="result"></div>
</div>

<div class="card">
<h3>当前配置</h3>
		<pre id="configDisplay" style="white-space:pre-wrap;word-break:break-all">加载中...</pre>
		<label><input type="checkbox" id="syncRemote"> 同步到远程</label>
		<button onclick="saveConfig()" id="saveBtn">保存配置</button>
		<div id="saveResult"></div>
	</div>

<script>
let currentConfig = null
fetch('/api/config').then(r=>r.json()).then(cfg => {
  currentConfig = cfg
  document.getElementById('configDisplay').textContent = JSON.stringify(cfg, null, 2)
  document.getElementById('configDisplay').contentEditable = true
})

function saveConfig() {
  const display = document.getElementById('configDisplay')
  const btn = document.getElementById('saveBtn')
  const result = document.getElementById('saveResult')
  const syncRemote = document.getElementById('syncRemote').checked
  btn.disabled = true
  btn.textContent = '保存中...'
  result.className = ''
  result.textContent = ''
  try {
    const updated = JSON.parse(display.textContent)
    fetch('/api/config', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({config: updated, sync_remote: syncRemote})
    }).then(r => r.json()).then(d => {
      if (d.error) {
        result.style.color = 'red'
        result.textContent = '保存失败: ' + d.error
      } else {
        result.style.color = 'green'
        result.textContent = syncRemote ? '保存成功（本地 + 远端）' : '保存成功（仅本地）'
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
  const ip = document.getElementById('printerIP').value.trim()
  const name = document.getElementById('printerName').value.trim()
  const btn = document.querySelector('button')
  const result = document.getElementById('result')
  if (!ip) { alert('请输入打印机 IP 地址'); return }
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
    } else {
      result.className = 'success'
      result.textContent = d.message || '安装成功'
    }
  }).catch(e => {
    result.className = 'error'
    result.textContent = '请求失败: ' + e
  }).finally(() => {
    btn.disabled = false
    btn.textContent = '开始安装'
  })
}
</script>
</body>
</html>`
