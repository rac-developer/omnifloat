package apps

import (
	"strings"
	"sync"
)

// AppItem represents an executable application or system utility found on the host.
type AppItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Category    string `json:"category"` // "app", "system", "tool"
	Description string `json:"description"`
}

// Indexer caches and searches applications on the system.
type Indexer struct {
	mu    sync.RWMutex
	items []AppItem
}

// NewIndexer creates a new indexer and triggers an initial background scan.
func NewIndexer() *Indexer {
	idx := &Indexer{
		items: make([]AppItem, 0),
	}
	go idx.Scan()
	return idx
}

// GetBuiltinSystemApps returns standard built-in utilities.
func GetBuiltinSystemApps() []AppItem {
	return []AppItem{
		{
			ID:          "sys-terminal",
			Name:        "Terminal de Windows",
			Path:        "wt.exe",
			Category:    "system",
			Description: "Línea de comandos moderna de Windows",
		},
		{
			ID:          "sys-powershell",
			Name:        "Windows PowerShell",
			Path:        "powershell.exe",
			Category:    "system",
			Description: "Consola de automatización y administración",
		},
		{
			ID:          "sys-cmd",
			Name:        "Símbolo del sistema (CMD)",
			Path:        "cmd.exe",
			Category:    "system",
			Description: "Intérprete de comandos clásico",
		},
		{
			ID:          "sys-taskmgr",
			Name:        "Administrador de tareas",
			Path:        "taskmgr.exe",
			Category:    "system",
			Description: "Monitor de procesos, rendimiento y recursos",
		},
		{
			ID:          "sys-notepad",
			Name:        "Bloc de notas (Notepad)",
			Path:        "notepad.exe",
			Category:    "tool",
			Description: "Editor de texto rápido",
		},
		{
			ID:          "sys-calc",
			Name:        "Calculadora",
			Path:        "calc.exe",
			Category:    "tool",
			Description: "Calculadora estándar y científica",
		},
		{
			ID:          "sys-explorer",
			Name:        "Explorador de archivos",
			Path:        "explorer.exe",
			Category:    "system",
			Description: "Gestor de carpetas y archivos",
		},
		{
			ID:          "sys-settings",
			Name:        "Configuración del Sistema",
			Path:        "ms-settings:",
			Category:    "system",
			Description: "Ajustes y personalización de Windows",
		},
		{
			ID:          "sys-control",
			Name:        "Panel de control",
			Path:        "control.exe",
			Category:    "system",
			Description: "Herramientas de configuración clásicas",
		},
		{
			ID:          "sys-paint",
			Name:        "Paint",
			Path:        "mspaint.exe",
			Category:    "tool",
			Description: "Editor gráfico básico",
		},
		{
			ID:          "sys-snipping",
			Name:        "Herramienta Recortes",
			Path:        "snippingtool.exe",
			Category:    "tool",
			Description: "Capturas de pantalla y anotaciones",
		},
	}
}

// Search searches indexed applications using substring and prefix matching.
func (idx *Indexer) Search(query string) []AppItem {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		// Return system apps as default recommendations
		if len(idx.items) > 6 {
			return idx.items[:6]
		}
		return idx.items
	}

	var exact []AppItem
	var prefix []AppItem
	var contains []AppItem

	for _, item := range idx.items {
		nameLower := strings.ToLower(item.Name)
		if nameLower == q {
			exact = append(exact, item)
		} else if strings.HasPrefix(nameLower, q) {
			prefix = append(prefix, item)
		} else if strings.Contains(nameLower, q) || strings.Contains(strings.ToLower(item.Description), q) {
			contains = append(contains, item)
		}
	}

	results := append(exact, prefix...)
	results = append(results, contains...)

	if len(results) > 10 {
		return results[:10]
	}
	return results
}
