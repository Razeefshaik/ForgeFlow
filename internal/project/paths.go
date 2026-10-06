// Package project locates the ForgeFlow application root, independently of contribution repositories.
package project

import (
	"fmt"
	"os"
	"path/filepath"
)

func isRoot(dir string) bool {
	for _, name := range []string{"SPEC.md", "go.mod"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// Locate prefers an explicit root, then the executable location, then cwd.
// This supports npm/go launches and bin/forgeflow launched from another directory.
func Locate(explicit, cwd, executable string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if !isRoot(root) {
			return "", fmt.Errorf("project root must contain SPEC.md and go.mod")
		}
		return root, nil
	}
	for _, start := range []string{filepath.Dir(executable), cwd} {
		dir, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for {
			if isRoot(dir) {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("ForgeFlow project root not found; run from the project directory or pass --root")
}
func Resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}
