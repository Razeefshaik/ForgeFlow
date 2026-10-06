package contributions

import (
	"path/filepath"
	"testing"
)

func TestExternalWorkspacePathsUseApplicationRoot(t *testing.T) {
	root := t.TempDir()
	got, err := WorkspacePath(root, "etcd-12345")
	if err != nil || got != filepath.Join(root, "contributions", "etcd-12345", "repo") {
		t.Fatal(got, err)
	}
	for _, id := range []string{"", "..", "../escape", "a/b", `a\b`} {
		if _, err = WorkspacePath(root, id); err == nil {
			t.Fatal("unsafe ID accepted", id)
		}
	}
}
