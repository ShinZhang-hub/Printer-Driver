package main

import (
	"fmt"
	"os"
	"printer-installer/internal/scanner"
)

func main() {
	subnet := "30.61.39.0/24"
	if len(os.Args) > 1 {
		subnet = os.Args[1]
	}

	fmt.Println("扫描子网:", subnet)
	printers := scanner.ScanNetwork(subnet)

	if len(printers) == 0 {
		fmt.Println("未发现 SNMP 打印机")
		return
	}

	for _, p := range printers {
		fmt.Printf("IP: %s\n", p.IP)
		fmt.Printf("  型号: %s\n", p.Model)
		fmt.Printf("  名称: %s\n", p.Name)
		fmt.Printf("  品牌: %s\n", p.Brand)
		fmt.Printf("  位置: %s\n", p.Location)
	}
}
