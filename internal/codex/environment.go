package codex

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// EnvironmentPreparer gives the control plane provisioning authority. Agent
// requests are data; no request is ever executed by a host shell.
type EnvironmentPreparer interface {
	PrepareEnvironment(context.Context, string, bool, domain.EnvironmentPlan) (domain.ExecutionEnvironment, error)
	EnvironmentFingerprint(string, domain.EnvironmentPlan) (string, error)
	EnvironmentBase() string
}

var imageReference = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,250}$`)
var immutableImage = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var environmentKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
var imageMissing = errors.New("Docker image is not cached")

func (c *CLI) EnvironmentBase() string { return c.VerificationImage }

// InspectEnvironment supplies facts the read-only agent cannot obtain through
// its native editing shell. No repository script or installation runs.
func (c *CLI) InspectEnvironment(ctx context.Context, dir string) (domain.ExecutionEnvironment, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	r := domain.ExecutionEnvironment{Status: "inspected", Capabilities: []string{"The configured image is locally available; below are actual offline tool probes, not inferred versions"}}
	id, err := localImage(ctx, c.VerificationImage)
	if err != nil {
		return r, err
	}
	r.BaseImage, r.Image = id, id
	probe := `for tool in go node npm python python3 java javac mvn gradle cargo rustc dotnet ruby bundle php composer gcc clang cmake make ninja uv bun yarn pnpm swift; do
 if command -v "$tool" >/dev/null 2>&1; then
  echo "=== $tool ==="
  case "$tool" in go) "$tool" version;; java) "$tool" -version;; dotnet) "$tool" --list-sdks;; *) "$tool" --version;; esac 2>&1 | head -n 4
 else echo "=== $tool: unavailable ==="; fi
done
git --version
git rev-parse --is-inside-work-tree`
	rec, err := c.containerCommandImage(ctx, id, dir, domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", probe}}, false)
	r.Checks = []domain.TestRun{rec}
	if err != nil {
		r.Status, r.Error = "blocked", err.Error()
	}
	return r, err
}

func (c *CLI) workspaceImage(dir string) string {
	c.environmentMu.Lock()
	defer c.environmentMu.Unlock()
	if env, ok := c.environments[filepath.Clean(dir)]; ok && env.Status == "ready" && env.Image != "" {
		return env.Image
	}
	return c.VerificationImage
}

func validateProvisionCommand(command domain.VerificationCommand) error {
	if err := validateCommandTimeout(command); err != nil {
		return err
	}
	if command.Program == "" || strings.HasPrefix(command.Program, "-") || strings.ContainsAny(command.Program, "\x00\r\n") || len(command.Arguments) > 100 {
		return errors.New("invalid environment command")
	}
	size := len(command.Program)
	for _, arg := range command.Arguments {
		size += len(arg)
		if strings.ContainsRune(arg, 0) {
			return errors.New("invalid environment argument")
		}
	}
	if size > 16<<10 {
		return errors.New("environment command exceeds 16 KiB")
	}
	return nil
}

// Generic executables are supported only behind the container boundary. Native
// execution retains its existing allowlist and argument checks.
func ValidateContainerCommand(command domain.VerificationCommand) error {
	if err := validateProvisionCommand(command); err != nil {
		return err
	}
	if strings.ContainsAny(command.Program, " \t") {
		return errors.New("container command program must be one executable; put arguments in the argument array")
	}
	return nil
}

func ValidateEnvironmentPlan(plan domain.EnvironmentPlan) error {
	if strings.TrimSpace(plan.Summary) == "" || !imageReference.MatchString(plan.BaseImage) || strings.HasSuffix(plan.BaseImage, ":latest") || (!strings.Contains(plan.BaseImage, "@") && strings.LastIndex(plan.BaseImage, ":") <= strings.LastIndex(plan.BaseImage, "/")) {
		return errors.New("environment plan needs a summary and explicit image reference")
	}
	if len(plan.Files) > 64 || len(plan.ImageCommands) > 24 || len(plan.WorkspaceCommands) > 24 || len(plan.HookCommands) > 8 || len(plan.CompletionGates) > 8 || (len(plan.Checks) == 0 && len(plan.Blockers) == 0) || len(plan.Checks) > 24 || len(plan.Environment) > 32 {
		return errors.New("environment plan exceeds bounds or has no prerequisite checks")
	}
	encoded, _ := json.Marshal(plan)
	if len(encoded) > 256<<10 {
		return errors.New("environment recipe exceeds 256 KiB")
	}
	for _, commands := range [][]domain.VerificationCommand{plan.ImageCommands, plan.WorkspaceCommands, plan.HookCommands, plan.Checks} {
		for _, command := range commands {
			if err := validateProvisionCommand(command); err != nil {
				return err
			}
		}
	}
	for _, gate := range plan.CompletionGates {
		if gate.Binding != "working_tree" && gate.Binding != "commit" {
			return errors.New("completion gate must declare working_tree or commit evidence binding")
		}
		if strings.TrimSpace(gate.Requirement) == "" || strings.TrimSpace(gate.Source) == "" || strings.TrimSpace(gate.Action) == "" || len(gate.Checks) == 0 || len(gate.Checks) > 8 {
			return errors.New("completion gate requires a repository source, action and public verification commands")
		}
		found := false
		for _, file := range plan.Files {
			if file == gate.Source {
				found = true
			}
		}
		if !found {
			return errors.New("completion gate source must be a fingerprinted repository input")
		}
		for _, command := range gate.Checks {
			if err := ValidateContainerCommand(command); err != nil {
				return err
			}
		}
	}
	seen := map[string]bool{}
	for _, variable := range plan.Environment {
		key, value := variable.Name, variable.Value
		if seen[key] {
			return fmt.Errorf("duplicate environment key %s", key)
		}
		seen[key] = true
		// These values belong to the boundary implementation, not model recipes.
		if !environmentKey.MatchString(key) || strings.ContainsAny(value, "\x00\r\n") || len(value) > 4096 || strings.HasPrefix(key, "GIT_") || strings.HasPrefix(key, "DOCKER_") || strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") || strings.Contains(key, "PASSWORD") || strings.Contains(key, "PROXY") || key == "HOME" || key == "TMPDIR" || key == "GOTMPDIR" || key == "GOTOOLCHAIN" {
			return fmt.Errorf("environment recipe cannot configure %s", key)
		}
	}
	return nil
}

func environmentInputs(dir string, plan domain.EnvironmentPlan) (map[string][]byte, error) {
	files := map[string][]byte{}
	total := 0
	for _, name := range plan.Files {
		if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00\r\n") || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
			return nil, errors.New("environment input must be a workspace-relative file")
		}
		for _, part := range strings.Split(name, "/") {
			lower := strings.ToLower(part)
			if lower == ".git" || lower == ".autopilot" || lower == ".forgeflow-runtime" || strings.HasPrefix(lower, ".env") || lower == ".npmrc" || lower == ".pypirc" || lower == "credentials" || lower == "auth.json" || lower == "settings.xml" {
				return nil, fmt.Errorf("environment input may contain credentials or runtime state: %s", name)
			}
		}
		p := filepath.Join(dir, filepath.FromSlash(name))
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Clean(real), filepath.Clean(p)) {
			return nil, errors.New("environment input crosses a link boundary")
		}
		if info.Size() > 4<<20 {
			return nil, errors.New("environment input exceeds 4 MiB")
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
		f.Close()
		if err != nil {
			return nil, err
		}
		total += len(b)
		if len(b) > 4<<20 || total > 16<<20 {
			return nil, errors.New("environment inputs exceed size bound")
		}
		files[name] = b
	}
	return files, nil
}

func (c *CLI) EnvironmentFingerprint(dir string, plan domain.EnvironmentPlan) (string, error) {
	if err := ValidateEnvironmentPlan(plan); err != nil {
		return "", err
	}
	files, err := environmentInputs(dir, plan)
	if err != nil {
		return "", err
	}
	// Explanatory wording is not a repair. Compare executable inputs so changing
	// a summary cannot authorize another identical build or contributor attempt.
	plan.Summary = ""
	plan.Requirements = nil
	plan.Blockers = nil
	plan.CompletionGates = append([]domain.CompletionGate(nil), plan.CompletionGates...)
	for i := range plan.CompletionGates {
		plan.CompletionGates[i].Requirement, plan.CompletionGates[i].Action = "", ""
	}
	plan.Files = append([]string(nil), plan.Files...)
	sort.Strings(plan.Files)
	plan.Environment = append([]domain.EnvironmentVariable(nil), plan.Environment...)
	sort.Slice(plan.Environment, func(i, j int) bool { return plan.Environment[i].Name < plan.Environment[j].Name })
	// JSON orders map keys, so this fingerprint is independent of map iteration.
	b, err := json.Marshal(struct {
		Plan  domain.EnvironmentPlan
		Files map[string][]byte
	}{plan, files})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func localImage(ctx context.Context, ref string) (string, error) {
	out, err := dockerControl(ctx, "image", "inspect", "--format", "{{.Id}}", ref)
	id := strings.TrimSpace(out)
	if err != nil || !immutableImage.MatchString(id) {
		if err != nil && strings.Contains(strings.ToLower(out), "no such image") {
			return "", fmt.Errorf("%w: %s", imageMissing, ref)
		}
		return "", fmt.Errorf("local image %s unavailable: %v; %s", ref, err, out)
	}
	return id, nil
}

func environmentDockerfile(base string, plan domain.EnvironmentPlan) (string, error) {
	if !immutableImage.MatchString(base) {
		return "", errors.New("managed image base must be an immutable local ID")
	}
	if err := ValidateEnvironmentPlan(plan); err != nil {
		return "", err
	}
	var recipe strings.Builder
	// BuildKit interprets FROM sha256:<config-ID> as a registry repository. Use
	// a locally pinned, content-addressed tag, verified against the ID by caller.
	fmt.Fprintf(&recipe, "FROM forgeflow-base:%s\nUSER 0:0\nWORKDIR /opt/forgeflow-inputs\n", strings.TrimPrefix(base, "sha256:"))
	if len(plan.Files) > 0 {
		recipe.WriteString("COPY inputs/ /opt/forgeflow-inputs/\n")
	}
	// JSON strings protect recipe boundaries; command arguments cannot add FROM,
	// privileged RUN options, mounts, host networking, or other Docker directives.
	for _, variable := range plan.Environment {
		value, _ := json.Marshal(variable.Value)
		fmt.Fprintf(&recipe, "ENV %s=%s\n", variable.Name, value)
	}
	for _, command := range plan.ImageCommands {
		args, _ := json.Marshal(append([]string{command.Program}, command.Arguments...))
		fmt.Fprintf(&recipe, "RUN %s\n", args)
	}
	recipe.WriteString("USER 1000:1000\nWORKDIR /workspace\n")
	return recipe.String(), nil
}

func buildEnvironmentImage(ctx context.Context, base, fingerprint, dir string, network bool, plan domain.EnvironmentPlan, report *domain.ExecutionEnvironment) (string, error) {
	sum := sha256.Sum256([]byte("environment-v1\x00" + base + "\x00" + fingerprint))
	tag := "forgeflow-managed:" + hex.EncodeToString(sum[:16])
	if id, err := localImage(ctx, tag); err == nil {
		report.Actions = append(report.Actions, "Reused matching immutable image "+id)
		return id, nil
	} else if !errors.Is(err, imageMissing) {
		return "", err
	}
	recipe, err := environmentDockerfile(base, plan)
	if err != nil {
		return "", err
	}
	baseTag := "forgeflow-base:" + strings.TrimPrefix(base, "sha256:")
	if id, e := localImage(ctx, baseTag); e == nil && id != base {
		return "", errors.New("managed base image tag changed unexpectedly")
	}
	if out, e := dockerControl(ctx, "tag", base, baseTag); e != nil {
		return "", fmt.Errorf("pin environment base image: %w; %s", e, out)
	}
	if id, e := localImage(ctx, baseTag); e != nil || id != base {
		return "", errors.New("managed base image identity could not be verified")
	}
	files, err := environmentInputs(dir, plan)
	if err != nil {
		return "", err
	}
	var input bytes.Buffer
	tw := tar.NewWriter(&input)
	filesWithRecipe := map[string][]byte{"Dockerfile": []byte(recipe)}
	for name, b := range files {
		filesWithRecipe["inputs/"+name] = b
	}
	for name, b := range filesWithRecipe {
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(b))}); err != nil {
			return "", err
		}
		if _, err = tw.Write(b); err != nil {
			return "", err
		}
	}
	if err = tw.Close(); err != nil {
		return "", err
	}
	networkMode := "none"
	if network {
		networkMode = "default"
	}
	cmd := exec.CommandContext(ctx, "docker", "build", "--network="+networkMode, "--pull=false", "--label", "forgeflow.managed=true", "--tag", tag, "-")
	cmd.Env, cmd.Stdin, cmd.WaitDelay = environment(), &input, 3*time.Second
	configureProcess(cmd)
	out := &bounded{limit: 32 << 10}
	cmd.Stdout, cmd.Stderr = out, out
	report.Actions = append(report.Actions, "Managed Dockerfile:\n"+recipe)
	err = cmd.Run()
	report.Actions = append(report.Actions, out.String())
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("environment image build failed: %w; %s", err, out.String())
	}
	if id, e := localImage(ctx, baseTag); e != nil || id != base {
		return "", errors.New("managed base image identity changed during build")
	}
	return localImage(ctx, tag)
}

func (c *CLI) PrepareEnvironment(ctx context.Context, dir string, network bool, plan domain.EnvironmentPlan) (report domain.ExecutionEnvironment, err error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report = domain.ExecutionEnvironment{Status: "checking", Network: network, Capabilities: []string{
		"Inspect repository and persisted environment/verification evidence",
		"Request ENVIRONMENT_BLOCKED for missing tools, versions, libraries, or services",
		"ForgeFlow provisions language-independent image commands and workspace dependencies in isolation",
		"Downloads require existing network approval; credentials and host services require operator input",
	}}
	key := filepath.Clean(dir)
	c.environmentMu.Lock()
	delete(c.environments, key)
	c.environmentMu.Unlock()
	defer func() {
		if err != nil {
			report.Status, report.Error = "blocked", err.Error()
			return
		}
		report.Status = "ready"
		c.environmentMu.Lock()
		defer c.environmentMu.Unlock()
		if c.environments == nil {
			c.environments = map[string]domain.ExecutionEnvironment{}
		}
		c.environments[key] = report
	}()
	if err = ctx.Err(); err != nil {
		return report, err
	}
	if c.VerificationImage == "" {
		return report, errors.New("environment provisioning requires explicitly configured container verification")
	}
	if err = ValidateEnvironmentPlan(plan); err != nil {
		return report, err
	}
	if _, err = containerArguments(c.VerificationImage, "preflight", dir, domain.VerificationCommand{Program: "sh"}, false); err != nil {
		return report, err
	}
	report.Fingerprint, err = c.EnvironmentFingerprint(dir, plan)
	if err != nil {
		return report, err
	}
	if len(plan.Blockers) > 0 {
		return report, fmt.Errorf("environment requirements need operator input: %s", strings.Join(plan.Blockers, "; "))
	}
	report.BaseImage, err = localImage(ctx, plan.BaseImage)
	if err != nil {
		if !errors.Is(err, imageMissing) {
			return report, err
		}
		if !network {
			return report, errors.New("environment base image is not cached; dependency download approval is required")
		}
		out, e := dockerControl(ctx, "pull", plan.BaseImage)
		report.Actions = append(report.Actions, "Pull "+plan.BaseImage+"\n"+out)
		if e != nil {
			return report, fmt.Errorf("environment image pull failed: %w; %s", e, out)
		}
		report.BaseImage, err = localImage(ctx, plan.BaseImage)
		if err != nil {
			return report, err
		}
	}
	// Inherited ONBUILD directives could add network fetches or mounts that do
	// not appear in the generated recipe. Require an ordinary Linux base.
	metadata, e := dockerControl(ctx, "image", "inspect", "--format", "{{.Os}} {{json .Config.OnBuild}}", report.BaseImage)
	if e != nil || (strings.TrimSpace(metadata) != "linux null" && strings.TrimSpace(metadata) != "linux []") {
		return report, errors.New("environment base must be a Linux image without inherited ONBUILD instructions")
	}
	report.Image, err = buildEnvironmentImage(ctx, report.BaseImage, report.Fingerprint, dir, network, plan, &report)
	if err != nil {
		return report, err
	}
	probe := &CLI{VerificationImage: report.Image}
	if err = probe.checkContainer(ctx, dir); err != nil {
		return report, err
	}
	run := func(command domain.VerificationCommand, online, hooks bool) error {
		rec, e := c.containerCommandMode(ctx, report.Image, dir, command, online, hooks)
		report.Checks = append(report.Checks, rec)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e != nil || rec.ExitCode != 0 {
			return fmt.Errorf("environment prerequisite failed: %s %s: %v; %s", command.Program, strings.Join(command.Arguments, " "), e, rec.Output)
		}
		return nil
	}
	// Even a cached image must pass current workspace checks. Mutable dependency
	// directories are not proof of readiness, so repeat explicit install commands.
	for _, command := range plan.WorkspaceCommands {
		if err = run(command, network, false); err != nil {
			return report, err
		}
	}
	for _, command := range plan.HookCommands {
		if err = run(command, network, true); err != nil {
			return report, err
		}
		if err = validateHookDirectory(filepath.Join(dir, ".git", "hooks")); err != nil {
			return report, err
		}
		if _, err = prepareHookChecksum(dir); err != nil {
			return report, err
		}
	}
	for _, command := range plan.Checks {
		if err = run(command, network, false); err != nil {
			return report, err
		}
	}
	if after, e := c.EnvironmentFingerprint(dir, plan); e != nil {
		return report, e
	} else if after != report.Fingerprint {
		return report, errors.New("environment preparation changed recipe inputs; inspect the preserved manifest changes before resuming")
	}
	return report, nil
}
