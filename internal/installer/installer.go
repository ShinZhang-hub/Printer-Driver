package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func Download(url, checksum string) string {
	local := filepath.Join(os.TempDir(), filepath.Base(url))

	// 检查本地是否已有
	if _, err := os.Stat(local); err == nil {
		if verifyChecksum(local, checksum) {
			log.Println("使用本地缓存:", local)
			return local
		}
	}

	log.Println("下载驱动:", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(local)
	if err != nil {
		log.Fatalf("创建文件失败: %v", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		log.Fatalf("写入失败: %v", err)
	}
	log.Printf("下载完成: %d bytes", written)

	if checksum != "" && !verifyChecksum(local, checksum) {
		log.Fatal("校验和不匹配")
	}

	return local
}

func Run(args []string) {
	if len(args) == 0 {
		args = []string{"/S"}
	}

	cmd := exec.Command("msiexec", append([]string{"/i"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("执行安装:", cmd.String())
	if err := cmd.Run(); err != nil {
		log.Fatalf("安装失败: %v", err)
	}
	fmt.Println("安装完成")
}

func verifyChecksum(path, expected string) bool {
	f, _ := os.Open(path)
	if f == nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)) == expected
}
