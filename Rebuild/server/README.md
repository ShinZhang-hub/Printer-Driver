# 试用版 Server（`Rebuild/server/`）

Node.js 无依赖，`node v24` 已验证。

## 运行

```powershell
cd Rebuild\server
node server.js
```

- `GET /api/v1/config` 公开（客户端拉配置/健康检查共用）
- `GET /api/v1/health` 附带 `{ok, version}`
- `POST /api/v1/config` 需 `Authorization: Bearer <admin.token>` + `server.conf.json` 里 `adminIps` 白名单

## 配置

- `server.conf.json`：`port/host/publicUrl/adminIps`，试用默认 `HTTP 0.0.0.0:443`
- `config.json`：对外配置，`version/updated_at/config_url` 由服务端在 POST 成功时重写
- `admin.token`：首次启动自动生成，不入库；生产环境用 `icacls` 锁到 `SYSTEM+Administrators` 可读
- `backups/`：每次 POST 成功备份旧版，保留 `keepBackups` 个
- `logs/audit.log`：`时间 IP 方法 路径 状态` 全记录

## 试用验证（已通过）

```powershell
$tok = (Get-Content admin.token -Raw).Trim()
Invoke-RestMethod http://127.0.0.1:443/api/v1/config
# 无token->401, 错token->401, 错body->422, 对token+好body->200且version+1
```

## 切 HTTPS（生产）

1. 导出已有证书 `CN=30.61.39.54` 为 `server.pfx` 放本目录
2. `server.conf.json`：`https.enabled=true`，`publicUrl=https://30.61.39.54:443`
3. 重启验证 `curl -k https://30.61.39.54:443/api/v1/config`

## 开机自启（试用通过后再做）

```powershell
schtasks /Create /TN "PrinterConfigServer" /TR "\"C:\Program Files\nodejs\node.exe\" \"C:\Users\v_shinzhang\Printer-Driver\Rebuild\server\server.js\"" /SC ONSTART /RU SYSTEM /RL HIGHEST /F
New-NetFirewallRule -DisplayName "PrinterConfig-443" -Direction Inbound -LocalPort 443 -Protocol TCP -Action Allow
```

生产建议把本目录拷到 `C:\PrinterServer\` 再建任务，仓库目录只留开发版。
