package codex

import (
	"context"
	"crypto/rand"
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
	args := []string{"create", "--pull=never", "--name", name, "--label", "forgeflow.verification=true", "--init", "--user", "1000:1000", "--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--memory", "2g", "--cpus", "2", "--network", netMode, "--workdir", "/workspace", "--tmpfs", "/tmp:rw,exec,nosuid,nodev,size=1g,mode=1777", "--mount", "type=bind,source=" + dir + ",target=/workspace", "--mount", "type=bind,source=" + filepath.Join(dir, ".git") + ",target=/workspace/.git,readonly"}
	for _, env := range []string{"HOME=/tmp", "TMPDIR=/tmp", "GOTMPDIR=/tmp", "GOTOOLCHAIN=local", "GOCACHE=/workspace/.forgeflow-runtime/linux/go-cache", "GOMODCACHE=/workspace/.forgeflow-runtime/linux/go-mod", "GOPATH=/workspace/.forgeflow-runtime/linux/go", "npm_config_cache=/workspace/.forgeflow-runtime/linux/npm", "PIP_CACHE_DIR=/workspace/.forgeflow-runtime/linux/python", "GRADLE_USER_HOME=/workspace/.forgeflow-runtime/linux/gradle", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=safe.directory", "GIT_CONFIG_VALUE_0=/workspace"} {
		args = append(args, "--env", env)
	}
	// Override any image entrypoint so the recorded command is the actual process.
	args = append(args, "--entrypoint", command.Program, image)
	return append(args, command.Arguments...), nil
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
	// Observe create completion before honoring cancellation; killing the Docker
	// client during creation can orphan a daemon-owned container. Every subsequent
	// exit path removes this exact randomly named container, including descendants.
	createCtx, stop := context.WithTimeout(context.Background(), 2*time.Minute)
	out, createErr := dockerControl(createCtx, args...)
	stop()
	if createErr != nil {
		rec.Output = out
		err = fmt.Errorf("container creation failed: %w; %s", createErr, out)
		// A timed-out create may have reached the daemon; use the reserved name.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanup, cleanupErr := dockerControl(cleanupCtx, "rm", "--force", "--volumes", name)
		if cleanupErr != nil && !strings.Contains(cleanup, "No such container") {
			err = errors.Join(err, fmt.Errorf("container cleanup failed for %s: %s", name, cleanup))
		}
		return rec, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
