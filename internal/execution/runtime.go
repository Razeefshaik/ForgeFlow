package execution

import (
	"errors"
	"forgeflow/internal/codex"
	"os"
	"path/filepath"
	"strings"
)

// generatedCachePatterns are interpreter, test-runner and build caches that a
// contributor or verification command can write while running checks. They
// are never part of a contribution; an unignored bytecode cache once stopped a
// finished patch at the binary-file guard.
var generatedCachePatterns = []string{"__pycache__/", "*.py[cod]", ".pytest_cache/", ".mypy_cache/", ".ruff_cache/", ".tox/", ".nox/", ".hypothesis/", "*.egg-info/", ".coverage", ".gradle/", "node_modules/"}

func (s *Service) prepareRuntime(repo string) error {
	if err := codex.PrepareWorkspaceRuntime(repo); err != nil {
		return err
	}
	if err := validateGitTree(repo); err != nil {
		return err
	}
	git, err := os.Lstat(filepath.Join(repo, ".git"))
	if err != nil || !git.IsDir() || git.Mode()&os.ModeSymlink != 0 {
		return errors.New("workspace Git metadata must be an independent clone")
	}
	info := filepath.Join(repo, ".git", "info")
	if err = os.MkdirAll(info, 0700); err != nil {
		return err
	}
	exclude := filepath.Join(info, "exclude")
	b, err := os.ReadFile(exclude)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Append only missing lines, so existing workspaces gain new patterns on
	// their next run and repeated runs leave the file unchanged.
	present := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		present[strings.TrimSpace(line)] = true
	}
	missing := []string{}
	for _, pattern := range append([]string{"/.forgeflow-runtime/"}, generatedCachePatterns...) {
		if !present[pattern] {
			missing = append(missing, pattern)
		}
	}
	if len(missing) > 0 {
		f, e := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = f.WriteString("\n" + strings.Join(missing, "\n") + "\n")
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return pinGitConfig(repo)
}
