package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend/icon.png
var trayIcon []byte

func startApp(srv *Server) error {
	app := application.New(application.Options{
		Name:        "sap-devs",
		Description: "SAP Developer Companion",
		Assets:      application.AlphaAssets,
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "sap-devs Dashboard",
		Width:           400,
		Height:          550,
		URL:             srv.PanelURL(),
		Frameless:       true,
		AlwaysOnTop:     true,
		Hidden:          true,
		DisableResize:   true,
		HideOnFocusLost: true,
		HideOnEscape:    true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	})

	panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		panel.Hide()
		e.Cancel()
	})

	srv.hideFunc = func() { panel.Hide() }

	configWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "sap-devs Config",
		Width:  520,
		Height: 700,
		URL:    srv.ConfigURL(),
		Hidden: true,
	})

	configWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		configWin.Hide()
		e.Cancel()
	})

	srv.configWindowFunc = func() {
		configWin.SetURL(srv.ConfigURL())
		configWin.Show()
		configWin.Focus()
	}

	systemTray := app.SystemTray.New()
	systemTray.SetIcon(trayIcon)
	systemTray.SetTooltip("sap-devs")

	menu := app.NewMenu()
	menu.Add(fmt.Sprintf("sap-devs %s", version)).SetEnabled(false)
	menu.AddSeparator()

	menu.Add("Sync Now").OnClick(func(ctx *application.Context) {
		go func() {
			cmd := exec.Command(sapDevsBinary(), "sync")
			_ = cmd.Run()
		}()
	})
	menu.Add("Inject Now").OnClick(func(ctx *application.Context) {
		go func() {
			cmd := exec.Command(sapDevsBinary(), "inject", "--no-sync")
			_ = cmd.Run()
		}()
	})

	menu.AddSeparator()
	menu.Add("Config...").OnClick(func(ctx *application.Context) {
		if srv.configWindowFunc != nil {
			srv.configWindowFunc()
		}
	})

	menu.AddSeparator()
	menu.Add("Open Terminal...").OnClick(func(ctx *application.Context) {
		openTerminal()
	})

	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.Quit()
	})

	systemTray.SetMenu(menu)
	systemTray.AttachWindow(panel).WindowOffset(2)

	return app.Run()
}

func openTerminal() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wt.exe"); err == nil {
			cmd = exec.Command("cmd", "/c", "start", "", "wt")
		} else {
			cmd = exec.Command("cmd", "/c", "start", "", "powershell")
		}
	case "darwin":
		cmd = exec.Command("open", "-a", "Terminal")
	default:
		if term := envOr("TERMINAL", ""); term != "" {
			cmd = exec.Command(term)
		} else if path, err := exec.LookPath("x-terminal-emulator"); err == nil {
			cmd = exec.Command(path)
		} else {
			cmd = exec.Command("xterm")
		}
	}
	_ = cmd.Start()
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
