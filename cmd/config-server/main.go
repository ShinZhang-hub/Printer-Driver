package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"printer-installer/internal/config"
)

var (
	store     *config.Config
	storeLock sync.RWMutex
)

func main() {
	store = loadStore()
	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/config", handleConfig)
	mux.HandleFunc("/api/v1/driver", handleDriver)

	log.Printf("配置中心启动 at http://0.0.0.0%s", port)
	if err := http.ListenAndServe(port, logRequest(mux)); err != nil {
		log.Fatal(err)
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// GET /api/v1/config → 返回完整配置
// PUT /api/v1/config → 更新配置（需 token）
func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		storeLock.RLock()
		defer storeLock.RUnlock()
		writeJSON(w, store)

	case http.MethodPut:
		if err := requireToken(r); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var updated config.Config
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		storeLock.Lock()
		updated.Touch()
		store = &updated
		saveStore(store)
		storeLock.Unlock()
		writeJSON(w, map[string]interface{}{"status": "ok", "version": updated.Version})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/v1/driver?model=C3070&brand=fujifilm → 返回匹配的驱动信息
func handleDriver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	model := r.URL.Query().Get("model")
	brand := r.URL.Query().Get("brand")

	storeLock.RLock()
	defer storeLock.RUnlock()

	driver := store.LookupDriver(model, brand)
	if driver == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeJSON(w, driver)
}

// --- 存储 ---

const storeFile = "config-server.json"

func loadStore() *config.Config {
	f, err := os.Open(storeFile)
	if err != nil {
		log.Printf("无持久化文件，使用默认配置: %v", err)
		cfg := defaultServerConfig()
		saveStore(cfg)
		return cfg
	}
	defer f.Close()
	cfg := defaultServerConfig()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		log.Printf("配置解析失败，使用默认: %v", err)
		saveStore(cfg)
	}
	return cfg
}

func saveStore(cfg *config.Config) {
	f, err := os.Create(storeFile)
	if err != nil {
		log.Printf("保存配置失败: %v", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(cfg)
}

func defaultServerConfig() *config.Config {
	hostname, _ := os.Hostname()
	return &config.Config{
		Version:   1,
		UpdatedAt: "2024-01-01T00:00:00Z",
		Subnet:    "192.168.1.0/24",
		PrinterIPs: []string{},
		Drivers: []config.DriverConfig{
			{
				Brand:     "fujifilm",
				Model:     "ApeosPort C3070",
				ID:        "fujifilm-apeosport-c3070",
				PkgURLWin: "http://" + hostname + ":8080/drivers/fujifilm-apeosport-c3070.exe",
				PkgURLMac: "http://" + hostname + ":8080/drivers/fujifilm-apeosport-c3070.pkg",
				InstallArgs: []string{"/S"},
				Version:   "1.0.0",
				Enabled:   true,
			},
		},
	}
}

// --- auth ---

var adminToken string

func init() {
	adminToken = os.Getenv("ADMIN_TOKEN")
}

func requireToken(r *http.Request) error {
	if adminToken == "" {
		return nil // 没设 token 则不校验
	}
	t := r.Header.Get("Authorization")
	if t != "Bearer "+adminToken {
		return fmt.Errorf("invalid token")
	}
	return nil
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
