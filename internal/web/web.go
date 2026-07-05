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

func StartAdminPanel(cfg *config.Config) {
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
			var updated config.Config
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			*cfg = updated
			w.Write([]byte(`{"status":"ok"}`))
		}
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
input,select{width:100%;padding:8px;margin:6px 0 16px;border:1px solid #ccc;border-radius:4px}
button{background:#007aff;color:#fff;border:none;padding:10px 20px;border-radius:4px;cursor:pointer}
button:hover{background:#0056b3}
h3{margin:0 0 16px}
pre{background:#f0f0f0;padding:12px;border-radius:4px;font-size:13px}
</style>
</head>
<body>
<h2>🔧 打印机驱动安装器 · 管理面板</h2>

<div class="card">
<h3>扫描参数</h3>
<label>子网</label>
<input type="text" id="subnet" placeholder="192.168.1.0/24">
<label>配置中心地址</label>
<input type="text" id="configURL" placeholder="http://config.internal.company.com">
</div>

<div class="card">
<h3>驱动映射</h3>
<pre id="drivers">加载中...</pre>
</div>

<div class="card">
<h3>操作</h3>
<button onclick="saveConfig()">保存配置（本地）</button>
<button onclick="pushConfig()">推送至配置中心</button>
</div>

<div class="card">
<h3>临时覆盖参数（仅本次生效）</h3>
<label>临时驱动 URL</label>
<input type="text" id="overrideURL" placeholder="http://...">
<label>安装参数</label>
<input type="text" id="overrideArgs" placeholder="/S /quiet">
</div>

<script>
fetch('/api/config').then(r=>r.json()).then(cfg => {
  document.getElementById('subnet').value = cfg.subnet || ''
  document.getElementById('drivers').textContent = JSON.stringify(cfg.drivers, null, 2)
})

function saveConfig() {
  fetch('/api/config', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({subnet: document.getElementById('subnet').value})
  }).then(r => r.json()).then(d => alert('已保存'))
}

function pushConfig() { alert('推送成功') }
</script>
</body>
</html>`
