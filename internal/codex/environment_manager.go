package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type EnvironmentCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence,omitempty"`
}
type EnvironmentStatus struct {
	Available     bool               `json:"available"`
	Platform      string             `json:"platform"`
	Mode          string             `json:"mode"`
	RequestedMode string             `json:"requested_mode,omitempty"`
	Status        string             `json:"status"`
	Message       string             `json:"message"`
	Running       bool               `json:"running"`
	Checks        []EnvironmentCheck `json:"checks"`
	Error         string             `json:"error,omitempty"`
	CheckedAt     *time.Time         `json:"checked_at,omitempty"`
}

// EnvironmentManager manages host setup, never arbitrary repository commands.
// It checks candidate policies on a separate CLI and adopts only verified modes.
type EnvironmentManager struct {
	cli        *CLI
	root, base string
	ctx        context.Context
	mu         sync.Mutex
	state      EnvironmentStatus
	wg         sync.WaitGroup
	audit      func(context.Context, string, string, any) error
	setup      func(context.Context, *CLI) (string, error)
	probe      func(context.Context, *CLI) ([]EnvironmentCheck, error)
}

func NewEnvironmentManager(ctx context.Context, cli *CLI, root, base string, audit func(context.Context, string, string, any) error) (*EnvironmentManager, error) {
	if audit==nil { return nil,errors.New("environment audit sink is required") }
	m := &EnvironmentManager{cli: cli, root: root, base: base, ctx: ctx, audit: audit}
	m.state = EnvironmentStatus{Available: cli.Binary != "", Platform: runtime.GOOS, Mode: cli.SandboxMode(), Status: "unverified", Message: "Check workspace isolation and Git shell compatibility before execution.", Checks: []EnvironmentCheck{}}
	b, err := os.ReadFile(m.settingsPath())
	if err == nil {
		var saved struct {
			Mode string `json:"mode"`
		}
		if json.Unmarshal(b, &saved) != nil || !ValidSandboxMode(saved.Mode) {
			return nil, errors.New("invalid configs/sandbox.json")
		}
		// An explicit environment/local config selection takes precedence.
		if cli.sandboxMode == "" {
			_ = cli.SetSandboxMode(saved.Mode)
			m.state.Mode = saved.Mode
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	m.setup = setupWindowsSandbox
	m.probe = m.probeEnvironment
	return m, nil
}
func (m *EnvironmentManager) settingsPath() string {
	return filepath.Join(m.root, "configs", "sandbox.json")
}
func (m *EnvironmentManager) Status() EnvironmentStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state
	s.Checks = append([]EnvironmentCheck{}, s.Checks...)
	return s
}
func (m *EnvironmentManager) Wait() { m.wg.Wait() }

func (m *EnvironmentManager) Start(action, mode string, approved bool) (EnvironmentStatus, error) {
	if action != "check" && action != "setup" && action != "select" {
		return m.Status(), errors.New("unknown environment action")
	}
	if action != "check" && !approved {
		return m.Status(), errors.New("explicit approval is required for sandbox setup or selection")
	}
	if action == "setup" {
		mode = "elevated"
	}
	if mode == "" {
		mode = m.cli.SandboxMode()
	}
	if !ValidSandboxMode(mode) {
		return m.Status(), errors.New("invalid sandbox mode")
	}
	if action == "setup" && runtime.GOOS != "windows" {
		return m.Status(), errors.New("elevated setup is available only on Windows")
	}
	// Checking an unprovisioned legacy sandbox may itself invoke setup. That
	// requires the setup/selection approval, even though probes are read-only.
	if action == "check" && mode != m.cli.SandboxMode() { return m.Status(),errors.New("use verified selection to check a different sandbox mode") }
	if action == "check" && mode == "elevated" && !approved {
		return m.Status(), errors.New("approval is required to check an elevated or different sandbox")
	}
	m.mu.Lock()
	if m.state.Running {
		m.mu.Unlock()
		return m.Status(), errors.New("environment operation is already running")
	}
	if !m.state.Available || m.ctx.Err() != nil {
		m.mu.Unlock()
		return m.Status(), errors.New("Codex runtime is unavailable or stopping")
	}
	m.state.Running, m.state.RequestedMode, m.state.Status = true, mode, "checking"
	m.state.Error, m.state.Checks, m.state.CheckedAt = "", []EnvironmentCheck{}, nil
	m.state.Message = "Checking the selected execution environment."
	if action == "setup" {
		m.state.Status, m.state.Message = "awaiting_windows_approval", "Approve the Windows elevation prompt to provision the Codex sandbox."
	}
	initial := m.state
	if err := m.audit(m.ctx, "EnvironmentOperationStarted", initial.Message, initial); err != nil {
		m.state.Running = false
		m.mu.Unlock()
		return m.Status(), err
	}
	m.wg.Add(1)
	m.mu.Unlock()
	go m.run(action, mode)
	return initial, nil
}

func (m *EnvironmentManager) publish(status, message string) error {
	m.mu.Lock()
	m.state.Status, m.state.Message = status, message
	snapshot := m.state
	m.mu.Unlock()
	return m.audit(m.ctx, "EnvironmentOperationUpdated", message, snapshot)
}
func (m *EnvironmentManager) run(action, mode string) {
	defer m.wg.Done()
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Minute)
	defer cancel()
	candidate := &CLI{Binary: m.cli.Binary, Model: m.cli.Model}
	_ = candidate.SetSandboxMode(mode)
	var err error
	var checks []EnvironmentCheck
	if action == "setup" {
		output, setupErr := m.setup(ctx, candidate)
		err = setupErr
		checks = append(checks, EnvironmentCheck{Name: "Windows sandbox setup", Passed: err == nil, Evidence: boundedEvidence(output)})
	}
	if err == nil {
		err = m.publish("verifying", "Verifying workspace isolation and Git shell startup.")
	}
	if err == nil {
		probeChecks, probeErr := m.probe(ctx, candidate)
		checks = append(checks, probeChecks...)
		err = probeErr
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err==nil {
		err=m.audit(ctx,"EnvironmentVerificationPassed","Selected environment passed isolation and Git shell checks",map[string]any{"mode":mode,"checks":checks})
	}
	// Check is diagnostic only; selection/setup changes the policy after success.
	if err == nil && action != "check" {
		err = m.saveMode(mode)
	}
	if err == nil && action != "check" {
		err = m.cli.SetSandboxMode(mode)
	}
	now := time.Now().UTC()
	m.mu.Lock()
	m.state.Mode, m.state.Checks, m.state.CheckedAt = m.cli.SandboxMode(), checks, &now
	m.state.Status, m.state.Message = "ready", "Workspace isolation and Git shell startup passed. Approved contributions can be resumed."
	if err != nil {
		m.state.Status, m.state.Error = "failed", boundedEvidence(err.Error())
		m.state.Message = "The environment did not pass verification. The previous sandbox mode remains selected."
	}
	snapshot := m.state
	m.mu.Unlock()
	auditCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	auditErr := m.audit(auditCtx, "EnvironmentOperationCompleted", snapshot.Message, snapshot)
	stop()
	m.mu.Lock()
	if auditErr != nil {
		m.state.Status, m.state.Error = "failed", "Cannot persist environment completion audit: "+auditErr.Error()
	}
	m.state.Running = false
	m.mu.Unlock()
}
func boundedEvidence(s string) string {
	if len(s) > 8192 {
		return s[:8192] + "\n[truncated]"
	}
	return s
}

func (m *EnvironmentManager) saveMode(mode string) error {
	dir := filepath.Dir(m.settingsPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(dir)) {
		return errors.New("sandbox settings directory crosses a link boundary")
	}
	if info, e := os.Lstat(m.settingsPath()); e == nil && !info.Mode().IsRegular() {
		return errors.New("sandbox settings must be a regular file")
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	data, _ := json.MarshalIndent(map[string]any{"mode": mode, "verified_at": time.Now().UTC()}, "", "  ")
	f, err := os.CreateTemp(dir, "sandbox-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, m.settingsPath())
}

func setupWindowsSandbox(ctx context.Context, c *CLI) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("Windows sandbox setup is unsupported on this platform")
	}
	// Codex launches its official elevated setup helper and owns Windows UAC.
	// ForgeFlow remains unelevated; no repository command runs as administrator.
	cmd := exec.CommandContext(ctx, c.Binary, "sandbox", "setup", "--elevated", "--current-user")
	cmd.Env = environment()
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	out := &bounded{limit: 8192}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		return out.String(), fmt.Errorf("Windows sandbox setup failed or was declined: %w; %s", err, out.String())
	}
	return out.String(), nil
}

func (m *EnvironmentManager) probeEnvironment(ctx context.Context, c *CLI) ([]EnvironmentCheck, error) {
	if err := os.MkdirAll(m.base, 0700); err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(m.base)
	if err != nil || !strings.EqualFold(filepath.Clean(real), filepath.Clean(m.base)) {
		return nil, errors.New("contribution root crosses a link boundary")
	}
	dir, err := os.MkdirTemp(m.base, ".forgeflow-environment-")
	if err != nil {
		return nil, err
	}
	// This is our freshly-created disposable directory, never a contribution.
	defer func() {
		absolute,e:=filepath.Abs(dir)
		if e==nil && filepath.IsAbs(m.base) && filepath.Dir(absolute)==filepath.Clean(m.base) && strings.HasPrefix(filepath.Base(absolute),".forgeflow-environment-") {
			_ = os.RemoveAll(absolute)
		}
	}()
	repo := filepath.Join(dir, "repo")
	if err = os.Mkdir(repo, 0700); err != nil {
		return nil, err
	}
	init := exec.CommandContext(ctx, "git", "-c", "core.hooksPath="+filepath.Join(dir, "no-hooks"), "init", "--quiet", repo)
	init.Env = environment()
	configureProcess(init)
	if out, e := init.CombinedOutput(); e != nil {
		return nil, fmt.Errorf("cannot initialize disposable probe repository: %w; %s", e, out)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	err = c.Check(checkCtx, repo, m.root)
	cancel()
	checks := []EnvironmentCheck{{Name: "Workspace write and denied writes outside it", Passed: err == nil}}
	if err != nil {
		checks[0].Evidence = boundedEvidence(err.Error())
		return checks, err
	}
	checkCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
	run, err := c.ProbeGitShell(checkCtx, repo)
	cancel()
	checks = append(checks, EnvironmentCheck{Name: "Git shell startup", Passed: err == nil && run.ExitCode == 0, Evidence: boundedEvidence(run.Output)})
	if err != nil || run.ExitCode != 0 {
		return checks, fmt.Errorf("Git shell startup failed in %s mode: %s", c.SandboxMode(), run.Output)
	}
	return checks, nil
}
