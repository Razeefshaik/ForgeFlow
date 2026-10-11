package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func environmentFixture(t *testing.T) (*EnvironmentManager, *CLI) {
	t.Helper()
	root, base := t.TempDir(), t.TempDir()
	cli := &CLI{Binary: "installed-codex"}
	m, err := NewEnvironmentManager(context.Background(), cli, root, base, func(context.Context, string, string, any) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	m.probe = func(context.Context, *CLI) ([]EnvironmentCheck, error) {
		return []EnvironmentCheck{{Name: "Isolation", Passed: true}, {Name: "Git shell", Passed: true}}, nil
	}
	return m, cli
}
func TestEnvironmentApprovalAndSandboxAllowlist(t *testing.T) {
	m, c := environmentFixture(t)
	for _, tc := range []struct {
		action, mode string
		approved     bool
	}{{"setup", "elevated", false}, {"select", "mxc", false}, {"select", "danger-full-access", true}, {"select", "auto\nanything", true}, {"run", "auto", true}, {"check", "elevated", false}} {
		if _, err := m.Start(tc.action, tc.mode, tc.approved); err == nil {
			t.Fatalf("accepted unsafe operation: %+v", tc)
		}
	}
	if c.SandboxMode() != "auto" || m.Status().Running {
		t.Fatal("rejected action changed state")
	}
}
func TestVerifiedModePersistsWithoutReplacingLocalConfiguration(t *testing.T) {
	m, c := environmentFixture(t)
	if err := os.Mkdir(filepath.Join(m.root, "configs"), 0700); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(m.root, "configs", "local.json")
	if err := os.WriteFile(local, []byte(`{"contributions_dir":"keep"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"mxc", "unelevated"} {
		if _, err := m.Start("select", mode, true); err != nil {
			t.Fatal(err)
		}
		m.Wait()
		if s := m.Status(); s.Status != "ready" || s.Running || c.SandboxMode() != mode {
			t.Fatal(s)
		}
	}
	b, _ := os.ReadFile(local)
	if string(b) != `{"contributions_dir":"keep"}` {
		t.Fatal("overwrote configuration")
	}
	reloaded, err := NewEnvironmentManager(context.Background(), &CLI{Binary: c.Binary}, m.root, m.base, m.audit)
	if err != nil || reloaded.Status().Mode != "unelevated" || reloaded.Status().Status != "unverified" {
		t.Fatal(reloaded, err)
	}
	explicit := &CLI{Binary: c.Binary}
	_ = explicit.SetSandboxMode("auto")
	reloaded, err = NewEnvironmentManager(context.Background(), explicit, m.root, m.base, m.audit)
	if err != nil || reloaded.Status().Mode != "auto" {
		t.Fatal("explicit auto overridden", err)
	}
}
func TestFailedProbeDoesNotAdoptOrSaveCandidate(t *testing.T) {
	m, c := environmentFixture(t)
	m.probe = func(context.Context, *CLI) ([]EnvironmentCheck, error) {
		return []EnvironmentCheck{{Name: "Git", Evidence: "signal pipe denied"}}, errors.New("signal pipe denied")
	}
	if _, err := m.Start("select", "elevated", true); err != nil {
		t.Fatal(err)
	}
	m.Wait()
	if s := m.Status(); s.Status != "failed" || !strings.Contains(s.Error, "signal pipe") || c.SandboxMode() != "auto" {
		t.Fatal(s)
	}
	if _, err := os.Stat(m.settingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("saved unverified mode", err)
	}
}
func TestEnvironmentSerializesOperationsAndAuditsRealResults(t *testing.T) {
	m, _ := environmentFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	m.probe = func(context.Context, *CLI) ([]EnvironmentCheck, error) {
		close(entered)
		<-release
		return []EnvironmentCheck{{Name: "isolation", Passed: true}, {Name: "shell", Passed: true}}, nil
	}
	var mu sync.Mutex
	var events []string
	m.audit = func(_ context.Context, kind, _ string, _ any) error {
		mu.Lock()
		events = append(events, kind)
		mu.Unlock()
		return nil
	}
	if _, err := m.Start("check", "auto", false); err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := m.Start("select", "mxc", true); err == nil {
		t.Fatal("concurrent selection allowed")
	}
	close(release)
	m.Wait()
	if !reflect.DeepEqual(events, []string{"EnvironmentOperationStarted", "EnvironmentOperationUpdated", "EnvironmentVerificationPassed", "EnvironmentOperationCompleted"}) {
		t.Fatal(events)
	}
	if _, err := os.Stat(m.settingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("diagnostic check persisted configuration", err)
	}
}
func TestEnvironmentAuditFailurePreventsSelection(t *testing.T) {
	m, c := environmentFixture(t)
	m.audit = func(_ context.Context, kind, _ string, _ any) error {
		if kind == "EnvironmentVerificationPassed" {
			return errors.New("disk full")
		}
		return nil
	}
	if _, err := m.Start("select", "mxc", true); err != nil {
		t.Fatal(err)
	}
	m.Wait()
	if m.Status().Status != "failed" || c.SandboxMode() != "auto" {
		t.Fatal(m.Status())
	}
}
func TestEnvironmentCancellationPreservesExistingMode(t *testing.T) {
	m, c := environmentFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	entered := make(chan struct{})
	m.probe = func(ctx context.Context, _ *CLI) ([]EnvironmentCheck, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if _, err := m.Start("select", "mxc", true); err != nil {
		t.Fatal(err)
	}
	<-entered
	cancel()
	m.Wait()
	if c.SandboxMode() != "auto" || m.Status().Status != "failed" {
		t.Fatal(m.Status())
	}
}
func TestExplicitLegacyModeDisablesMXCPreference(t *testing.T) {
	for _, mode := range []string{"mxc", "elevated", "unelevated"} {
		args := strings.Join(windowsPolicy(mode), " ")
		if !strings.Contains(args, "features.prefer_mxc=false") || !strings.Contains(args, `windows.sandbox="`+mode+`"`) {
			t.Fatal(args)
		}
	}
}
