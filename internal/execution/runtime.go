package execution

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) prepareRuntime(repo string) error {
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
	if !strings.Contains(string(b), "/.forgeflow-runtime/") {
		f, e := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = f.WriteString("\n/.forgeflow-runtime/\n")
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
