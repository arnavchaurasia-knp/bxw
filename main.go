// Command bxw lets coding agents drive the Windows Chromium browser you
// already have open, in the profile you name, over the DevTools Protocol.
//
// It is a single binary with two roles: a foreground command/MCP server, and
// a background daemon that holds one CDP connection. This file only dispatches.
package main

import (
	"os"

	"bxw/internal/app"
	"bxw/internal/exit"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		os.Stderr.WriteString("bxw: " + err.Error() + "\n")
		os.Exit(exit.Code(err))
	}
}
