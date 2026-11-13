// Package app is the CLI/MCP surface. M0 ships discovery; M2/M3 add the daemon,
// agent launch, and the first live tab enumeration over CDP; M4 adds DOM/AX snapshot
// and ref-based actions (click, fill, type, key, eval).
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	case "snapshot":
		return runSnapshot(args[1:])
	case "click":
		return runClick(args[1:])
	case "fill":
		return runFill(args[1:])
	case "type":
		return runType(args[1:])
	case "key":
		return runKey(args[1:])
	case "eval":
		return runEval(args[1:])
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
  bxw list browsers                   list installed Chromium browsers
  bxw list profiles <browser>         list a browser's profiles
  bxw launch <browser> [url]          start an isolated agent instance (CDP on)
  bxw list tabs [browser]             list open tabs of the bxw-managed instance
  bxw snapshot <tab> [--out <file>]   accessibility snapshot with stable element refs (e1, e2...)
  bxw click <tab> <ref|sel> [--double] click an element by ref or CSS selector
  bxw fill <tab> <ref|sel> <val>...   fill form inputs, selects, or toggles
  bxw type <tab> <text> [--submit <k>] type text into active element
  bxw key <tab> <combo>               press key or combination (e.g. Enter, Control+A)
  bxw eval <tab> <expr>               evaluate JavaScript on the tab
  bxw stop <browser>                  stop the daemon
  bxw doctor                          self-check this machine
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
		name, err := resolveBrowser(browserFlag(args[1:]))
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

func runSnapshot(args []string) error {
	var pos []string
	var browserName, outFile string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		case "--out":
			if i+1 < len(args) {
				outFile = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if strings.HasPrefix(args[i], "--out=") {
				outFile = strings.TrimPrefix(args[i], "--out=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	tab := ""
	if len(pos) >= 1 {
		tab = pos[0]
	}

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "snapshot", map[string]any{"tab": tab})
	if err != nil {
		return err
	}

	var snap daemon.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return exit.New(exit.Internal, "decode snapshot: %v", err)
	}

	text := snap.Text()
	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(text), 0o644); err != nil {
			return exit.New(exit.User, "write %s: %v", outFile, err)
		}
		fmt.Printf("wrote %s (%d elements)\n", outFile, len(snap.Nodes))
		return nil
	}

	fmt.Print(text)
	return nil
}

func runClick(args []string) error {
	var pos []string
	var browserName string
	double := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--double":
			double = true
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	if len(pos) < 2 {
		return exit.New(exit.Usage, "usage: bxw click <tab> <element> [--double]")
	}

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "click", map[string]any{
		"tab":      pos[0],
		"selector": pos[1],
		"double":   double,
	})
	if err != nil {
		return err
	}

	var res daemon.ActResult
	_ = json.Unmarshal(data, &res)

	action := "clicked"
	if double {
		action = "double-clicked"
	}
	fmt.Printf("%s %s\n", action, pos[1])
	if res.Navigated {
		fmt.Printf("navigated to %s (%s)\n", res.URL, res.Title)
	}
	return nil
}

func runFill(args []string) error {
	var pos []string
	var browserName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	if len(pos) < 3 || len(pos)%2 != 1 {
		return exit.New(exit.Usage, "usage: bxw fill <tab> <element> <value> [<element> <value> ...]")
	}

	tab := pos[0]
	var pairs [][2]string
	for i := 1; i < len(pos); i += 2 {
		pairs = append(pairs, [2]string{pos[i], pos[i+1]})
	}
	pairsJSON, _ := json.Marshal(pairs)

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "fill", map[string]any{
		"tab":   tab,
		"pairs": string(pairsJSON),
	})
	if err != nil {
		return err
	}

	var res daemon.ActResult
	_ = json.Unmarshal(data, &res)

	fmt.Printf("filled %d field(s)\n", len(pairs))
	if res.Navigated {
		fmt.Printf("navigated to %s (%s)\n", res.URL, res.Title)
	}
	return nil
}

func runType(args []string) error {
	var pos []string
	var browserName, submit string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		case "--submit":
			if i+1 < len(args) {
				submit = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if strings.HasPrefix(args[i], "--submit=") {
				submit = strings.TrimPrefix(args[i], "--submit=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	if len(pos) < 2 {
		return exit.New(exit.Usage, "usage: bxw type <tab> <text> [--submit <key>]")
	}

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "type", map[string]any{
		"tab":    pos[0],
		"text":   pos[1],
		"submit": submit,
	})
	if err != nil {
		return err
	}

	var res daemon.ActResult
	_ = json.Unmarshal(data, &res)

	fmt.Printf("typed %q\n", pos[1])
	if res.Navigated {
		fmt.Printf("navigated to %s (%s)\n", res.URL, res.Title)
	}
	return nil
}

func runKey(args []string) error {
	var pos []string
	var browserName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	if len(pos) < 2 {
		return exit.New(exit.Usage, "usage: bxw key <tab> <key> (such as Enter, Tab, Control+A)")
	}

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "key", map[string]any{
		"tab":  pos[0],
		"keys": pos[1],
	})
	if err != nil {
		return err
	}

	var res daemon.ActResult
	_ = json.Unmarshal(data, &res)

	fmt.Printf("pressed %s\n", pos[1])
	if res.Navigated {
		fmt.Printf("navigated to %s (%s)\n", res.URL, res.Title)
	}
	return nil
}

func runEval(args []string) error {
	var pos []string
	var browserName, filePath string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser":
			if i+1 < len(args) {
				browserName = args[i+1]
				i++
			}
		case "--file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--browser=") {
				browserName = strings.TrimPrefix(args[i], "--browser=")
			} else if strings.HasPrefix(args[i], "--file=") {
				filePath = strings.TrimPrefix(args[i], "--file=")
			} else if !strings.HasPrefix(args[i], "-") {
				pos = append(pos, args[i])
			}
		}
	}

	var script string
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return exit.New(exit.User, "read %s: %v", filePath, err)
		}
		script = string(b)
	} else if len(pos) >= 2 {
		script = pos[1]
	} else {
		return exit.New(exit.Usage, "usage: bxw eval <tab> '<expression>' | bxw eval <tab> --file <script.js>")
	}

	if len(pos) < 1 {
		return exit.New(exit.Usage, "usage: bxw eval <tab> '<expression>'")
	}

	name, err := resolveBrowser(browserName)
	if err != nil {
		return err
	}

	data, err := daemon.Call(name, "eval", map[string]any{
		"tab":    pos[0],
		"script": script,
	})
	if err != nil {
		return err
	}

	var formatted any
	if err := json.Unmarshal(data, &formatted); err == nil {
		pretty, _ := json.MarshalIndent(formatted, "", "  ")
		fmt.Println(string(pretty))
	} else {
		fmt.Println(string(data))
	}
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

func resolveBrowser(flag string) (string, error) {
	if flag != "" {
		if _, ok := browser.Find(flag); !ok {
			return "", exit.New(exit.NotFound, "browser %q not found (try: bxw list browsers)", flag)
		}
		return flag, nil
	}
	bs := browser.Installed()
	for _, b := range bs {
		if daemon.Running(b.Name) {
			return b.Name, nil
		}
	}
	if len(bs) > 0 {
		return bs[0].Name, nil
	}
	return "", exit.New(exit.NotFound, "no supported Chromium browser found")
}

func browserFlag(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--browser" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "--browser=") {
			return strings.TrimPrefix(args[i], "--browser=")
		}
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return ""
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
