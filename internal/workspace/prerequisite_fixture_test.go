package workspace

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"forgeflow/internal/github"
	"net/http"
	"strings"
	"testing"
	"time"
)

// servePrerequisite models a contributable repository: instructions, a root
// go.mod, a test file and a successful four-minute GitHub Actions run.
func servePrerequisite(t *testing.T, w http.ResponseWriter, r *http.Request, repo, head, body string) bool {
	t.Helper()
	blob := func(content string) string {
		return fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content))))
	}
	mod := "module example.com/fixture\n\ngo 1.26\n"
	sha, modSHA := blob(body), blob(mod)
	switch r.URL.Path {
	case "/repos/" + repo + "/git/trees/" + head:
		json.NewEncoder(w).Encode(map[string]any{"tree": []map[string]any{
			{"path": "CONTRIBUTING.md", "type": "blob", "mode": "100644", "sha": sha, "size": len(body)},
			{"path": "go.mod", "type": "blob", "mode": "100644", "sha": modSHA, "size": len(mod)},
			{"path": "fixture_test.go", "type": "blob", "mode": "100644", "sha": strings.Repeat("c", 40), "size": 20},
		}})
		return true
	case "/repos/" + repo + "/git/blobs/" + sha:
		json.NewEncoder(w).Encode(github.Blob{SHA: sha, Size: len(body), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(body))})
		return true
	case "/repos/" + repo + "/git/blobs/" + modSHA:
		json.NewEncoder(w).Encode(github.Blob{SHA: modSHA, Size: len(mod), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(mod))})
		return true
	case "/repos/" + repo + "/actions/runs":
		end := time.Now().UTC().Add(-time.Hour)
		fmt.Fprintf(w, `{"workflow_runs":[{"name":"tests","path":".github/workflows/ci.yml","head_branch":"main","event":"push","status":"completed","conclusion":"success","run_started_at":%q,"updated_at":%q}]}`, end.Add(-4*time.Minute).Format(time.RFC3339), end.Format(time.RFC3339))
		return true
	}
	return false
}
