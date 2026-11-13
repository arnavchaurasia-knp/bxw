// Package app is the CLI/MCP surface. M0 ships discovery commands only;
// attach/snapshot/act land in later milestones.
package app

import (
	"fmt"
	"os"
	"text/tabwriter"

	"bxw/internal/browser"
	"bxw/internal/exit"
)

// Version is overridden at build time via -ldflags "-X bxw/internal/app.Version=...".
var Version = "0.0.0-dev"

// Run dispatches a command line (os.Args[1:]).
func Run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("bxw", Version)
		return nil
	case "list":
		return runList(args[1:])
	case "doctor":
		return runDoctor()
	case "help", "-h", "--help":
		return usage()
	default:
		return exit.New(exit.Usage, "unknown command %q (try: bxw help)", args[0])
	}
}

func usage() error {
	fmt.Print(`bxw — drive your Windows Chromium browser by profile over CDP

usage:
  bxw list browsers            list installed Chromium browsers
  bxw list profiles <browser>  list a browser's profiles
  bxw doctor                   self-check this machine
  bxw version
`)
	return nil
}

func runList(args []string) error {
	if len(args) == 0 {
		return exit.New(exit.Usage, "usage: bxw list <browsers|profiles> [browser]")
	}
	switch args[0] {
	case "browsers":
		bs := browser.Installed()
		if len(bs) == 0 {
			return exit.New(exit.NotFound, "no supported Chromium browser found")
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tEXE\tUSER DATA")
		for _, b := range bs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", b.Name, b.Exe, b.DataDir)
		}
		return w.Flush()
	case "profiles":
		if len(args) < 2 {
			return exit.New(exit.Usage, "usage: bxw list profiles <browser>")
		}
		b, ok := browser.Find(args[1])
		if !ok {
			return exit.New(exit.NotFound, "browser %q not installed (try: bxw list browsers)", args[1])
		}
		ps, err := browser.Profiles(b)
		if err != nil {
			return exit.New(exit.Upstream, "%v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DIR\tNAME")
		for _, p := range ps {
			fmt.Fprintf(w, "%s\t%s\n", p.Dir, p.Name)
		}
		return w.Flush()
	default:
		return exit.New(exit.Usage, "unknown list target %q (want: browsers|profiles)", args[0])
	}
}

func runDoctor() error {
	fmt.Println("bxw doctor —", Version)
	bs := browser.Installed()
	fmt.Printf("  Chromium browsers found: %d\n", len(bs))
	for _, b := range bs {
		ps, err := browser.Profiles(b)
		status := fmt.Sprintf("%d profile(s)", len(ps))
		if err != nil {
			status = "profiles unreadable: " + err.Error()
		}
		fmt.Printf("    - %-9s %s  (%s)\n", b.Name, b.Exe, status)
	}
	if len(bs) == 0 {
		return exit.New(exit.NotFound, "no supported Chromium browser found")
	}
	return nil
}
