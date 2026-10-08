package codex

import (
	"context"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestApprovedNetworkRemovesOnlyInheritedDiscardProxies(t *testing.T) {
	env := []string{"HTTPS_PROXY=http://127.0.0.1:9", "http_proxy=http://localhost:9/", "ALL_PROXY=socks5://[::1]:9", "NO_PROXY=localhost", "OTHER=value", "https_proxy=http://proxy.example:8080", "HTTP_PROXY=http://user:secret@127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9000"}
	if actual := approvedNetworkEnvironment(env, false); !reflect.DeepEqual(actual, env) {
		t.Fatal("unapproved network environment changed")
	}
	expected := env[3:]
	if actual := approvedNetworkEnvironment(env, true); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("valid proxies were modified or discard proxies survived: %v", actual)
	}
	if !strings.Contains(env[0], "127.0.0.1:9") {
		t.Fatal("parent environment was mutated")
	}
}

func TestRealApprovedSandboxTransport(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_SANDBOX_TEST") != "1" {
		t.Skip("opt in to an actual installed sandbox and public dependency endpoint probe")
	}
	cli := Resolve("", "")
	repo, appRoot := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := cli.Check(ctx, repo, appRoot); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
	// No model session is used. This proves actual sandbox network access and
	// verifies that a stale discard proxy cannot reach the approved child.
	script := `if((process.env.HTTPS_PROXY||'').includes('127.0.0.1:9')){console.error('discard proxy inherited');process.exit(7)} require('https').get('https://proxy.golang.org/golang.org/toolchain/@v/list',r=>{r.resume();r.on('end',()=>{console.log('dependency endpoint status '+r.statusCode);process.exit(r.statusCode===200?0:8)})}).on('error',e=>{console.error(e.code);process.exit(9)})`
	run, err := cli.sandbox(ctx, repo, domain.VerificationCommand{Program: "node", Arguments: []string{"-e", script}}, true)
	if err != nil || run.ExitCode != 0 {
		t.Fatalf("approved sandbox transport failed: %v, %s", err, run.Output)
	}
	t.Log(run.Output)
}

func TestWorkspaceCacheCollisionFailsBeforeCommandLaunch(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".forgeflow-runtime"), []byte("tracked file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceEnvironmentForNetwork(repo, true); err == nil {
		t.Fatal("cache collision silently fell back to host directories")
	}
	value, _ := os.ReadFile(filepath.Join(repo, ".forgeflow-runtime"))
	if string(value) != "tracked file" {
		t.Fatal("existing repository file was replaced")
	}
}
