# Windows 构建（控制台版，供 PS1 脚本调用）
.PHONY: windows
windows:
	go build -o bin/printer-installer.exe -ldflags="-s -w" .

# Windows 静默版（无窗口，双击静默安装）
.PHONY: windows-silent
windows-silent:
	go build -o bin/printer-installer-silent.exe -ldflags="-s -w -H=windowsgui" .

# Windows 打包（含 PowerShell 脚本）
.PHONY: winapp
winapp: windows
	mkdir -p "bin/PrinterInstaller"
	cp bin/printer-installer.exe "bin/PrinterInstaller/"
	cp winapp/PrinterInstaller.ps1 "bin/PrinterInstaller/"
	cp config.json "bin/PrinterInstaller/" 2>/dev/null || true
	@echo "=== 构建完成: bin/PrinterInstaller/ ==="
	@echo "以管理员身份运行 PrinterInstaller.ps1（需 PowerShell 5.1+）"

# Windows 单文件版（exe内嵌ps1）
.PHONY: winapp-standalone
winapp-standalone: windows
	mkdir -p "bin/PrinterInstaller"
	# Build self-contained PS1 with embedded EXE
	cp winapp/PrinterInstaller.ps1 "bin/PrinterInstaller/PrinterInstaller-standalone.ps1"
	openssl base64 -in bin/printer-installer.exe | tr -d '\n' > /tmp/exe-b64.txt
	b64size=$$(wc -c < /tmp/exe-b64.txt | tr -d ' ') && sed -i '' "s|___EXE_BASE64___|$$b64size|" "bin/PrinterInstaller/PrinterInstaller-standalone.ps1"
	@echo "=== 单文件版构建完成: bin/PrinterInstaller/PrinterInstaller-standalone.ps1 ==="
	@echo "以管理员身份运行此ps1即可（无需exe)"

# macOS 构建 (arm64, Apple Silicon)
.PHONY: darwin-arm64
darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/printer-installer-darwin-arm64 .

# macOS 构建 (amd64, Intel)
.PHONY: darwin-amd64
darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o bin/printer-installer-darwin-amd64 .

# macOS 构建（当前架构）
.PHONY: darwin
darwin:
	go build -o bin/printer-installer-darwin .

# macOS .app 捆绑包（独立运行，驱动已内嵌）
.PHONY: app
app: darwin
	mkdir -p "bin/PrinterInstaller.app/Contents/MacOS"
	mkdir -p "bin/PrinterInstaller.app/Contents/Resources/drivers/fujifilm"
	cp macapp/icons/AppIcon.icns "bin/PrinterInstaller.app/Contents/Resources/AppIcon.icns"
	cp bin/printer-installer-darwin "bin/PrinterInstaller.app/Contents/MacOS/"
	cp macapp/PrinterInstaller.sh "bin/PrinterInstaller.app/Contents/MacOS/PrinterInstaller"
	cp macapp/Info.plist "bin/PrinterInstaller.app/Contents/"
	cp mac_printer_driver.dmg "bin/PrinterInstaller.app/Contents/Resources/drivers/fujifilm/"
	chmod +x "bin/PrinterInstaller.app/Contents/MacOS/PrinterInstaller"
	xattr -cr "bin/PrinterInstaller.app" 2>/dev/null || true
	@echo "=== 构建完成: bin/PrinterInstaller.app ==="
	@echo "双击 PrinterInstaller.app 即可运行（已内嵌驱动，无需额外文件）"

# DMG 打包
.PHONY: dmg
dmg: app
	rm -rf "bin/PrinterInstaller-dmg"
	mkdir -p "bin/PrinterInstaller-dmg"
	cp -R "bin/PrinterInstaller.app" "bin/PrinterInstaller-dmg/"
	ln -s /Applications "bin/PrinterInstaller-dmg/Applications"
	rm -f "bin/PrinterInstaller.dmg"
	hdiutil create -volname "PrinterInstaller" -srcfolder "bin/PrinterInstaller-dmg" -ov -format UDZO "bin/PrinterInstaller.dmg" 2>&1
	rm -rf "bin/PrinterInstaller-dmg"
	@echo "=== DMG 构建完成: bin/PrinterInstaller.dmg ==="

# PKG 打包（含去隔离脚本）
.PHONY: pkg
pkg: app
	rm -rf "/tmp/printer-installer-pkg-root"
	mkdir -p "/tmp/printer-installer-pkg-root"
	cp -R "bin/PrinterInstaller.app" "/tmp/printer-installer-pkg-root/"
	find "/tmp/printer-installer-pkg-root" -name "._*" -delete
	pkgbuild --root "/tmp/printer-installer-pkg-root" \
		--install-location "/Applications" \
		--scripts "macapp/scripts" \
		--identifier "com.fujifilm.printer-installer" \
		--version "1.0.0" \
		"bin/PrinterInstaller.pkg" 2>&1
	rm -rf "/tmp/printer-installer-pkg-root"
	@echo "=== PKG 构建完成: bin/PrinterInstaller.pkg ==="
	@echo "静默安装: sudo installer -pkg bin/PrinterInstaller.pkg -target /"

# 全部构建
.PHONY: all
all: windows windows-debug darwin-arm64 darwin-amd64 app

# 清理
.PHONY: clean
clean:
	rm -f bin/printer-installer*.exe bin/printer-installer-darwin* bin/PrinterInstaller.pkg
	rm -rf bin/PrinterInstaller.app bin/PrinterInstaller.dmg bin/PrinterInstaller-dmg
