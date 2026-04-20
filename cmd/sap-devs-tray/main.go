package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("sap-devs-tray %s\n", version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configDir := defaultConfigDir()
	cacheDir := defaultCacheDir()

	srv, err := NewServer(configDir, cacheDir)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}
	go srv.Start()

	return startApp(srv)
}
