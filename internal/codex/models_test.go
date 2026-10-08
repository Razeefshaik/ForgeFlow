package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalModelsOnlyListsVisibleValidChoices(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	cache := `{"models":[{"slug":"model-a","display_name":"Model A","visibility":"list"},{"slug":"model-hidden","display_name":"Hidden","visibility":"hide"},{"slug":"--flag","display_name":"Flag","visibility":"list"},{"slug":"model-a","display_name":"Duplicate","visibility":"list"}]}`
	if err := os.WriteFile(filepath.Join(home, "models_cache.json"), []byte(cache), 0600); err != nil {
		t.Fatal(err)
	}
	models, err := LocalModels()
	if err != nil || len(models) != 1 || models[0].Slug != "model-a" || models[0].DisplayName != "Model A" {
		t.Fatalf("unexpected catalogue: %+v, %v", models, err)
	}
}

func TestValidateModelID(t *testing.T) {
	for _, value := range []string{"--flag", "model name", "model\nother"} {
		if ValidateModelID(value) == nil {
			t.Fatalf("accepted invalid model %q", value)
		}
	}
	if err := ValidateModelID("gpt-6.1-sol"); err != nil {
		t.Fatal(err)
	}
}
