package scanner

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

type PrinterInfo struct {
	IP       string
	Model    string
	Brand    string
	Name     string
	Location string
}

// ProbeSingleIP 探测单个 IP 地址的打印机信息
func ProbeSingleIP(ip string) (*PrinterInfo, error) {
	if ip == "" {
		return nil, fmt.Errorf("IP 地址不能为空")
	}
	p := snmpProbe(ip)
	if p == nil {
		return nil, fmt.Errorf("在 %s 未发现 SNMP 打印机", ip)
	}
	return p, nil
}

// ScanNetwork SNMP 扫描网段内所有打印机
func ScanNetwork(subnet string) []PrinterInfo {
	if subnet == "" {
		subnet = "192.168.1.0/24"
	}

	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		log.Printf("无效子网: %v", err)
		return nil
	}

	var printers []PrinterInfo
	for ip = ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		// 快速 ping 检测存活
		if !ping(ip.String()) {
			continue
		}
		if p := snmpProbe(ip.String()); p != nil {
			printers = append(printers, *p)
		}
	}
	return printers
}

const (
	oidSysName  = ".1.3.6.1.2.1.1.5.0"
	oidModel    = ".1.3.6.1.2.1.25.3.2.1.3.1"
	oidLocation = ".1.3.6.1.2.1.1.6.0"
	oidBrandOID = ".1.3.6.1.2.1.1.1.0" // sysDescr
)

func snmpProbe(ip string) *PrinterInfo {
	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: "tencent",
		Version:   gosnmp.Version2c,
		Timeout:   time.Second * 2,
	}
	if err := params.Connect(); err != nil {
		return nil
	}
	defer params.Conn.Close()

	oids := []string{oidSysName, oidModel, oidLocation, oidBrandOID}
	result, err := params.Get(oids)
	if err != nil {
		return nil
	}

	if len(result.Variables) < 4 {
		return nil
	}

	model := pduString(result.Variables[1])
	if model == "" {
		return nil // 没有型号信息，跳过
	}

	return &PrinterInfo{
		IP:       ip,
		Name:     pduString(result.Variables[0]),
		Model:    model,
		Location: pduString(result.Variables[2]),
		Brand:    detectBrand(pduString(result.Variables[3]), model),
	}
}

func detectBrand(sysDescr, model string) string {
	// 品牌识别规则（配置中心会覆盖此内置映射）
	brands := map[string]string{
		"fujifilm":  "fujifilm",
		"apeosport": "fujifilm",
		"revoria":   "fujifilm",
		"hp":        "hp",
		"laserjet":  "hp",
		"canon":     "canon",
		"ir":        "canon",
		"ricoh":     "ricoh",
	}
	for key, brand := range brands {
		if contains(model, key) || contains(sysDescr, key) {
			return brand
		}
	}
	return "unknown"
}

func pduString(v gosnmp.SnmpPDU) string {
	switch val := v.Value.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func ping(ip string) bool {
	conn, err := net.DialTimeout("tcp", ip+":161", time.Millisecond*300)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
