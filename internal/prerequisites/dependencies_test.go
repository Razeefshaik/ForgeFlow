package prerequisites

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNestedTemplateDocumentationIsProvisionable(t *testing.T) {
	files := map[string]string{
		"README.md":                           "# AWS SAM Backstage templates\nGenerated applications deploy to AWS.\n",
		"skeleton/crud/mkdocs.yml":            "site_name: '${{ values.component_name }} handbook'\nplugins:\n  - techdocs-core\nnav:\n  - Handbook: index.md\n",
		"skeleton/base/Pipfile":               "[requires]\npython_version = '3.13'\n",
		"services/api/package.json":           "{\"scripts\":{\"test\":\"vitest run\"}}",
		"pipeline/.github/workflows/main.yml": "jobs:\n  deploy:\n    steps:\n      - run: sam deploy\n        env: ${{ secrets.AWS_SECRET_ACCESS_KEY }}\n",
	}
	tree, session := inspectFixture(t, files, nil)
	a := Inspect(context.Background(), session, "new/project", strings.Repeat("a", 40), tree, LinuxProfile())
	if a.Status != "eligible" || len(a.Findings) != 0 || len(a.Requirements) != 3 {
		t.Fatalf("ordinary dependencies blocked or missed: %+v", a)
	}
	for _, f := range a.Files {
		if f == "pipeline/.github/workflows/main.yml" {
			t.Fatal("generated deployment treated as contribution CI")
		}
	}
	found := false
	for _, r := range a.Requirements {
		if r.Source == "skeleton/crud/mkdocs.yml" && r.Kind == "documentation_build" {
			found = true
		}
	}
	if !found {
		t.Fatal("nested unpinned MkDocs build missed")
	}
}

func TestNestedVerificationStillRejectsUnsupportedRequirements(t *testing.T) {
	tree, session := inspectFixture(t, map[string]string{"packages/api/package.json": `{"scripts":{"test":"docker compose run tests"}}`}, nil)
	a := Inspect(context.Background(), session, "new/project", strings.Repeat("a", 40), tree, LinuxProfile())
	if a.Status != "excluded" || len(a.Findings) == 0 || a.Findings[0].Kind != "nested_docker" {
		t.Fatal(a)
	}
}

func TestActualHandbookPrerequisitesReadOnly(t *testing.T) {
	source := os.Getenv("FORGEFLOW_TEST_PREREQUISITES_SOURCE")
	if source == "" {
		t.Skip("opt in with an existing read-only handbook checkout")
	}
	files := map[string]string{}
	err := filepath.WalkDir(source, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 || (!selectedFile(rel) && !isDocument(rel)) {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	tree, session := inspectFixture(t, files, nil)
	a := Inspect(context.Background(), session, "new/project", strings.Repeat("a", 40), tree, LinuxProfile())
	if evidence := os.Getenv("FORGEFLOW_TEST_EVIDENCE"); evidence != "" {
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		body, err := json.MarshalIndent(a, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(evidence, "actual-handbook-prerequisites.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if a.Status != "eligible" {
		t.Fatalf("actual handbook prerequisites rejected: %+v", a)
	}
	found := false
	for _, r := range a.Requirements {
		if r.Source == "skeleton/crud/mkdocs.yml" {
			found = true
		}
	}
	if !found {
		t.Fatal("actual documentation configuration missed")
	}
}
