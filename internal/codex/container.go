package codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const containerCreateTimeout = 2 * time.Minute
const containerProbeTimeout = 30 * time.Second
const containerCleanupTimeout = 30 * time.Second

// Include daemon startup/create time and one-time runtime volume
// initialization before the short isolation command runs.
const containerCheckTimeout = 2*containerCreateTimeout + containerProbeTimeout + containerCleanupTimeout

// ContainerRuntimeDir holds container HOME, tool caches and dependency
// environments. It is a per-workspace Docker volume rather than part of the
// host bind mount: on Docker Desktop for Windows, small-file writes through the
// bind mount measured roughly 100x slower (3,000 files: 26s versus 0.24s),
// which made ordinary dependency installation exceed provisioning deadlines.
const ContainerRuntimeDir = "/workspace/.forgeflow-runtime/linux"

var runtimeSubdirectories = []string{"home", "xdg", "go-cache", "go-mod", "go", "npm", "python", "gradle"}

// RuntimeVolumeName derives the one volume owned by a workspace. Different
// workspaces never share a volume, so installed tools cannot leak between
// contributions, and the name is stable so caches survive fresh containers.
func RuntimeVolumeName(dir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dir))))
	return "forgeflow-runtime-" + hex.EncodeToString(sum[:12])
}

// containerArguments exposes exactly one independent clone to an unprivileged
// Linux process. The host environment, home, Docker socket and parent metadata
// are deliberately absent. Docker is an explicit verification backend, never a
// fallback for a failed native command or a prerequisite for serving the UI.
func containerArguments(image, name, dir string, command domain.VerificationCommand, network bool) ([]string, error) {
	if image == "" || strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \t\r\n\x00") {
		return nil, errors.New("verification image must be an explicit local Docker image reference")
	}
	if !filepath.IsAbs(dir) || strings.ContainsAny(dir, ",\"\r\n\x00") {
		return nil, errors.New("container workspace must be an absolute path without mount delimiters")
	}
	for _, p := range []string{dir, filepath.Join(dir, ".git")} {
		info, err := os.Lstat(p)
		real, resolveErr := filepath.EvalSymlinks(p)
		if err != nil || resolveErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Clean(real), filepath.Clean(p)) {
			return nil, errors.New("container workspace and Git metadata must be independent directories")
		}
	}
	netMode := "none"
	if network {
		netMode = "bridge"
	}
	// JVM and Node builds count every thread against the pids limit; 256 tasks
	// was exhausted by ordinary Gradle test workers. 4096 still bounds fork bombs.
	args := []string{"create", "--pull=never", "--name", name, "--label", "forgeflow.verification=true", "--init", "--user", "1000:1000", "--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--pids-limit", "4096", "--memory", "4g", "--cpus", "4", "--network", netMode, "--workdir", "/workspace", "--tmpfs", "/tmp:rw,exec,nosuid,nodev,size=2g,mode=1777", "--mount", "type=bind,source=" + dir + ",target=/workspace", "--mount", "type=bind,source=" + filepath.Join(dir, ".git") + ",target=/workspace/.git,readonly", "--mount", "type=volume,source=" + RuntimeVolumeName(dir) + ",target=" + ContainerRuntimeDir}
	for _, env := range []string{"HOME=/workspace/.forgeflow-runtime/linux/home", "XDG_CACHE_HOME=/workspace/.forgeflow-runtime/linux/xdg", "TMPDIR=/tmp", "GOTMPDIR=/tmp", "GOTOOLCHAIN=local", "GOCACHE=/workspace/.forgeflow-runtime/linux/go-cache", "GOMODCACHE=/workspace/.forgeflow-runtime/linux/go-mod", "GOPATH=/workspace/.forgeflow-runtime/linux/go", "npm_config_cache=/workspace/.forgeflow-runtime/linux/npm", "PIP_CACHE_DIR=/workspace/.forgeflow-runtime/linux/python", "GRADLE_USER_HOME=/workspace/.forgeflow-runtime/linux/gradle", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=safe.directory", "GIT_CONFIG_VALUE_0=/workspace", "GIT_CONFIG_KEY_1=core.checkStat", "GIT_CONFIG_VALUE_1=minimal"} {
		args = append(args, "--env", env)
	}
	// Override any image entrypoint so the recorded command is the actual process.
	args = append(args, "--entrypoint", command.Program, image)
	return append(args, command.Arguments...), nil
}

func prepareLinuxContainerCaches(dir string) error {
	paths := []string{
		filepath.Join(dir, ".forgeflow-runtime"),
		filepath.Join(dir, ".forgeflow-runtime", "linux"),
	}
	for _, sub := range []string{"home", "xdg", "go-cache", "go-mod", "go", "npm", "python", "gradle"} {
		paths = append(paths, filepath.Join(dir, ".forgeflow-runtime", "linux", sub))
	}
	for _, path := range paths {
		if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return errors.New("container runtime cache is not writable")
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("container runtime cache must be an independent directory")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(path)) {
			return errors.New("container runtime cache crosses a link boundary")
		}
	}
	return nil
}

// BootstrapImage is the verification image built from scripts/execution-image.
const BootstrapImage = "forgeflow-verification:go1.26-node22"

// missingImageHint turns Docker's "no such image" into the exact repair. A
// Docker Desktop reset deletes local images, and the configured image is never
// pulled implicitly.
func missingImageHint(image, output string) string {
	text := strings.ToLower(output)
	if !strings.Contains(text, "no such image") && !strings.Contains(text, "unable to find image") && !strings.Contains(text, "not cached") {
		return ""
	}
	return fmt.Sprintf("The verification image %s is not available in Docker (a Docker Desktop reset or image cleanup removes local images). Rebuild the bootstrap image from the ForgeFlow root with `docker build --tag %s scripts/execution-image`, set \"verification_image\": %q in configs/local.json, restart ForgeFlow and recover the contribution.", image, BootstrapImage, BootstrapImage)
}

// ensureRuntimeVolume creates the workspace's runtime volume and gives UID 1000
// its top-level directories. A fresh volume is root-owned, so this one short
// container runs as root with only CAP_CHOWN, offline, with a read-only root
// and no workspace mount. Initialization is idempotent and non-recursive, so
// existing caches are never rewritten.
func (c *CLI) ensureRuntimeVolume(ctx context.Context, image, dir string) error {
	volume := RuntimeVolumeName(dir)
	c.environmentMu.Lock()
	ready := c.volumes[volume+"\x00"+image]
	c.environmentMu.Unlock()
	if ready {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	createCtx, stop := context.WithTimeout(context.Background(), containerCreateTimeout)
	out, err := dockerControl(createCtx, "volume", "create", "--label", "forgeflow.runtime=true", volume)
	stop()
	if err != nil {
		return fmt.Errorf("container creation failed: runtime volume %s: %w; %s", volume, err, out)
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	name := "forgeflow-volume-init-" + hex.EncodeToString(nonce[:])
	script := "set -eu\n"
	owned := []string{"/runtime"}
	for _, sub := range runtimeSubdirectories {
		script += "mkdir -p /runtime/" + sub + "\n"
		owned = append(owned, "/runtime/"+sub)
	}
	script += "chown 1000:1000 " + strings.Join(owned, " ") + "\n"
	args := []string{"run", "--rm", "--pull=never", "--name", name, "--label", "forgeflow.verification=true", "--user", "0:0", "--read-only", "--cap-drop=ALL", "--cap-add=CHOWN", "--security-opt", "no-new-privileges", "--network", "none", "--pids-limit", "64", "--mount", "type=volume,source=" + volume + ",target=/runtime", "--entrypoint", "sh", image, "-c", script}
	// As with verification containers, wait for the Docker client rather than
	// abandoning a daemon-owned container mid-creation; then remove that exact name.
	runCtx, cancel := context.WithTimeout(context.Background(), containerCreateTimeout)
	out, err = dockerControl(runCtx, args...)
	cancel()
	if err != nil {
		cleanupCtx, done := context.WithTimeout(context.Background(), containerCleanupTimeout)
		cleanup, cleanupErr := dockerControl(cleanupCtx, "rm", "--force", name)
		done()
		err = fmt.Errorf("container creation failed: runtime volume %s could not be initialized: %w; %s", volume, err, out)
		if hint := missingImageHint(image, out); hint != "" {
			err = fmt.Errorf("%w\n%s", err, hint)
		}
		if cleanupErr != nil && !strings.Contains(cleanup, "No such container") {
			err = errors.Join(err, fmt.Errorf("container cleanup failed for %s: %s", name, cleanup))
		}
		return err
	}
	c.environmentMu.Lock()
	if c.volumes == nil {
		c.volumes = map[string]bool{}
	}
	c.volumes[volume+"\x00"+image] = true
	c.environmentMu.Unlock()
	return nil
}

// RemoveRuntimeVolume deletes exactly the volume owned by dir. It is used for
// disposable probe workspaces and abandoned contributions; it never prunes.
func (c *CLI) RemoveRuntimeVolume(ctx context.Context, dir string) error {
	volume := RuntimeVolumeName(dir)
	out, err := dockerControl(ctx, "volume", "rm", "--force", volume)
	c.environmentMu.Lock()
	for key := range c.volumes {
		if strings.HasPrefix(key, volume+"\x00") {
			delete(c.volumes, key)
		}
	}
	c.environmentMu.Unlock()
	if err != nil && !strings.Contains(strings.ToLower(out), "no such volume") {
		return fmt.Errorf("remove runtime volume %s: %w; %s", volume, err, out)
	}
	return nil
}

func dockerControl(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = environment()
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	out := &bounded{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return out.String(), err
}

func (c *CLI) containerCommand(ctx context.Context, dir string, command domain.VerificationCommand, network bool) (rec domain.TestRun, err error) {
	return c.containerCommandImage(ctx, c.workspaceImage(dir), dir, command, network)
}

func (c *CLI) containerCommandImage(ctx context.Context, image, dir string, command domain.VerificationCommand, network bool) (rec domain.TestRun, err error) {
	return c.containerCommandMode(ctx, image, dir, command, network, false)
}

func (c *CLI) containerCommandMode(ctx context.Context, image, dir string, command domain.VerificationCommand, network, prepareHooks bool) (rec domain.TestRun, err error) {
	rec = domain.TestRun{Command: command, ExecutedCommand: &command, Directory: dir, StartedAt: time.Now().UTC(), ExitCode: -1}
	defer func() { rec.FinishedAt = time.Now().UTC() }()
	if err = ctx.Err(); err != nil {
		return rec, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return rec, err
	}
	name := "forgeflow-verify-" + hex.EncodeToString(nonce[:])
	args, err := containerArguments(image, name, dir, command, network)
	if err != nil {
		return rec, err
	}
	if err = prepareLinuxContainerCaches(dir); err != nil {
		return rec, err
	}
	if err = c.ensureRuntimeVolume(ctx, image, dir); err != nil {
		rec.Output = err.Error()
		return rec, err
	}
	if prepareHooks {
		args, err = hookPreparationArguments(args, dir)
		if err != nil {
			return rec, err
		}
	}
	// Observe create completion before honoring cancellation; killing the Docker
	// client during creation can orphan a daemon-owned container. Every subsequent
	// exit path removes this exact randomly named container, including descendants.
	createCtx, stop := context.WithTimeout(context.Background(), containerCreateTimeout)
	out, createErr := dockerControl(createCtx, args...)
	stop()
	if createErr != nil {
		rec.Output = out
		err = fmt.Errorf("container creation failed: %w; %s", createErr, out)
		if hint := missingImageHint(image, out); hint != "" {
			err = fmt.Errorf("%w\n%s", err, hint)
			rec.Output += "\n" + hint
		}
		// A timed-out create may have reached the daemon; use the reserved name.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
		defer cancel()
		cleanup, cleanupErr := dockerControl(cleanupCtx, "rm", "--force", "--volumes", name)
		if cleanupErr != nil && !strings.Contains(cleanup, "No such container") {
			err = errors.Join(err, fmt.Errorf("container cleanup failed for %s: %s", name, cleanup))
		}
		return rec, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
		defer cancel()
		output, cleanupErr := dockerControl(cleanupCtx, "rm", "--force", "--volumes", name)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("container cleanup failed for %s: %s", name, output))
			rec.Output += "\n" + err.Error()
		}
	}()
	rec.Output = fmt.Sprintf("Docker container %s; image %s; working directory /workspace\n", name, image)
	if err = ctx.Err(); err != nil {
		return rec, err
	}
	cmd := exec.CommandContext(ctx, "docker", "start", "--attach", name)
	cmd.Env = environment()
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	output := &bounded{limit: 256 << 10}
	cmd.Stdout, cmd.Stderr = output, output
	err = cmd.Run()
	if cmd.ProcessState != nil {
		rec.ExitCode = cmd.ProcessState.ExitCode()
	}
	rec.Output += output.String()
	rec.Truncated = output.truncated
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return rec, err
}

func (c *CLI) checkContainer(ctx context.Context, dir string) error {
	// Real positive and negative writes. EACCES/EROFS are required: missing paths
	// or a broken interpreter must never count as successful isolation.
	probe := `set -eu
export LC_ALL=C
for cmd in id touch rm grep; do command -v "$cmd" >/dev/null; done
test "$(id -u)" = 1000
test -d /workspace/.git
test ! -e /workspace/.forgeflow-container-probe
(set -C; echo allowed > /workspace/.forgeflow-container-probe)
rm /workspace/.forgeflow-container-probe
for p in /outside-probe /workspace/.git/forgeflow-probe; do
 if touch "$p" 2>/tmp/forgeflow-denial; then exit 1; fi
 grep -Eq 'Permission denied|Read-only file system' /tmp/forgeflow-denial
done
test ! -e /var/run/docker.sock
for p in /sys/class/net/*; do test "$p" = /sys/class/net/lo; done
echo 'container workspace write, root/git denial, non-root identity and absent Docker socket: PASS'`
	rec, err := c.containerCommand(ctx, dir, domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", probe}}, false)
	if err != nil || rec.ExitCode != 0 {
		return fmt.Errorf("container isolation check failed: %v; %s", err, rec.Output)
	}
	return nil
}
