package codex

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type sandboxRuntimeInUseError struct {
	Path  string
	Cause error
}

func (e *sandboxRuntimeInUseError) Error() string {
	return fmt.Sprintf("Codex's runtime file is in use: %s (Windows error 32). Close the Codex desktop app and affected VS Code sessions, then retry sandbox setup from ForgeFlow in your browser. Their background helpers can restart while those apps remain open. Recorded helper error: %v", e.Path, e.Cause)
}
func (e *sandboxRuntimeInUseError) Unwrap() error { return e.Cause }

// The helper may return only "setup refresh had errors". Inspect bounded,
// recent public helper logs for that attempt, without reading sandbox secrets,
// changing ACLs, stopping other applications, or retrying provisioning blindly.
func diagnoseSandboxSetupFailure(err error, started time.Time) error {
	if err == nil || !strings.Contains(err.Error(), "setup refresh had errors") {
		return nil
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, e := os.UserHomeDir()
		if e != nil {
			return nil
		}
		home = filepath.Join(userHome, ".codex")
	}
	path := sandboxRuntimeLockFromLogs(home, started, time.Now().UTC())
	if path == "" {
		return nil
	}
	return &sandboxRuntimeInUseError{Path: path, Cause: err}
}

func sandboxRuntimeLockFromLogs(home string, started, ended time.Time) string {
	dir := filepath.Join(home, ".sandbox")
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type logFile struct {
		name string
		info os.FileInfo
	}
	var logs []logFile
	for _, entry := range entries {
		name := entry.Name()
		if name != "sandbox.log" && !(strings.HasPrefix(name, "sandbox.") && strings.HasSuffix(name, ".log")) {
			continue
		}
		info, e := entry.Info()
		if e == nil && info.Mode().IsRegular() && !info.ModTime().Before(started.Add(-time.Second)) {
			logs = append(logs, logFile{name, info})
		}
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].info.ModTime().After(logs[j].info.ModTime()) })
	if len(logs) > 2 {
		logs = logs[:2]
	}
	for _, log := range logs {
		f, e := os.Open(filepath.Join(dir, log.name))
		if e != nil {
			continue
		}
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() || !os.SameFile(log.info, info) {
			f.Close()
			continue
		}
		offset := max(int64(0), info.Size()-(64<<10))
		path := sandboxRuntimeLockFromLog(io.NewSectionReader(f, offset, 64<<10), started, ended)
		f.Close()
		if path != "" {
			return path
		}
	}
	return ""
}

func sandboxRuntimeLockFromLog(r io.Reader, started, ended time.Time) string {
	scanner := bufio.NewScanner(r)
	var path string
	for scanner.Scan() {
		line := scanner.Text()
		end := strings.IndexByte(line, ']')
		if !strings.HasPrefix(line, "[") || end < 2 {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line[1:end])
		if err != nil || at.Before(started.Add(-time.Second)) || at.After(ended.Add(time.Second)) {
			continue
		}
		message := strings.TrimSpace(line[end+1:])
		prefix := "runtime read/execute validation failed: validate runtime read/execute access on "
		if !strings.HasPrefix(message, prefix) || !strings.Contains(message, "(os error 32)") {
			continue
		}
		file, _, found := strings.Cut(strings.TrimPrefix(message, prefix), ": open ACL target")
		if !found || len(file) > 4096 || strings.ContainsAny(file, "\r\n\x00") {
			continue
		}
		path = file
	}
	return path
}
