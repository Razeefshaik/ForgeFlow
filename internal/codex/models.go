package codex

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var modelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)

// ValidateModelID accepts a Codex model ID without allowing flags or whitespace.
func ValidateModelID(model string) error {
	if model != "" && !modelID.MatchString(model) {
		return errors.New("enter a valid Codex model ID")
	}
	return nil
}

type ModelOption struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

// LocalModels reads Codex's local catalogue. Entries are choices, not proof of entitlement.
func LocalModels() ([]ModelOption, error) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = filepath.Join(userHome, ".codex")
	}
	f, err := os.Open(filepath.Join(home, "models_cache.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 2<<20 {
		return nil, errors.New("Codex model catalogue is too large")
	}
	var cache struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
		} `json:"models"`
	}
	if err := json.Unmarshal(b, &cache); err != nil {
		return nil, err
	}
	models := make([]ModelOption, 0, len(cache.Models))
	seen := make(map[string]bool)
	for _, m := range cache.Models {
		if m.Visibility != "list" || ValidateModelID(m.Slug) != nil || m.Slug == "" || seen[m.Slug] {
			continue
		}
		seen[m.Slug] = true
		name := strings.TrimSpace(m.DisplayName)
		if name == "" {
			name = m.Slug
		}
		models = append(models, ModelOption{Slug: m.Slug, DisplayName: name})
	}
	return models, nil
}
