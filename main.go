package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogpu/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/rac-developer/omnifloat/pkg/ipc"
)

//go:embed all:frontend
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

type islandFS struct {
	fs.FS
}

func (i islandFS) Open(name string) (fs.File, error) {
	if name == "index.html" {
		return i.FS.Open("island.html")
	}
	return i.FS.Open(name)
}

func main() {
	startHidden := false
	isToggleFlag := false
	isIslandFlag := false

	for _, arg := range os.Args[1:] {
		switch strings.ToLower(arg) {
		case "--island":
			isIslandFlag = true
		case "--hidden", "-d", "--daemon":
			startHidden = true
		case "--toggle", "-t":
			isToggleFlag = true
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		}
	}

	// 1. If another instance is running, signal it via IPC and exit (unless starting island)
	if !isIslandFlag {
		if isToggleFlag {
			if ipc.TrySendToggle() {
				os.Exit(0)
			}
		} else if !startHidden {
			if ipc.SendCommand("SHOW") {
				fmt.Println("OmniFloat is already running. Showing active launcher.")
				os.Exit(0)
			}
		}
	}

	// 2. Initialize application state
	app := NewApp(isIslandFlag)
	app.SetStartHidden(startHidden)

	// 3. Enforce single instance & IPC handling
	if !isIslandFlag {
		ipcServer, err := ipc.StartServer(func(cmd string) {
			switch strings.ToUpper(strings.TrimSpace(cmd)) {
			case "TOGGLE":
				app.ToggleLauncher()
			case "SHOW":
				app.ShowLauncher()
			case "SHOW_SETTINGS":
				app.ShowLauncher()
				if app.ctx != nil {
					wailsRuntime.EventsEmit(app.ctx, "launcher:open_settings")
				}
			case "HIDE":
				app.HideLauncher()
			case "CONFIG_UPDATED":
				app.ReloadConfigFromDisk()
			case "QUIT":
				app.QuitApp()
			}
		})
		if err != nil {
			log.Printf("[OmniFloat] Notice: IPC server not bound: %v", err)
		} else {
			app.SetIPCServer(ipcServer)
		}

		// Initialize System Tray in background (for main launcher process)
		initSystemTray(app, appIcon)
	}

	// Resolve embedded filesystem root
	var appFS fs.FS = assets
	if sub, err := fs.Sub(assets, "frontend"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			appFS = sub
		}
	}

	if isIslandFlag {
		appFS = islandFS{appFS}
	}

	windowWidth := LauncherWidth
	windowHeight := LauncherHeight
	windowTitle := "OmniFloat"

	if isIslandFlag {
		windowWidth = IslandWidth
		windowHeight = IslandHeight
		windowTitle = "OmniFloat Island"
	}

	userDataDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "OmniFloat", "WebView2")
	if isIslandFlag {
		userDataDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "OmniFloat", "WebView2_Island")
	}

	// 4. Configure Wails Window & Glassmorphism Options
	wailsOptions := &options.App{
		Title:             windowTitle,
		Width:             windowWidth,
		Height:            windowHeight,
		DisableResize:     true,
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       startHidden || isIslandFlag,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{
			Assets: appFS,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:                true,
			WindowIsTranslucent:                 true,
			DisableFramelessWindowDecorations:   true,
			DisableWindowIcon:                   true,
			WebviewUserDataPath:                 userDataDir,
			WebviewDisableRendererCodeIntegrity: true,
			Theme:                               windows.Dark,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: true,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
		},
	}

	// 5. Launch Wails application loop
	if err := wails.Run(wailsOptions); err != nil {
		log.Fatalf("Error running OmniFloat: %v", err)
	}
}

func initSystemTray(app *App, icon []byte) {
	menu := systray.NewMenu()
	menu.Add("Abrir Buscador (Alt+Espacio)", func() {
		app.ShowLauncher()
	})
	menu.Add("Alternar Buscador", func() {
		app.ToggleLauncher()
	})
	menu.AddSeparator()
	menu.Add("Configuración...", func() {
		app.ShowLauncher()
		// Open settings modal
		wailsRuntime.EventsEmit(app.ctx, "launcher:open_settings")
	})
	menu.Add("Buscar Actualizaciones...", func() {
		go func() {
			info, err := app.CheckForUpdates()
			if err == nil && info != nil && info.Available {
				app.ShowLauncher()
			}
		}()
	})
	menu.AddSeparator()
	menu.Add("Salir de OmniFloat", func() {
		app.QuitApp()
	})

	tray := systray.New().
		SetAppName("OmniFloat").
		SetTooltip("OmniFloat (Alt+Espacio)").
		SetMenu(menu).
		OnClick(func() {
			app.ToggleLauncher()
		})

	if len(icon) > 0 {
		tray.SetIcon(icon)
	}

	tray.Show()

	go func() {
		if err := tray.Run(); err != nil {
			log.Printf("[OmniFloat Tray] Notice: %v", err)
		}
	}()
}

func printHelp() {
	fmt.Println("OmniFloat - Lanzador flotante minimalista con Glassmorphism")
	fmt.Println()
	fmt.Println("Uso:")
	fmt.Println("  omnifloat [opciones]")
	fmt.Println()
	fmt.Println("Opciones:")
	fmt.Println("  --toggle, -t   Alternar visibilidad del buscador (o iniciar si no está en ejecución)")
	fmt.Println("  --hidden, -d   Iniciar oculto en segundo plano (bandeja del sistema)")
	fmt.Println("  --island       Ejecutar widget independiente de Isla Flotante")
	fmt.Println("  --help, -h     Mostrar esta ayuda")
	fmt.Println()
	fmt.Println("Atajo Global:")
	fmt.Println("  Alt + Espacio  Abrir paleta de comandos centrada en pantalla")
}
