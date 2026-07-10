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

	fmt.Println("Scanning subnet:", subnet)
	printers := scanner.ScanNetwork(subnet)

	if len(printers) == 0 {
		fmt.Println("No SNMP printers found")
		return
	}

	for _, p := range printers {
		fmt.Printf("IP: %s\n", p.IP)
		fmt.Printf("  Model: %s\n", p.Model)
		fmt.Printf("  Name: %s\n", p.Name)
		fmt.Printf("  Brand: %s\n", p.Brand)
		fmt.Printf("  Location: %s\n", p.Location)
	}
}
