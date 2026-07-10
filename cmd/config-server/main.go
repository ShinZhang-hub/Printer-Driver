package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"printer-installer/internal/config"
)

var (
	store     *config.Config
	storeLock sync.RWMutex
)

func main() {
	store = loadStore()
	port := ":9527"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/config", handleConfig)
	mux.HandleFunc("/api/v1/driver", handleDriver)

	log.Printf("Config server started at http://0.0.0.0%s", port)
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

// GET /api/v1/config → returns full config
// PUT /api/v1/config → updates config (requires token)
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

// GET /api/v1/driver?model=C3070&brand=fujifilm → returns matching driver info
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

// --- storage ---

var storeFile string
var adminToken string

func init() {
	storeFile = defaultStorePath()
	adminToken = os.Getenv("ADMIN_TOKEN")
}

func defaultStorePath() string {
	dir := filepath.Join("/var", "lib", "printer-installer", "server")
	if runtime.GOOS == "windows" {
		dir = filepath.Join("C:\\ProgramData", "PrinterInstaller", "server")
	}
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config-server.json")
}

func loadStore() *config.Config {
	f, err := os.Open(storeFile)
	if err != nil {
		log.Printf("no persistent config file, using defaults: %v", err)
		cfg := defaultServerConfig()
		saveStore(cfg)
		return cfg
	}
	defer f.Close()
	cfg := defaultServerConfig()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		log.Printf("failed to parse config, using defaults: %v", err)
		saveStore(cfg)
	}
	return cfg
}

func saveStore(cfg *config.Config) {
	f, err := os.Create(storeFile)
	if err != nil {
		log.Printf("failed to save config: %v", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(cfg)
}

func defaultServerConfig() *config.Config {
	return &config.Config{
		Version:   1,
		UpdatedAt: "2024-01-01T00:00:00Z",
		ConfigURL: "http://30.61.40.61:9527",
		Locations: []config.LocationConfig{
			{
				Name:         "JP-Tower",
				Subnets:      []string{"30.61.40.0/24", "30.61.39.0/24"},
				PrinterIP:    "30.61.40.40",
				PrinterName:  "Printer-Osaka",
				PrinterModel: "FF Apeos C2571",
				PortNumber:   9100,
				Protocol:     "raw",
			},
			{
				Name:         "Business-Tower",
				Subnets:      []string{"30.61.30.0/24", "30.61.31.0/24", "30.61.32.0/24"},
				PrinterIP:    "30.61.30.30",
				PrinterName:  "Printer-Tencent",
				PrinterModel: "FF Apeos C3070",
				PortNumber:   9100,
				Protocol:     "raw",
			},
		},
		Drivers: []config.DriverConfig{
			{
				Brand:       "fujifilm",
				Model:       "FF Apeos C2571",
				ID:          "fujifilm-ff-apeos-c2571",
				InstallArgs: []string{"/S"},
				Version:     "1.0.0",
				Enabled:     true,
			},
		},
	}
}

// --- auth ---

func requireToken(r *http.Request) error {
	if adminToken == "" {
		return nil // no token configured, skip check
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
