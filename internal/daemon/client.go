package daemon

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"time"

	"bxw/internal/exit"
	"bxw/internal/osport"
)

const connectWait = 2 * time.Minute

// Run is the entrypoint for the background daemon process (`bxw __daemon --browser <name>`).
func Run(browserName string) error {
	code := Serve(browserName)
	if code != 0 {
		return exit.New(exit.Class(code), "daemon exited with code %d", code)
	}
	return nil
}

// Running reports whether a daemon answers on the browser's socket.
func Running(browser string) bool {
	p := osport.Current()
	sock := p.SocketPath(browser)
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Call sends an RPC request to the browser's daemon and returns the response Data bytes.
// If the daemon is not running, it starts it first (unless op == "stop").
func Call(browser, op string, args any) ([]byte, error) {
	p := osport.Current()
	sock := p.SocketPath(browser)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		if op == "stop" {
			return nil, nil // stopping a non-running daemon is a no-op
		}
		if conn, err = startAndDial(browser); err != nil {
			return nil, err
		}
	}
	defer conn.Close()

	var argsMap map[string]any
	if args != nil {
		if m, ok := args.(map[string]any); ok {
			argsMap = m
		} else {
			b, _ := json.Marshal(args)
			_ = json.Unmarshal(b, &argsMap)
		}
	}
	req := Request{Op: op, Args: argsMap}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, exit.New(exit.Upstream, "send to daemon: %v", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, exit.New(exit.Upstream, "daemon closed connection: %v", err)
	}
	if resp.Error != "" {
		code := resp.Code
		if code == 0 {
			code = int(exit.Upstream)
		}
		return nil, exit.New(exit.Class(code), "%s", resp.Error)
	}
	return resp.Data, nil
}

// startAndDial ensures only one daemon starts by holding an exclusive lock.
func startAndDial(browser string) (net.Conn, error) {
	p := osport.Current()
	lockPath := p.LockPath(browser)
	sockPath := p.SocketPath(browser)
	logPath := p.LogPath(browser)

	unlock, err := p.Lock(lockPath)
	if err != nil {
		return nil, exit.New(exit.Upstream, "acquire daemon lock: %v", err)
	}
	defer unlock()

	// Check if another command started the daemon while we waited for the lock
	if conn, err := net.Dial("unix", sockPath); err == nil {
		return conn, nil
	}

	if err := spawn(browser, logPath); err != nil {
		return nil, err
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, exit.New(exit.Upstream, "daemon started but socket does not answer: %v (see %s)", err, logPath)
	}
	return conn, nil
}

func spawn(browser, logPath string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	p := osport.Current()
	proc, stdout, err := p.SpawnDaemon(self, browser, logPath)
	if err != nil {
		return exit.New(exit.Upstream, "start daemon: %v", err)
	}
	defer stdout.Close()
	defer proc.Release()

	lineChan := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(stdout).ReadString('\n')
		lineChan <- s
	}()

	select {
	case s := <-lineChan:
		return handshake(s, logPath)
	case <-time.After(connectWait + 10*time.Second):
		_ = proc.Kill()
		return exit.New(exit.Permission, "timed out waiting for the browser to allow the connection")
	}
}

func handshake(s, logPath string) error {
	var r Response
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return exit.New(exit.Upstream, "daemon failed to start (see %s)", logPath)
	}
	if r.Error != "" {
		code := r.Code
		if code == 0 {
			code = int(exit.Upstream)
		}
		return exit.New(exit.Class(code), "%s", r.Error)
	}
	return nil
}
