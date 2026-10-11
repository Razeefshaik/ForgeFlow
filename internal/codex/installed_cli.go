package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Only default resolution considers already-installed official CLI releases.
// An explicit FORGEFLOW_CODEX_BIN/local setting remains authoritative. No install,
// account change, administrator setup or unsandboxed repository execution occurs.
func newestInstalledWindowsCLI(fallback, home string) string {
	paths, _ := filepath.Glob(filepath.Join(home, ".codex", "packages", "app-server-daemon", "releases", "*", "bin", "codex.exe"))
	// Bound version probes; release directories contain version prefixes.
	sort.Slice(paths, func(i, j int) bool { return newerVersion(releaseVersion(paths[i]), releaseVersion(paths[j])) })
	if len(paths) > 8 {
		paths = paths[:8]
	}
	paths = append([]string{fallback}, paths...)
	best := fallback
	var version [3]int
	for _, path := range paths {
		if path == "" {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, path, "--version")
		configureProcess(cmd)
		out, err := cmd.Output()
		cancel()
		if err != nil {
			continue
		}
		var v [3]int
		if _, err = fmt.Sscanf(strings.TrimSpace(string(out)), "codex-cli %d.%d.%d", &v[0], &v[1], &v[2]); err != nil {
			continue
		}
		if best == "" || newerVersion(v, version) {
			best, version = path, v
		}
	}
	return best
}

func releaseVersion(path string) [3]int {
	var v [3]int
	_, _ = fmt.Sscanf(filepath.Base(filepath.Dir(filepath.Dir(path))), "%d.%d.%d", &v[0], &v[1], &v[2])
	return v
}
func newerVersion(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
