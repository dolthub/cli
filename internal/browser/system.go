package browser

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

type startFunc func(name string, args ...string) error

// System opens web URLs with the operating system's default browser.
type System struct {
	goos  string
	start startFunc
}

// NewSystem constructs the production browser implementation.
func NewSystem() System {
	return System{goos: runtime.GOOS, start: startCommand}
}

func startCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Browse opens rawURL without waiting for the browser process to exit.
func (s System) Browse(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("browser URL must be an absolute HTTP or HTTPS URL")
	}
	start := s.start
	if start == nil {
		start = startCommand
	}
	var name string
	var args []string
	switch s.goos {
	case "darwin":
		name, args = "open", []string{rawURL}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	case "linux", "freebsd", "openbsd", "netbsd":
		name, args = "xdg-open", []string{rawURL}
	default:
		return fmt.Errorf("opening a browser is not supported on %s", s.goos)
	}
	if err := start(name, args...); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
