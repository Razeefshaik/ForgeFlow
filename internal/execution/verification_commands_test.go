package execution

import (
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlannedBuildSubsumesOnlyDetectedTestRun(t *testing.T) {
	repo := func(files ...string) string {
		dir := t.TempDir()
		for _, name := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	gradle := repo("build.gradle", "gradlew", "go.mod")
	maven := repo("pom.xml", "mvnw")
	cmd := func(program string, args ...string) domain.VerificationCommand {
		return domain.VerificationCommand{Program: program, Arguments: args}
	}
	goTest, goVet, gradlewTest, mvnwTest := cmd("go", "test", "./..."), cmd("go", "vet", "./..."), cmd("./gradlew", "test"), cmd("./mvnw", "test")
	for _, tc := range []struct {
		name    string
		repo    string
		planned []domain.VerificationCommand
		want    []domain.VerificationCommand
	}{
		// 2026-10-17: the full Gradle build ran twice.
		{"provisioned gradle build", gradle, []domain.VerificationCommand{cmd("/opt/gradle-8.10.2/bin/gradle", "--offline", "--no-daemon", "build")}, []domain.VerificationCommand{goTest, goVet, cmd("/opt/gradle-8.10.2/bin/gradle", "--offline", "--no-daemon", "build")}},
		{"wrapper check", gradle, []domain.VerificationCommand{cmd("./gradlew", "check")}, []domain.VerificationCommand{goTest, goVet, cmd("./gradlew", "check")}},
		{"tests excluded", gradle, []domain.VerificationCommand{cmd("./gradlew", "build", "-x", "test")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("./gradlew", "build", "-x", "test")}},
		{"check excluded", gradle, []domain.VerificationCommand{cmd("gradle", "build", "--exclude-task=check")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("gradle", "build", "--exclude-task=check")}},
		{"subproject build", gradle, []domain.VerificationCommand{cmd("./gradlew", ":app:build")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("./gradlew", ":app:build")}},
		{"other project directory", gradle, []domain.VerificationCommand{cmd("./gradlew", "-p", "tools", "build")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("./gradlew", "-p", "tools", "build")}},
		{"skip property", gradle, []domain.VerificationCommand{cmd("./gradlew", "build", "-PskipTests")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("./gradlew", "build", "-PskipTests")}},
		{"planned test is kept", gradle, []domain.VerificationCommand{gradlewTest, cmd("./gradlew", "build")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("./gradlew", "build")}},
		{"other tool family", gradle, []domain.VerificationCommand{cmd("mvn", "verify")}, []domain.VerificationCommand{goTest, goVet, gradlewTest, cmd("mvn", "verify")}},
		{"maven verify", maven, []domain.VerificationCommand{cmd("mvn", "-B", "verify")}, []domain.VerificationCommand{cmd("mvn", "-B", "verify")}},
		{"maven wrapper install", maven, []domain.VerificationCommand{cmd("./mvnw", "clean", "install", "-DskipTests=false")}, []domain.VerificationCommand{cmd("./mvnw", "clean", "install", "-DskipTests=false")}},
		{"maven skipTests", maven, []domain.VerificationCommand{cmd("./mvnw", "package", "-DskipTests")}, []domain.VerificationCommand{mvnwTest, cmd("./mvnw", "package", "-DskipTests")}},
		{"maven test skip", maven, []domain.VerificationCommand{cmd("./mvnw", "verify", "-Dmaven.test.skip=true")}, []domain.VerificationCommand{mvnwTest, cmd("./mvnw", "verify", "-Dmaven.test.skip=true")}},
		{"maven test filter", maven, []domain.VerificationCommand{cmd("./mvnw", "-Dtest=ParserTest", "verify")}, []domain.VerificationCommand{mvnwTest, cmd("./mvnw", "-Dtest=ParserTest", "verify")}},
		{"maven module selection", maven, []domain.VerificationCommand{cmd("./mvnw", "-pl", "core", "verify")}, []domain.VerificationCommand{mvnwTest, cmd("./mvnw", "-pl", "core", "verify")}},
		{"maven compile only", maven, []domain.VerificationCommand{cmd("./mvnw", "compile")}, []domain.VerificationCommand{mvnwTest, cmd("./mvnw", "compile")}},
	} {
		got, err := verificationCommands(tc.repo, &domain.Plan{Tests: tc.planned}, codex.ValidateContainerCommand)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if display(got) != display(tc.want) {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, display(got), display(tc.want))
		}
	}
}

func display(commands []domain.VerificationCommand) string {
	parts := []string{}
	for _, c := range commands {
		parts = append(parts, displayCommand(c))
	}
	return strings.Join(parts, " | ")
}
