package workspace

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"forgeflow/internal/github"
	"net/http"
	"testing"
)

func servePrerequisite(t *testing.T, w http.ResponseWriter, r *http.Request, repo, head, body string) bool {
	t.Helper()
	sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(body), body)))
	sha := fmt.Sprintf("%x", sum)
	switch r.URL.Path {
	case "/repos/" + repo + "/git/trees/" + head:
		json.NewEncoder(w).Encode(map[string]any{"tree": []map[string]any{{"path": "CONTRIBUTING.md", "type": "blob", "mode": "100644", "sha": sha, "size": len(body)}}})
		return true
	case "/repos/" + repo + "/git/blobs/" + sha:
		json.NewEncoder(w).Encode(github.Blob{SHA: sha, Size: len(body), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(body))})
		return true
	}
	return false
}
