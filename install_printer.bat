@echo off
chcp 65001 >nul
title Fujifilm Printer Installer

set DRV_DIR=%~dp0drivers\fujifilm\ffopkplw250320w646fml\Software\ART_EX\amd64\Common\001
set PORT_NAME=IP_30.61.40.40
set PRINTER_IP=30.61.40.40
set PRINTER_NAME=Printer-Osaka
set DRIVER_MODEL=FF Apeos C2571

echo ========================================
echo  Fujifilm Apeos C2571 Printer Installer
echo ========================================
echo.

echo [1/4] Adding driver...
pnputil /add-driver "%DRV_DIR%\FFSB2PLWJ.INF" /install
if %errorlevel% neq 0 (
    echo [WARNING] pnputil may have warnings above, continuing...
)

echo [2/4] Creating TCP/IP port %PORT_NAME%...
cscript %windir%\System32\Printing_Admin_Scripts\ja-JP\prnport.vbs -a -r %PORT_NAME% -h %PRINTER_IP% -o raw -n 9100

echo [3/4] Installing printer %PRINTER_NAME%...
rundll32 printui.dll,PrintUIEntry /if /b "%PRINTER_NAME%" /r "%PORT_NAME%" /m "%DRIVER_MODEL%"

echo [4/4] Setting as default printer...
rundll32 printui.dll,PrintUIEntry /y /n "%PRINTER_NAME%"

echo.
echo ========================================
echo  Installation complete!
echo  Printer: %PRINTER_NAME%
echo  IP: %PRINTER_IP%
echo ========================================
pause
