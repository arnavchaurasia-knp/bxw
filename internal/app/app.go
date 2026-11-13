// Package app is the CLI/MCP surface. M0 ships discovery; M2/M3 add the daemon,
// agent launch, and the first live tab enumeration over CDP.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"bxw/internal/browser"
	"bxw/internal/daemon"
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
	case "launch":
		return runLaunch(args[1:])
	case "stop":
		return runStop(args[1:])
	case "doctor":
		return runDoctor()
	case "__daemon": // internal: the background singleton entrypoint
		return runDaemon(args[1:])
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
  bxw launch <browser>         start an isolated agent instance (CDP on)
  bxw list tabs [browser]      list open tabs of the bxw-managed instance
  bxw stop <browser>           stop the daemon
  bxw doctor                   self-check this machine
  bxw version
`)
	return nil
}

func runList(args []string) error {
	if len(args) == 0 {
		return exit.New(exit.Usage, "usage: bxw list <browsers|profiles|tabs> [browser]")
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
	case "tabs":
		name, err := browserArg(args[1:])
		if err != nil {
			return err
		}
		data, err := daemon.Call(name, "list_tabs", nil)
		if err != nil {
			return exit.New(exit.Upstream, "%v", err)
		}
		var tabs []daemon.Tab
		if err := json.Unmarshal(data, &tabs); err != nil {
			return exit.New(exit.Internal, "decode tabs: %v", err)
		}
		if len(tabs) == 0 {
			fmt.Println("(no open tabs)")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tURL")
		for _, t := range tabs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", short(t.ID), t.Title, t.URL)
		}
		return w.Flush()
	default:
		return exit.New(exit.Usage, "unknown list target %q (want: browsers|profiles|tabs)", args[0])
	}
}

func runLaunch(args []string) error {
	if len(args) < 1 {
		return exit.New(exit.Usage, "usage: bxw launch <browser> [url] [--headless]")
	}
	headless := false
	targetURL := "about:blank"
	var browserName string

	for _, a := range args {
		if a == "--headless" {
			headless = true
		} else if browserName == "" {
			browserName = a
		} else if targetURL == "about:blank" {
			targetURL = a
		}
	}

	b, ok := browser.Find(browserName)
	if !ok {
		return exit.New(exit.NotFound, "browser %q not installed (try: bxw list browsers)", browserName)
	}
	data, err := daemon.Call(b.Name, "launch", map[string]any{"headless": headless, "url": targetURL})
	if err != nil {
		return exit.New(exit.Upstream, "%v", err)
	}
	var res map[string]any
	_ = json.Unmarshal(data, &res)
	ws := res["ws"]
	if ws == nil {
		ws = string(data)
	}
	fmt.Printf("launched %s agent instance (CDP ready: %v)\n", b.Label, ws)
	return nil
}

func runStop(args []string) error {
	if len(args) < 1 {
		return exit.New(exit.Usage, "usage: bxw stop <browser>")
	}
	if _, err := daemon.Call(args[0], "stop", nil); err != nil {
		return exit.New(exit.Upstream, "%v", err)
	}
	fmt.Printf("stopped daemon for %s\n", args[0])
	return nil
}

func runDaemon(args []string) error {
	name := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--browser" && i+1 < len(args) {
			name = args[i+1]
			i++
		}
	}
	if name == "" {
		return exit.New(exit.Usage, "__daemon requires --browser <name>")
	}
	return daemon.Run(name)
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

// browserArg returns the named browser, or the first installed one if omitted.
func browserArg(args []string) (string, error) {
	if len(args) >= 1 {
		if _, ok := browser.Find(args[0]); !ok {
			return "", exit.New(exit.NotFound, "browser %q not installed (try: bxw list browsers)", args[0])
		}
		return args[0], nil
	}
	bs := browser.Installed()
	if len(bs) == 0 {
		return "", exit.New(exit.NotFound, "no supported Chromium browser found")
	}
	return bs[0].Name, nil
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
