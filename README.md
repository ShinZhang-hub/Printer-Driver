# PrinterInstaller

macOS / Windows 打印机驱动一键安装工具。

## macOS 使用

```bash
# 构建 .app
make app

# 分发
make dmg    # DMG 安装包
make pkg    # PKG 安装包（推荐内部分发）

# 使用
双击 PrinterInstaller.app   → 自动检测位置 → 弹窗确认 → 安装
Shift + 双击                 → 管理面板（配置编辑）
```

## Windows 使用

```bash
# 构建
make windows

# 双击 printer-installer.exe → Fyne 原生 GUI
```

## 配置服务器

```bash
# 构建（Windows 服务器）
make config-server-windows
→ bin/config-server.exe

# 运行
config-server.exe     # 监听 :9527
```

## 目录结构

```
macapp/                 macOS .app 打包（shell + JXA 弹窗）
winapp/                 Windows 打包资源
internal/
  i18n/                 多语言文案（EN/JA/KO/ZH）
  installer/            打印机安装/删除
  fyneui/               Fyne 跨平台 GUI
  config/               配置文件读取/保存
  embeds/drivers/       内嵌打印机驱动
config.json              默认配置（含位置、打印机、子网）
```

## 按钮一览

| 操作 | 说明 |
|------|------|
| `make app` | 构建 macOS .app（JXA 原生弹窗） |
| `make pkg` | 构建 macOS PKG 安装包 |
| `make dmg` | 构建 macOS DMG 安装包 |
| `make windows` | 构建 Windows .exe |
| `make winapp` | 构建 Windows 单文件 ps1 |
```

## 语言

跟随系统自动切换 EN / JA / KO / ZH。文案在 `internal/i18n/strings.go`。
