//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
	"time"
)

func formatUptime(this js.Value, args []js.Value) interface{} {
	seconds := args[0].Int()
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%d days %d hours %d min", days, hours, mins)
}

func formatBytes(this js.Value, args []js.Value) interface{} {
	bytes := args[0].Float()
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", bytes/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", bytes/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.0f KB", bytes/1024)
	default:
		return fmt.Sprintf("%.0f B", bytes)
	}
}

func main() {
	js.Global().Set("goFormatUptime", js.FuncOf(formatUptime))
	js.Global().Set("goFormatBytes", js.FuncOf(formatBytes))
	// Keep alive
	select {}
}
