# Windows 构建
go build -o bin/printer-installer.exe -ldflags="-s -w -H=windowsgui" .

# 带管理员 manifest 打包
go build -o bin/printer-installer-debug.exe .
