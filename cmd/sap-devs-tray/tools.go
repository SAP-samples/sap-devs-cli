//go:build tools

package main

import (
	// Ensure Wails v3 is pinned in go.mod for the tray binary.
	_ "github.com/wailsapp/wails/v3/pkg/application"
)
