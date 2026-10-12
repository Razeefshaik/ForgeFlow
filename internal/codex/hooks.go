package codex

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Hook files and the hook manager's checksum belong to preparation only.
// Config, index, refs and objects retain the read-only parent mount. Installers
// run as UID 1000 without host credentials, privileges or a Docker socket.
func hookPreparationArguments(args []string, dir string) ([]string, error) {
	hooks := filepath.Join(dir, ".git", "hooks")
	if err := os.Mkdir(hooks, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err := validateHookDirectory(hooks); err != nil {
		return nil, err
	}
	checksum, err := prepareHookChecksum(dir)
	if err != nil {
		return nil, err
	}
	for i, arg := range args {
		if arg == "--entrypoint" {
			result := append([]string(nil), args[:i]...)
			result = append(result, "--mount", "type=bind,source="+hooks+",target=/workspace/.git/hooks")
			result = append(result, "--mount", "type=bind,source="+checksum+",target=/workspace/.git/info/lefthook.checksum")
			return append(result, args[i:]...), nil
		}
	}
	return nil, errors.New("hook preparation requires an explicit container entrypoint")
}

func prepareHookChecksum(dir string) (string, error) {
	info := filepath.Join(dir, ".git", "info")
	if err := os.Mkdir(info, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	real, err := filepath.EvalSymlinks(info)
	if err != nil || !strings.EqualFold(filepath.Clean(real), filepath.Clean(info)) {
		return "", errors.New("Git info directory crosses a link boundary")
	}
	name := filepath.Join(info, "lefthook.checksum")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		err = f.Close()
	} else if errors.Is(err, os.ErrExist) {
		err = nil
	}
	if err != nil {
		return "", err
	}
	stat, err := os.Lstat(name)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 64<<10 {
		return "", errors.New("hook checksum must be a small regular file")
	}
	return name, nil
}

func validateHookDirectory(hooks string) error {
	real, err := filepath.EvalSymlinks(hooks)
	if err != nil || !strings.EqualFold(filepath.Clean(real), filepath.Clean(hooks)) {
		return errors.New("Git hooks directory crosses a link boundary")
	}
	count, size := 0, int64(0)
	return filepath.WalkDir(hooks, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 256 || d.Type()&os.ModeSymlink != 0 {
			return errors.New("Git hooks contain links or exceed inspection bounds")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("Git hook is not a regular file")
		}
		size += info.Size()
		if size > 4<<20 {
			return errors.New("Git hooks exceed four MiB")
		}
		return nil
	})
}
