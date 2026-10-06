package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProductionSPARootRoutesAssetsAndMissingAsset(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>ForgeFlow entry</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("console.log('app')"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer((Server{WebDir: root}).Handler())
	defer server.Close()
	for _, path := range []string{"/", "/opportunities", "/contributions"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || string(b) != "<html>ForgeFlow entry</html>" {
			t.Fatalf("SPA route %s failed: %d %s %v", path, resp.StatusCode, b, err)
		}
	}
	for path, want := range map[string]int{"/assets/app.js": 200, "/assets/missing.js": 404} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("asset %s returned %d", path, resp.StatusCode)
		}
	}
}
