package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocateRootFromSubdirectoryAndExecutable(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"SPEC.md", "go.mod"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, cwd := range []string{filepath.Join(root, "apps", "web"), t.TempDir()} {
		got, err := Locate("", cwd, filepath.Join(root, "bin", "forgeflow.exe"))
		if err != nil || got != root {
			t.Fatal(got, err)
		}
	}
	if _, err := Locate(t.TempDir(), root, ""); err == nil {
		t.Fatal("invalid explicit root accepted")
	}
	if got := Resolve(root, "data/demo.db"); got != filepath.Join(root, "data", "demo.db") {
		t.Fatal("database not rooted", got)
	}
	absolute := filepath.Join(t.TempDir(), "custom.db")
	if Resolve(root, absolute) != absolute {
		t.Fatal("absolute override changed")
	}
}

func TestVerificationImageIsExplicitAndEnvironmentOverridesLocal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FORGEFLOW_VERIFICATION_IMAGE", "")
	s, err := LoadSettings(root)
	if err != nil || s.VerificationImage != "" {
		t.Fatal("Docker became mandatory", s, err)
	}
	if err = os.Mkdir(filepath.Join(root, "configs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "configs", "local.json"), []byte(`{"verification_image":"fixture:1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = LoadSettings(root)
	if err != nil || s.VerificationImage != "fixture:1" {
		t.Fatal(s, err)
	}
	t.Setenv("FORGEFLOW_VERIFICATION_IMAGE", "fixture:2")
	s, err = LoadSettings(root)
	if err != nil || s.VerificationImage != "fixture:2" {
		t.Fatal(s, err)
	}
}
