package execution

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"forgeflow/internal/codex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type gitOutput struct {
	sync.Mutex
	b         strings.Builder
	truncated bool
}

func (o *gitOutput) Write(p []byte) (int, error) {
	o.Lock()
	defer o.Unlock()
	n := len(p)
	left := (4 << 20) - o.b.Len()
	if len(p) > left {
		p = p[:left]
		o.truncated = true
	}
	o.b.Write(p)
	return n, nil
}
func (s *Service) git(ctx context.Context, repo string, args ...string) (string, error) {
	if err := verifyGitConfig(repo); err != nil {
		return "", err
	}
	prefix := []string{"-c", "core.longpaths=true", "-c", "core.hooksPath=" + filepath.Join(filepath.Dir(repo), ".autopilot", "empty-hooks"), "-c", "credential.helper=", "-c", "core.fsmonitor=false", "-c", "core.pager=cat", "-c", "diff.external=", "-c", "submodule.recurse=false"}
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	cmd.Dir = repo
	cmd.Env = gitEnv()
	cmd.WaitDelay = 3 * time.Second
	codex.ConfigureProcess(cmd)
	out := &gitOutput{}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if out.truncated {
		return "", errors.New("Git output exceeds four MiB limit; contribution is too broad")
	}
	var exit *exec.ExitError
	if err != nil && !(len(args) > 0 && args[0] == "diff" && contains(args, "--no-index") && errors.As(err, &exit) && exit.ExitCode() == 1) {
		return "", fmt.Errorf("Git operation failed: %s", out.b.String())
	}
	return out.b.String(), nil
}
func gitEnv() []string {
	env := []string{}
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || upper == "GH_TOKEN" || upper == "GITHUB_TOKEN" {
			continue
		}
		env = append(env, e)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}

func (s *Service) verifyApprovedGit(ctx context.Context, repo, base, approvedBranch string) error {
	head, err := s.git(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("could not verify approved Git HEAD: %w", err)
	}
	if strings.TrimSpace(head) != base {
		return errors.New("contributor changed Git history; inspect workspace before continuing")
	}
	branch, err := s.git(ctx, repo, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("could not verify approved Git branch: %w", err)
	}
	if strings.TrimSpace(branch) != approvedBranch {
		return errors.New("contributor left its approved branch")
	}
	return nil
}
func (s *Service) diff(ctx context.Context, repo, base string) (string, []string, error) {
	tracked, err := s.git(ctx, repo, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--diff-filter=a", base, "--")
	if err != nil {
		return "", nil, err
	}
	names, err := s.git(ctx, repo, "diff", "--name-only", "-z", "--no-renames", "--diff-filter=a", base, "--")
	if err != nil {
		return "", nil, err
	}
	files := []string{}
	for _, name := range strings.Split(names, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	// Files added since base exist only after PR preparation commits them.
	// Rendering them exactly like untracked files keeps the reviewed patch,
	// including binary and oversized notes, identical across that commit.
	added, err := s.git(ctx, repo, "diff", "--name-only", "-z", "--no-renames", "--diff-filter=A", base, "--")
	if err != nil {
		return "", nil, err
	}
	untracked, err := s.git(ctx, repo, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", nil, err
	}
	var full strings.Builder
	full.WriteString(tracked)
	for _, name := range strings.Split(added+untracked, "\x00") {
		if name == "" {
			continue
		}
		path := filepath.Join(repo, filepath.FromSlash(name))
		rel, e := filepath.Rel(repo, path)
		if e != nil || strings.HasPrefix(rel, "..") {
			return "", nil, errors.New("untracked path outside repository")
		}
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() {
			return "", nil, errors.New("untracked file is linked or not a regular file; inspect contribution")
		}
		var patch string
		if info.Size() > 256<<10 {
			patch, e = untrackedNote(path, name, info)
		} else {
			var b []byte
			if b, e = os.ReadFile(path); e != nil {
				return "", nil, e
			}
			if bytes.IndexByte(b, 0) >= 0 {
				patch, e = untrackedNote(path, name, info)
			} else {
				patch, e = s.git(ctx, repo, "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--", os.DevNull, name)
			}
		}
		if e != nil {
			return "", nil, e
		}
		full.WriteString(patch)
		files = append(files, name)
		if full.Len() > 4<<20 {
			return "", nil, errors.New("diff exceeds four MiB bound")
		}
	}
	// Canonical section order is stable before and after untracked files are committed.
	sections := strings.Split(full.String(), "diff --git ")
	if len(sections) > 1 {
		sort.Strings(sections[1:])
		sort.Strings(files)
		return sections[0] + "diff --git " + strings.Join(sections[1:], "diff --git "), files, nil
	}
	return full.String(), files, nil
}

// untrackedNote lists a binary or oversized new file without inlining it,
// using Git's new-file header. The full blob ID binds the patch hash, and so
// review and PR preparation, to the exact bytes on disk.
func untrackedNote(path, name string, info os.FileInfo) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum, nul := sha1.New(), &nulDetector{}
	fmt.Fprintf(sum, "blob %d\x00", info.Size())
	n, err := io.Copy(io.MultiWriter(sum, nul), f)
	if err != nil {
		return "", err
	}
	if n != info.Size() {
		return "", errors.New("untracked file changed while it was inspected; inspect contribution")
	}
	mode := "100644"
	if info.Mode()&0111 != 0 {
		mode = "100755"
	}
	note := fmt.Sprintf("Contents omitted, %d bytes exceed the 256 KiB inline diff limit\n", n)
	if nul.found {
		note = fmt.Sprintf("Binary files /dev/null and %s differ\n", gitPath("b/", name))
	}
	return fmt.Sprintf("diff --git %s %s\nnew file mode %s\nindex %s..%x\n%s", gitPath("a/", name), gitPath("b/", name), mode, strings.Repeat("0", 40), sum.Sum(nil), note), nil
}

type nulDetector struct{ found bool }

func (d *nulDetector) Write(p []byte) (int, error) {
	d.found = d.found || bytes.IndexByte(p, 0) >= 0
	return len(p), nil
}

// gitPath C-quotes a header path containing control, quote, backslash or
// non-ASCII bytes, as Git does, so every note header stays one line.
func gitPath(prefix, name string) string {
	path := prefix + name
	if !strings.ContainsFunc(path, func(r rune) bool { return r < 0x20 || r == '"' || r == '\\' || r >= 0x7f }) {
		return path
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(path); i++ {
		switch c := path[i]; {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, "\\%03o", c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func contains(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
