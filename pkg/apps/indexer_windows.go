//go:build windows

package apps

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Scan walks Windows Start Menu directories to index installed applications (.lnk files).
func (idx *Indexer) Scan() {
	var collected []AppItem
	seen := make(map[string]bool)

	// Add built-in system tools first
	for _, sys := range GetBuiltinSystemApps() {
		collected = append(collected, sys)
		seen[strings.ToLower(sys.Name)] = true
	}

	searchDirs := []string{
		filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs"),
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs"),
	}

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".lnk" {
				return nil
			}

			rawName := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			cleanName := strings.TrimSpace(rawName)
			lowerName := strings.ToLower(cleanName)

			// Filter out uninstallers and readme/documentation links
			if strings.Contains(lowerName, "uninstall") ||
				strings.Contains(lowerName, "desinstalar") ||
				strings.Contains(lowerName, "help") ||
				strings.Contains(lowerName, "ayuda") ||
				strings.Contains(lowerName, "readme") ||
				strings.Contains(lowerName, "leeme") {
				return nil
			}

			if seen[lowerName] {
				return nil
			}
			seen[lowerName] = true

			collected = append(collected, AppItem{
				ID:          "app-" + lowerName,
				Name:        cleanName,
				Path:        path,
				Category:    "app",
				Description: "Aplicación de Windows",
			})

			return nil
		})
	}

	idx.mu.Lock()
	idx.items = collected
	idx.mu.Unlock()

	log.Printf("[OmniFloat Apps] Indexed %d applications and system tools", len(collected))
}

// Launch executes the target shortcut, executable, or system protocol URI without showing a console.
func Launch(target string) error {
	cmd := exec.Command("cmd", "/c", "start", "", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
