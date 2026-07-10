package drvpack

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func extract(dmgPath string) (string, error) {
	dmgPath, err := filepath.Abs(dmgPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve DMG path: %w", err)
	}

	workDir, err := os.MkdirTemp("", "printer-installer-dmg-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	mountPoint := filepath.Join(workDir, "mnt")
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to create mount point: %w", err)
	}

	cmd := exec.Command("hdiutil", "attach", dmgPath, "-mountpoint", mountPoint, "-nobrowse", "-quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("DMG mount failed: %s\n%w", strings.TrimSpace(string(out)), err)
	}

	ppdOutput := filepath.Join(workDir, "ppds")
	os.MkdirAll(ppdOutput, 0755)

	collectPPDs(mountPoint, ppdOutput)

	if isEmptyDir(ppdOutput) {
		filepath.WalkDir(mountPoint, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".pkg") {
				expandDir := filepath.Join(workDir, "pkg-expand")
				os.RemoveAll(expandDir)

				if exec.Command("pkgutil", "--expand", path, expandDir).Run() != nil {
					return nil
				}

				collectPPDs(expandDir, ppdOutput)

				filepath.WalkDir(expandDir, func(p string, d2 os.DirEntry, err2 error) error {
					if err2 != nil || d2.IsDir() {
						return nil
					}
					if strings.EqualFold(filepath.Base(p), "payload") {
						payloadDir := filepath.Join(workDir, "payload-extract")
						os.RemoveAll(payloadDir)
						os.MkdirAll(payloadDir, 0755)

						extractPayload(p, payloadDir)
						collectPPDs(payloadDir, ppdOutput)
						installDriverComponents(payloadDir)
					}
					return nil
				})
			}
			return nil
		})
	}

	exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()

	if isEmptyDir(ppdOutput) {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("no PPD files found in DMG")
	}

	return ppdOutput, nil
}

func extractPayload(payloadPath, dest string) {
	f, err := os.Open(payloadPath)
	if err != nil {
		return
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return
	}
	defer gr.Close()

	exec.Command("cpio", "-id").Run()
	// cpio needs to run in dest dir with stdin from the decompressed payload
	cmd := exec.Command("cpio", "-i", "-d")
	cmd.Dir = dest
	cmd.Stdin = gr
	cmd.Run()
}

func collectPPDs(srcDir, dstDir string) {
	filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		base := filepath.Base(path)

		if ext == ".ppd" {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			os.WriteFile(filepath.Join(dstDir, base), data, 0644)
			return nil
		}

		if ext == ".gz" {
			tmpPath := filepath.Join(dstDir, ".tmp-"+base)
			if err := decompressGzip(path, tmpPath); err != nil {
				return nil
			}
			defer os.Remove(tmpPath)

			data, readErr := os.ReadFile(tmpPath)
			if readErr != nil {
				return nil
			}
			if isPPDContent(data) {
				outName := strings.TrimSuffix(base, ".gz") + ".ppd"
				os.WriteFile(filepath.Join(dstDir, outName), data, 0644)
			}
		}
		return nil
	})
}

func isPPDContent(data []byte) bool {
	head := string(data)
	if len(head) > 512 {
		head = head[:512]
	}
	return strings.Contains(head, "*PPD-Adobe:") || strings.Contains(head, "*ModelName:")
}

func decompressGzip(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, gr)
	return err
}

func isEmptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

func installDriverComponents(payloadDir string) {
	if os.Geteuid() != 0 {
		return
	}

	src := filepath.Join(payloadDir, "FUJIFILM")
	if _, err := os.Stat(src); err != nil {
		return
	}

	dst := "/Library/Printers/FUJIFILM"
	os.MkdirAll(dst, 0755)

	filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			os.MkdirAll(target, 0755)
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		mode := os.FileMode(0644)
		if filepath.Base(filepath.Dir(path)) == "MacOS" {
			mode = 0555
		}
		if strings.HasSuffix(target, "FFACMMCFilter") {
			mode = 0555
		}
		os.WriteFile(target, data, mode)
		return nil
	})
}
