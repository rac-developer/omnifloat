//go:build !windows

package apps

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scan parses Linux .desktop files from /usr/share/applications and user local directories.
func (idx *Indexer) Scan() {
	var collected []AppItem
	seen := make(map[string]bool)

	// Add built-ins
	for _, sys := range GetBuiltinSystemApps() {
		collected = append(collected, sys)
		seen[strings.ToLower(sys.Name)] = true
	}

	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		"/usr/share/applications",
		filepath.Join(homeDir, ".local", "share", "applications"),
	}

	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".desktop") {
				continue
			}

			fullPath := filepath.Join(dir, f.Name())
			name, execCmd, noDisplay := parseDesktopFile(fullPath)
			if noDisplay || name == "" || execCmd == "" {
				continue
			}

			lowerName := strings.ToLower(name)
			if seen[lowerName] {
				continue
			}
			seen[lowerName] = true

			collected = append(collected, AppItem{
				ID:          "app-" + lowerName,
				Name:        name,
				Path:        execCmd,
				Category:    "app",
				Description: "Aplicación Linux",
			})
		}
	}

	idx.mu.Lock()
	idx.items = collected
	idx.mu.Unlock()

	log.Printf("[OmniFloat Apps] Indexed %d applications on Linux", len(collected))
}

func parseDesktopFile(path string) (name, execCmd string, noDisplay bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", true
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Name=") && name == "" {
			name = strings.TrimPrefix(line, "Name=")
		} else if strings.HasPrefix(line, "Exec=") && execCmd == "" {
			execCmd = strings.TrimPrefix(line, "Exec=")
			// Strip field codes like %u, %f, %U
			fields := strings.Fields(execCmd)
			var clean []string
			for _, f := range fields {
				if !strings.HasPrefix(f, "%") {
					clean = append(clean, f)
				}
			}
			execCmd = strings.Join(clean, " ")
		} else if strings.HasPrefix(line, "NoDisplay=true") {
			noDisplay = true
		}
	}
	return name, execCmd, noDisplay
}

// Launch executes the command on Linux.
func Launch(target string) error {
	cmd := exec.Command("sh", "-c", target)
	return cmd.Start()
}
