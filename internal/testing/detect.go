package testing

import (
	"encoding/json"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
)

func Detect(repo string) []domain.VerificationCommand {
	exists := func(name string) bool {
		v, e := os.Lstat(filepath.Join(repo, name))
		return e == nil && v.Mode().IsRegular()
	}
	commands := []domain.VerificationCommand{}
	if exists("go.mod") {
		commands = append(commands, domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, domain.VerificationCommand{Program: "go", Arguments: []string{"vet", "./..."}})
	}
	if exists("pom.xml") {
		p := "mvn"
		if exists("mvnw") {
			p = "./mvnw"
		}
		commands = append(commands, domain.VerificationCommand{Program: p, Arguments: []string{"test"}})
	}
	if exists("build.gradle") || exists("build.gradle.kts") {
		p := "gradle"
		if exists("gradlew") {
			p = "./gradlew"
		}
		commands = append(commands, domain.VerificationCommand{Program: p, Arguments: []string{"test"}})
	}
	if exists("pyproject.toml") || exists("pytest.ini") || exists("setup.py") {
		commands = append(commands, domain.VerificationCommand{Program: "python", Arguments: []string{"-m", "pytest"}})
	}
	if exists("package.json") {
		var p struct {
			Scripts map[string]string `json:"scripts"`
		}
		b, e := os.ReadFile(filepath.Join(repo, "package.json"))
		if e == nil && len(b) < 128<<10 && json.Unmarshal(b, &p) == nil {
			for _, name := range []string{"test", "lint", "build", "typecheck"} {
				if p.Scripts[name] != "" {
					commands = append(commands, domain.VerificationCommand{Program: "npm", Arguments: []string{"run", name}})
				}
			}
		}
	}
	return commands
}
