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
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"

	"printer-installer/internal/config"
	"printer-installer/internal/installer"
	"printer-installer/internal/log"
)

type installHandler func(ip, name string) error

func StartAdminPanel(cfg *config.Config, embedded []byte, fn installHandler) (string, <-chan struct{}) {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	var authed atomic.Bool
	if cfg.AdminPasswordHash == "" {
		authed.Store(true)
	}

	authGate := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !authed.Load() {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !authed.Load() {
			w.Write([]byte(loginPageHTML))
			return
		}
		w.Write([]byte(adminHTML))
	})

	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, 400)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(cfg.AdminPasswordHash), []byte(req.Password)); err != nil {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid password"})
			return
		}
		authed.Store(true)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/config", authGate(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("/api/config/reload", authGate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		if reloaded := config.LoadRemote(embedded); reloaded != nil {
			*cfg = *reloaded
			json.NewEncoder(w).Encode(cfg)
		} else {
			http.Error(w, `{"error":"reload failed"}`, 500)
		}
	}))

	mux.HandleFunc("/api/install", authGate(func(w http.ResponseWriter, r *http.Request) {
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
			http.Error(w, `{"error":"IP cannot be empty"}`, 400)
			return
		}
		err := fn(req.IP, req.Name)
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": installer.ResultMessage})

		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		stdlog.Fatalf("failed to start admin panel: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	var connCount int64
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			log.Info("no active connections, shutting down")
			cancel()
		})
	}
	srv.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			atomic.AddInt64(&connCount, 1)
		case http.StateClosed, http.StateHijacked:
			if atomic.AddInt64(&connCount, -1) <= 0 {
				time.AfterFunc(3*time.Second, func() {
					if atomic.LoadInt64(&connCount) <= 0 {
						shutdown()
					}
				})
			}
		}
	}

	fmt.Println("Admin panel started:", url)
	fmt.Println("Close this window to exit")

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
			log.Info("Edge --app mode: %s", edgePath)
			exec.Command(edgePath, "--app="+url).Start()
		} else {
			log.Info("Edge not found, falling back to url.dll")
			exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

const loginPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Printer Driver - Admin Login</title>
<style>
body{font-family:sans-serif;margin:0;background:#f5f5f5;display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{background:#fff;padding:32px;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.1);width:360px;max-width:90vw}
h2{margin-top:0;text-align:center}
input{width:100%;padding:8px;margin:6px 0 16px;border:1px solid #ccc;border-radius:4px;box-sizing:border-box}
button{background:#007aff;color:#fff;border:none;padding:10px 20px;border-radius:4px;cursor:pointer;font-size:14px;width:100%}
button:hover{background:#0056b3}
button:disabled{background:#999;cursor:not-allowed}
#error{color:#dc3545;text-align:center;margin-top:12px;display:none}
</style>
</head>
<body>
<div class="card">
<h2>Admin Panel</h2>
<p style="text-align:center;color:#666;margin-bottom:20px">Enter password to continue</p>
<input type="password" id="password" placeholder="Password" onkeydown="if(event.key==='Enter')login()">
<button onclick="login()" id="loginBtn">Login</button>
<div id="error"></div>
</div>
<script>
function login() {
  const pw = document.getElementById('password').value
  const btn = document.getElementById('loginBtn')
  const err = document.getElementById('error')
  btn.disabled = true
  btn.textContent = 'Verifying...'
  err.style.display = 'none'
  fetch('/api/auth', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({password: pw})
  }).then(r => {
    if (r.ok) {
      window.location.href = '/'
    } else {
      return r.json().then(d => { throw new Error(d.error || 'Invalid password') })
    }
  }).catch(e => {
    err.textContent = e.message
    err.style.display = 'block'
    btn.disabled = false
    btn.textContent = 'Login'
  })
}
</script>
</body>
</html>`

const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Printer Driver - Admin Panel</title>
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
<h2>Printer Driver Installer</h2>

<div class="card">
<h3 id="manualTitle" onclick="onManualClick()">Manual Install</h3>
<label>Printer IP Address</label>
<input type="text" id="printerIP" placeholder="e.g. 30.61.40.40" oninput="onIPChange()">
<label>Printer Name</label>
<input type="text" id="printerName" placeholder="leave empty to use config name">
<button onclick="startInstall()" id="installBtn">Start Install</button>
<div id="result"></div>
</div>

<div class="card" id="configCard" style="display:none">
<h3>Current Config</h3>
		<pre id="configDisplay" style="white-space:pre-wrap;word-break:break-all">Loading...</pre>
		<button onclick="saveConfig()" id="saveBtn">Save Config</button>
		<button onclick="reloadConfig()" id="reloadBtn" style="background:#555;margin-left:8px">Refresh Config</button>
		<div id="saveResult"></div>
	</div>

<script>
let currentConfig = null
let manualClicks = 0

function onManualClick() {
  manualClicks++
  if (manualClicks >= 6) {
    document.getElementById('configCard').style.display = 'block'
    document.getElementById('manualTitle').onclick = null
    document.getElementById('manualTitle').style.cursor = 'default'
  }
}
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
  btn.textContent = 'Refreshing...'
  fetch('/api/config/reload', {method:'POST'}).then(r=>{
    if (!r.ok) throw new Error(r.statusText)
    return r.json()
  }).then(cfg => {
    currentConfig = cfg
    display.textContent = JSON.stringify(cfg, null, 2)
    document.getElementById('saveResult').textContent = 'Config refreshed'
  }).catch(e => {
    document.getElementById('saveResult').textContent = 'Refresh failed: '+e.message
  }).finally(() => {
    btn.disabled = false
    btn.textContent = 'Refresh Config'
  })
}

function saveConfig() {
  const display = document.getElementById('configDisplay')
  const btn = document.getElementById('saveBtn')
  const result = document.getElementById('saveResult')
  btn.disabled = true
  btn.textContent = 'Saving...'
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
        result.textContent = 'Save failed: ' + d.error
        return
      }
      result.style.color = 'green'
      result.textContent = 'Saved, refreshing remote config...'
      return fetch('/api/config/reload', {method:'POST'}).then(r => {
        if (!r.ok) throw new Error(r.statusText)
        return r.json()
      }).then(cfg => {
        currentConfig = cfg
        display.textContent = JSON.stringify(cfg, null, 2)
        result.textContent = 'Saved, config refreshed from remote'
      })
    }).catch(e => {
      result.style.color = 'red'
      result.textContent = 'Request failed: ' + e
    }).finally(() => {
      btn.disabled = false
      btn.textContent = 'Save Config'
    })
  } catch (e) {
    result.style.color = 'red'
    result.textContent = 'Invalid JSON: ' + e.message
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
  btn.textContent = 'Installing...'
  result.className = ''
  result.style.display = 'none'
  fetch('/api/install', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ip, name})
  }).then(r => r.json()).then(d => {
    if (d.error) {
      result.className = 'error'
      result.textContent = 'Install failed: ' + d.error
      btn.disabled = false
      btn.textContent = 'Start Install'
    } else {
      result.className = 'success'
      result.textContent = (d.message || 'Installation successful').replace(/\n/g, '\n') + '\n\nPage will close shortly'
      btn.textContent = 'Done'
      document.body.innerHTML = '<div style="text-align:center;margin-top:80px"><h2>Installation Complete</h2><pre style="max-width:600px;margin:0 auto;text-align:left;background:#f8f8f8;padding:16px;border-radius:4px;font-size:13px">' + (d.message || 'Installation successful') + '</pre><p style="margin-top:20px;color:#666">This page will close shortly</p></div>'
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
