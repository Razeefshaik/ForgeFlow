package prerequisites

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func inspectFixture(t *testing.T, files map[string]string, mutate func(*github.Blob)) (github.Tree, *github.Session) {
	t.Helper()
	nodes := []map[string]any{}
	blobs := map[string]github.Blob{}
	for file, body := range files {
		sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(body), body)))
		sha := fmt.Sprintf("%x", sum)
		nodes = append(nodes, map[string]any{"path": file, "type": "blob", "mode": "100644", "sha": sha, "size": len(body)})
		blobs[sha] = github.Blob{SHA: sha, Size: len(body), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(body))}
	}
	var tree github.Tree
	b, _ := json.Marshal(map[string]any{"tree": nodes})
	if err := json.Unmarshal(b, &tree); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blob, ok := blobs[strings.TrimPrefix(r.URL.Path, "/repos/new/project/git/blobs/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if mutate != nil {
			mutate(&blob)
		}
		json.NewEncoder(w).Encode(blob)
	}))
	t.Cleanup(srv.Close)
	c := github.New("")
	c.BaseURL = srv.URL
	return tree, &github.Session{Client: c, MaxRequests: MaxFiles}
}

func TestFreshRepositoryRequirementsAcrossLanguages(t *testing.T) {
	for _, tc := range []struct {
		name         string
		files        map[string]string
		status, kind string
	}{
		{"versioned-go", map[string]string{"CONTRIBUTING.md": "Run go test ./... before contributing.", "go.mod": "module new/project\ngo 1.27.1"}, "eligible", ""},
		{"versioned-python", map[string]string{"CONTRIBUTING.md": "Required checks: pytest and ruff.", "pyproject.toml": "[project]\nrequires-python = '>=3.14'"}, "eligible", ""},
		{"versioned-java", map[string]string{"CONTRIBUTING.md": "Run mvn verify.", "pom.xml": "<project><java.version>25</java.version></project>"}, "eligible", ""},
		{"versioned-node", map[string]string{"package.json": "{\"engines\":{\"node\":\">=24\"},\"scripts\":{\"test\":\"vitest run\"}}"}, "eligible", ""},
		{"trusted-signing", map[string]string{"CONTRIBUTING.md": "Contributors must supply an Ed25519 signed receipt."}, "excluded", "trusted_signing"},
		{"private-live-service", map[string]string{"CONTRIBUTING.md": "Required live integration tests need a running private server and credentials."}, "excluded", "private_credentials"},
		{"compose-verification", map[string]string{"CONTRIBUTING.md": "Run make test.", "Makefile": "test: service\n\tpytest\nservice:\n\tdocker compose up -d\n"}, "excluded", "nested_docker"},
		{"optional-image", map[string]string{"CONTRIBUTING.md": "Run make test.\nDocker is optional.", "Makefile": "test:\n\tgo test ./...\nrelease:\n\tdocker build .\n"}, "eligible", ""},
		{"release-secret-irrelevant", map[string]string{".github/workflows/ci.yml": "jobs:\n  test:\n    steps:\n      - run: go test ./...\n  release:\n    steps:\n      - env: ${{ secrets.PUBLISH_TOKEN }}\n"}, "eligible", ""},
		{"system-writes", map[string]string{"CONTRIBUTING.md": "Run make test.", "Makefile": "test:\n\tsudo mkdir /etc/application\n"}, "excluded", "runtime_privilege"},
		{"gpu-checks", map[string]string{"CONTRIBUTING.md": "Verification requires an NVIDIA GPU."}, "excluded", "unsupported_platform"},
		{"large-verification", map[string]string{"CONTRIBUTING.md": "Local tests require at least 8 GiB memory."}, "excluded", "resource_limit"},
		{"ci-service-needs-review", map[string]string{".github/workflows/test.yml": "jobs:\n  integration:\n    services:\n      db:\n        image: postgres:17\n"}, "needs_assessment", "needs_assessment"},
		{"optional-product-credential", map[string]string{"README.md": "# Using the API\nThe hosted product requires an access token.\n# Development\nNo signing key is required.\nRun go test ./..."}, "eligible", ""},
		{"example-dsn-is-not-a-private-credential-requirement", map[string]string{"AGENTS.md": "Example configuration:\nVERA_POSTGRES_DSN=postgres://USER:PASSWORD@HOST:5432/DATABASE?sslmode=require\nRun go test ./..."}, "eligible", ""},
		{"referenced-script", map[string]string{"CONTRIBUTING.md": "Run make verify.", "Makefile": "verify:\n\t./scripts/check.sh\n", "scripts/check.sh": "#!/bin/sh\ndocker exec service test-command\n"}, "excluded", "nested_docker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, session := inspectFixture(t, tc.files, nil)
			a := Inspect(context.Background(), session, "new/project", strings.Repeat("a", 40), tree, LinuxProfile())
			if a.Status != tc.status {
				t.Fatal(a)
			}
			if tc.kind != "" {
				found := false
				for _, f := range a.Findings {
					if f.Kind == tc.kind && f.Source != "" {
						found = true
					}
				}
				if !found {
					t.Fatal(a)
				}
			}
		})
	}
}

func TestIncompleteOrCorruptedEvidenceNeverRanks(t *testing.T) {
	for _, mode := range []string{"truncated", "bad-sha", "oversized", "budget", "cancelled", "native-only", "missing-linked-document"} {
		t.Run(mode, func(t *testing.T) {
			files := map[string]string{"CONTRIBUTING.md": "Run go test ./..."}
			var mutate func(*github.Blob)
			if mode == "bad-sha" {
				mutate = func(b *github.Blob) {
					b.Content = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", b.Size)))
				}
			}
			if mode == "oversized" {
				files["CONTRIBUTING.md"] = strings.Repeat("x", MaxFileBytes+1)
			}
			if mode == "missing-linked-document" {
				files["CONTRIBUTING.md"] = "See [required verification](docs/testing.md)."
			}
			tree, session := inspectFixture(t, files, mutate)
			if mode == "truncated" {
				tree.Truncated = true
			}
			if mode == "budget" {
				session.MaxRequests = 0
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			p := LinuxProfile()
			if mode == "native-only" {
				p.LinuxProvisioning = false
			}
			a := Inspect(ctx, session, "new/project", strings.Repeat("a", 40), tree, p)
			if a.Status != "needs_assessment" || len(a.Findings) == 0 {
				t.Fatal(a)
			}
		})
	}
}
