package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct {
	ContributionsDir  string `json:"contributions_dir"`
	CodexBinary       string `json:"codex_binary,omitempty"`
	CodexModel        string `json:"codex_model,omitempty"`
	CodexSandbox      string `json:"codex_sandbox,omitempty"`
	VerificationImage string `json:"verification_image,omitempty"`
}

func LoadSettings(root string) (Settings, error) {
	settings := Settings{ContributionsDir: "contributions"}
	b, err := os.ReadFile(filepath.Join(root, "configs", "local.json"))
	if err == nil {
		if err = json.Unmarshal(b, &settings); err != nil {
			return settings, errors.New("invalid configs/local.json")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return settings, err
	}
	for key, target := range map[string]*string{"FORGEFLOW_CONTRIBUTIONS_DIR": &settings.ContributionsDir, "FORGEFLOW_CODEX_BIN": &settings.CodexBinary, "FORGEFLOW_CODEX_MODEL": &settings.CodexModel, "FORGEFLOW_CODEX_SANDBOX": &settings.CodexSandbox, "FORGEFLOW_VERIFICATION_IMAGE": &settings.VerificationImage} {
		if value := os.Getenv(key); value != "" {
			*target = value
		}
	}
	if settings.ContributionsDir == "" {
		settings.ContributionsDir = "contributions"
	}
	settings.ContributionsDir = Resolve(root, settings.ContributionsDir)
	return settings, nil
}
