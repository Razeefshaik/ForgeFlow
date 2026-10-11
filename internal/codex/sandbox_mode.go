package codex

import (
	"errors"
	"strconv"
)

func ValidSandboxMode(mode string) bool {
	return mode == "auto" || mode == "mxc" || mode == "elevated" || mode == "unelevated"
}

func (c *CLI) SetSandboxMode(mode string) error {
	if !ValidSandboxMode(mode) {
		return errors.New("sandbox mode must be auto, mxc, elevated or unelevated")
	}
	c.modeMu.Lock()
	c.sandboxMode = mode
	c.modeMu.Unlock()
	return nil
}

func (c *CLI) SandboxMode() string {
	c.modeMu.RLock()
	defer c.modeMu.RUnlock()
	if c.sandboxMode == "" {
		return "auto"
	}
	return c.sandboxMode
}

func windowsPolicy(mode string) []string {
	if mode == "auto" {
		return []string{"-c", "features.prefer_mxc=true", "-c", `windows.sandbox="unelevated"`}
	}
	// Explicit legacy selection must not be silently replaced by MXC.
	return []string{"-c", "features.prefer_mxc=false", "-c", "windows.sandbox=" + strconv.Quote(mode)}
}
