//go:build !windows

package installer

import "fmt"

func installDriver(p Params) error {
	return fmt.Errorf("当前平台不支持 pnputil 安装")
}

func addPort(p Params) error {
	return fmt.Errorf("当前平台不支持 prnport.vbs")
}

func addPrinter(p Params) error {
	return fmt.Errorf("当前平台不支持 printui.dll")
}

func setDefault(p Params) error {
	return fmt.Errorf("当前平台不支持 printui.dll")
}

func closeProgressWindow() {}
