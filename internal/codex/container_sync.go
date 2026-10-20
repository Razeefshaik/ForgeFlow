package codex

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Verification ran directly on the Windows bind mount of the workspace. A
// clean osh-core compile took 2 min 15 s in a Docker volume and more than
// 10 minutes on the bind mount, and dependency installs were ~100x slower.
// Commands therefore run on a per-workspace copy in a Docker volume. Before
// each command ForgeFlow streams only the host files that changed (tracked and
// new files, Git metadata and host-written evidence) into the copy and removes
// files the contributor deleted. Build outputs created in the copy (build/,
// node_modules/) stay there, so builds remain incremental, and nothing a
// command writes can reach the host workspace.

const syncTimeout = 10 * time.Minute

// SourceVolumeName is the copy of one workspace. Like the runtime volume it is
// never shared between workspaces.
func SourceVolumeName(dir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dir))))
	return "forgeflow-source-" + hex.EncodeToString(sum[:12])
}

type syncEntry struct {
	size, modified int64
}

type workspaceSync struct {
	mu       sync.Mutex
	manifest map[string]syncEntry
}

// Sync state is per process and per workspace: a fresh process performs a
// complete sync, after which only changed files are sent.
var syncs sync.Map

func syncerFor(volume string) *workspaceSync {
	v, _ := syncs.LoadOrStore(volume, &workspaceSync{})
	return v.(*workspaceSync)
}

// hostSourceSet lists the files Git considers part of the working tree
// (tracked and untracked-but-not-ignored), relative and slash-separated.
func hostSourceSet(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-c", "core.fsmonitor=false", "-c", "core.quotepath=off", "ls-files", "-z", "-co", "--exclude-standard")
	cmd.Dir = dir
	cmd.Env = append(environment(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_OPTIONAL_LOCKS=0")
	configureProcess(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("list workspace files for the verification copy: %w; %s", err, strings.TrimSpace(errOut.String()))
	}
	files := []string{}
	seen := map[string]bool{}
	for _, name := range strings.Split(out.String(), "\x00") {
		if name == "" || seen[name] || strings.ContainsAny(name, "\n\r") || name == ".forgeflow-runtime" || strings.HasPrefix(name, ".forgeflow-runtime/") {
			continue
		}
		seen[name] = true
		files = append(files, name)
	}
	return files, nil
}

// syncFiles adds Git metadata and host-written evidence (top-level files and
// evidence/ in .forgeflow-runtime, such as completion receipts) to the source
// set. Host tool caches in .forgeflow-runtime stay on the host.
func syncFiles(dir string, source []string) ([]string, []string, error) {
	files := append([]string{}, source...)
	// Git requires its directory skeleton (objects/, refs/tags/ ...) even when
	// those directories are empty, so they are sent explicitly.
	gitDirs := []string{}
	err := filepath.WalkDir(filepath.Join(dir, ".git"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel, e := filepath.Rel(dir, p); e == nil {
				gitDirs = append(gitDirs, filepath.ToSlash(rel))
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, e := filepath.Rel(dir, p)
			if e != nil {
				return e
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	runtimeDir := filepath.Join(dir, ".forgeflow-runtime")
	if entries, e := os.ReadDir(runtimeDir); e == nil {
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				files = append(files, ".forgeflow-runtime/"+entry.Name())
			}
		}
		_ = filepath.WalkDir(filepath.Join(runtimeDir, "evidence"), func(p string, d os.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				if rel, e := filepath.Rel(dir, p); e == nil {
					files = append(files, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	return files, gitDirs, nil
}

// syncScript extracts the changed files, then deletes working-tree files the
// host no longer has. Ignored files (build outputs, dependencies) are kept.
const syncScript = `set -eu
cd /workspace
tar -x -f - --no-same-owner
if [ -f .forgeflow-sync/keep ]; then
  git -c safe.directory=/workspace -c core.fsmonitor=false -c core.quotepath=off ls-files -co --exclude-standard | grep -Fxv -f .forgeflow-sync/keep | while IFS= read -r f; do rm -f -- "$f"; done
fi
rm -rf .forgeflow-sync
`

// syncWorkspace brings the workspace's source volume up to date with dir.
func (c *CLI) syncWorkspace(ctx context.Context, dir string) error {
	volume := SourceVolumeName(dir)
	image := c.syncImage()
	if err := c.ensureVolume(ctx, image, volume, nil); err != nil {
		return err
	}
	state := syncerFor(volume)
	state.mu.Lock()
	defer state.mu.Unlock()
	source, err := hostSourceSet(ctx, dir)
	if err != nil {
		return err
	}
	files, gitDirs, err := syncFiles(dir, source)
	if err != nil {
		return fmt.Errorf("list Git metadata for the verification copy: %w", err)
	}
	next := map[string]syncEntry{}
	changed := []string{}
	for _, name := range files {
		info, e := os.Lstat(filepath.Join(dir, filepath.FromSlash(name)))
		if e != nil || !info.Mode().IsRegular() {
			continue // deleted since listing, or a link/device: never followed
		}
		entry := syncEntry{size: info.Size(), modified: info.ModTime().UnixNano()}
		next[name] = entry
		if old, ok := state.manifest[name]; !ok || old != entry {
			changed = append(changed, name)
		}
	}
	keep := strings.Join(append(source, ".forgeflow-sync/keep"), "\n") + "\n"
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(writeSyncArchive(writer, dir, gitDirs, changed, keep))
	}()
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	name := "forgeflow-sync-" + hex.EncodeToString(nonce[:])
	runCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "docker", "run", "--rm", "-i", "--pull=never", "--name", name, "--label", "forgeflow.verification=true", "--network", "none", "--user", "1000:1000", "--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--tmpfs", "/tmp:rw,nosuid,nodev,size=256m", "--mount", "type=volume,source="+volume+",target=/workspace", "--entrypoint", "sh", image, "-c", syncScript)
	cmd.Env, cmd.Stdin, cmd.WaitDelay = environment(), reader, 3*time.Second
	configureProcess(cmd)
	out := &bounded{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = out, out
	err = cmd.Run()
	reader.Close()
	if err != nil {
		cleanupCtx, done := context.WithTimeout(context.Background(), containerCleanupTimeout)
		_, _ = dockerControl(cleanupCtx, "rm", "--force", name)
		done()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fmt.Errorf("container creation failed: could not sync the workspace into its verification copy: %w; %s", err, out.String())
		if hint := missingImageHint(image, out.String()); hint != "" {
			err = fmt.Errorf("%w\n%s", err, hint)
		}
		// The copy may be partially updated; the next sync sends everything.
		state.manifest = nil
		return err
	}
	state.manifest = next
	return nil
}

func writeSyncArchive(w io.Writer, dir string, gitDirs, changed []string, keep string) error {
	tw := tar.NewWriter(w)
	dirs := map[string]bool{}
	for _, d := range gitDirs {
		dirs[d] = true
		if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0755, ModTime: time.Now()}); err != nil {
			return err
		}
	}
	addDirs := func(name string) error {
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			d := strings.Join(parts[:i], "/")
			if dirs[d] {
				continue
			}
			dirs[d] = true
			if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0755, ModTime: time.Now()}); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range changed {
		path := filepath.Join(dir, filepath.FromSlash(name))
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			f.Close()
			continue
		}
		if err = addDirs(name); err != nil {
			f.Close()
			return err
		}
		// Windows has no executable bit; the bind mount exposed every file as
		// executable, which wrapper scripts such as ./gradlew rely on.
		if err = tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0755, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
			f.Close()
			return err
		}
		n, err := io.CopyN(tw, f, info.Size())
		f.Close()
		if err != nil || n != info.Size() {
			return fmt.Errorf("%s changed while it was copied for verification", name)
		}
	}
	if err := addDirs(".forgeflow-sync/keep"); err != nil {
		return err
	}
	// Create the runtime directory as UID 1000 before Docker creates the nested
	// runtime-volume mount point there as root; recipes write files beside it.
	for _, d := range []string{".forgeflow-runtime", ".forgeflow-runtime/linux"} {
		if !dirs[d] {
			dirs[d] = true
			if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0755, ModTime: time.Now()}); err != nil {
				return err
			}
		}
	}
	if err := tw.WriteHeader(&tar.Header{Name: ".forgeflow-sync/keep", Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(keep)), ModTime: time.Now()}); err != nil {
		return err
	}
	if _, err := io.WriteString(tw, keep); err != nil {
		return err
	}
	return tw.Close()
}

// syncImage runs the copy step. The bootstrap verification image supplies
// sh, tar, grep and Git; repository images need not.
func (c *CLI) syncImage() string {
	if c.SyncImage != "" {
		return c.SyncImage
	}
	return c.VerificationImage
}
