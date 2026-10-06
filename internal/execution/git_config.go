package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func configDigest(repo string) (string, error) {
	if err := validateGitTree(repo); err != nil {
		return "", err
	}
	path := filepath.Join(repo, ".git", "config")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return "", errors.New("workspace Git configuration is linked, missing or oversized")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func validateGitTree(repo string) error {
	path := filepath.Join(repo, ".git")
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !strings.EqualFold(filepath.Clean(path), filepath.Clean(resolved)) {
		return errors.New("Git metadata crosses a link boundary")
	}
	count := 0
	return filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 50000 {
			return errors.New("Git metadata exceeds inspection bound")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("linked Git metadata is not allowed for host operations")
		}
		return nil
	})
}
func verifyGitConfig(repo string) error {
	expected, err := os.ReadFile(filepath.Join(filepath.Dir(repo), ".autopilot", "git-config.sha256"))
	if err != nil {
		return errors.New("trusted Git configuration snapshot is missing")
	}
	actual, err := configDigest(repo)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(expected)) != actual {
		return errors.New("workspace Git configuration changed after approval; Git operations refused")
	}
	return nil
}
func pinGitConfig(repo string) error {
	snapshot := filepath.Join(filepath.Dir(repo), ".autopilot", "git-config.sha256")
	if _, err := os.Stat(snapshot); err == nil {
		return verifyGitConfig(repo)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	digest, err := configDigest(repo)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "core.fsmonitor=false", "config", "--local", "--no-includes", "--null", "--list")
	cmd.Dir = repo
	cmd.Env = gitEnv()
	b, err := cmd.Output()
	if err != nil {
		return errors.New("could not validate workspace Git configuration")
	}
	allowed := map[string]bool{"core.repositoryformatversion": true, "core.filemode": true, "core.bare": true, "core.logallrefupdates": true, "core.symlinks": true, "core.ignorecase": true, "core.precomposeunicode": true, "remote.origin.url": true, "remote.origin.fetch": true}
	for _, entry := range strings.Split(string(b), "\x00") {
		if entry == "" {
			continue
		}
		key, value, _ := strings.Cut(entry, "\n")
		if !allowed[key] && !(strings.HasPrefix(key, "branch.autopilot/") && (strings.HasSuffix(key, ".remote") || strings.HasSuffix(key, ".merge"))) {
			return errors.New("unsafe local Git configuration; inspect before execution")
		}
		if key == "remote.origin.url" && !strings.HasPrefix(value, "https://github.com/") {
			return errors.New("workspace remote must be the approved public GitHub HTTPS repository")
		}
	}
	// The agent cannot write this metadata directory. Never refresh a pinned digest.
	f, err := os.OpenFile(snapshot, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(digest + "\n")
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
