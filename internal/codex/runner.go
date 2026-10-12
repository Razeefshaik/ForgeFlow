// Package codex integrates bounded CLI sessions and sandboxed verification commands.
package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Request struct {
	Directory, Role, Prompt, Schema, OutputFile, Model string
	Network                                            bool
}
type Result struct {
	SessionID, Output string
	Usage             map[string]int64
}
type Runner interface {
	Run(context.Context, Request, func(json.RawMessage) error) (Result, error)
	Command(context.Context, string, domain.VerificationCommand, bool) (domain.TestRun, error)
	Check(context.Context, string, string) error
}
type CLI struct {
	Binary, Model string
	// Explicit, optional Linux image for deterministic verification only.
	// Model sessions continue to use the native sandbox and local authentication.
	VerificationImage string
	environmentMu     sync.Mutex
	environments      map[string]domain.ExecutionEnvironment
	modeMu            sync.RWMutex
	sandboxMode       string
}

// ConfigureProcess makes cancellation terminate the owned process tree.
func ConfigureProcess(cmd *exec.Cmd) { configureProcess(cmd) }

func Resolve(binary, model string) *CLI {
	if binary == "" {
		binary, _ = exec.LookPath("codex")
		if runtime.GOOS == "windows" {
			home, _ := os.UserHomeDir()
			candidate := filepath.Join(home, "AppData", "Roaming", "npm", "node_modules", "@openai", "codex", "node_modules", "@openai", "codex-win32-x64", "vendor", "x86_64-pc-windows-msvc", "bin", "codex.exe")
			if _, err := os.Stat(candidate); err == nil {
				binary = candidate
			} else if path, err := exec.LookPath("codex.exe"); err == nil {
				binary = path
			} else {
				binary = ""
			}
			binary = newestInstalledWindowsCLI(binary, home)
		}
	}
	return &CLI{Binary: binary, Model: model}
}
func (c *CLI) policy(readOnly, network bool) []string {
	filesystem := `{":root"="read",":workspace_roots"={"."="write"}}`
	if readOnly {
		filesystem = `{":root"="read"}`
	}
	args := []string{"-c", "permissions.forgeflow.filesystem=" + filesystem, "-c", fmt.Sprintf("permissions.forgeflow.network.enabled=%t", network), "-c", `approval_policy="never"`}
	if runtime.GOOS == "windows" {
		args = append(args, windowsPolicy(c.SandboxMode())...)
	}
	return args
}
func (c *CLI) Run(ctx context.Context, r Request, onEvent func(json.RawMessage) error) (Result, error) {
	var result Result
	if c.Binary == "" {
		return result, errors.New("Codex CLI unavailable; install it and run codex login")
	}
	if c.VerificationImage != "" && (r.Role == "contributor" || r.Role == "planner" || r.Role == "reviewer") {
		r.Prompt += "\nForgeFlow runs the plan's verification commands separately in a Linux container. Consult the supplied environment evidence for actual tools and versions; never assume a tool is installed. Use portable repository-relative commands. Native Windows tools are a different environment. Report ENVIRONMENT_BLOCKED when a prerequisite prevents progress; do not change required versions or tests to fit the runner."
	}
	readOnly := r.Role == "reviewer" || r.Role == "planner" || r.Role == "environment-planner" || r.Role == "operator" || r.Role == "assistant-reader"
	args := []string{"exec", "--ignore-user-config", "--ignore-rules", "--json", "--color", "never", "-C", r.Directory, "-c", `default_permissions="forgeflow"`, "-c", "mcp_servers={}", "-c", "features.multi_agent=false", "-c", `web_search="disabled"`}
	args = append(args, c.policy(readOnly, r.Network)...)
	if r.Role == "assistant-reader" || r.Role == "assistant-editor" {
		args = append(args, "--skip-git-repo-check")
	}
	if r.Role == "operator" {
		args = append(args, "--skip-git-repo-check")
		for _, feature := range []string{"shell_tool", "unified_exec", "apps", "plugins", "multi_agent", "browser_use", "computer_use", "image_generation", "view_image", "memories", "skill_search", "code_mode_host"} {
			args = append(args, "-c", "features."+feature+"=false")
		}
		args = append(args, "-c", "features.skip_host_skill_discovery=true")
	}
	model := r.Model
	if model == "" {
		model = c.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if r.Schema != "" {
		args = append(args, "--output-schema", r.Schema)
	}
	if r.OutputFile != "" {
		args = append(args, "--output-last-message", r.OutputFile)
	}
	args = append(args, "-")
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, c.Binary, args...)
	cmd.Dir = r.Directory
	if r.Role == "operator" || r.Role == "assistant-reader" {
		// Inspection must not create build/cache directories in the target folder.
		cmd.Env = approvedNetworkEnvironment(environment(), r.Network)
	} else {
		env, err := workspaceEnvironmentForNetwork(r.Directory, r.Network)
		if err != nil {
			return result, err
		}
		cmd.Env = env
	}
	cmd.Stdin = strings.NewReader(r.Prompt)
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	stderr := &bounded{limit: 32 << 10}
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return result, errors.New("could not start Codex CLI")
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	total := 0
	completed := false
	var streamErr error
	for scanner.Scan() {
		b := append([]byte{}, scanner.Bytes()...)
		total += len(b)
		if total > 16<<20 {
			streamErr = errors.New("Codex event output exceeded 16 MiB bound")
			cancel()
			break
		}
		var event struct {
			Type     string           `json:"type"`
			ThreadID string           `json:"thread_id"`
			Usage    map[string]int64 `json:"usage"`
			Item     struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if err = json.Unmarshal(b, &event); err != nil {
			streamErr = errors.New("Codex returned an invalid event stream")
			cancel()
			break
		}
		if event.Type == "thread.started" {
			result.SessionID = event.ThreadID
		}
		if event.Type == "turn.completed" {
			completed = true
			result.Usage = event.Usage
		}
		if event.Type == "turn.failed" || event.Type == "error" {
			streamErr = errors.New(eventFailure(b))
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			result.Output = event.Item.Text
		}
		if onEvent != nil {
			if e := onEvent(json.RawMessage(b)); e != nil {
				streamErr = e
				cancel()
				break
			}
		}
	}
	if e := scanner.Err(); e != nil {
		streamErr = errors.New("Codex event stream could not be read")
		cancel()
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if streamErr != nil {
		return result, streamErr
	}
	if err != nil {
		return result, fmt.Errorf("Codex exited unsuccessfully: %s", stderr.String())
	}
	if !completed || strings.TrimSpace(result.Output) == "" {
		return result, errors.New("Codex did not finish a response")
	}
	return result, nil
}

// Keep the transport's actual reason available to recovery classification;
// a generic "inspect output" error hides rate limits and authentication.
func eventFailure(raw []byte) string {
	var event struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(raw, &event)
	message := event.Message
	if message == "" && len(event.Error) > 0 {
		if json.Unmarshal(event.Error, &message) != nil {
			var detail struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(event.Error, &detail)
			message = detail.Message
		}
	}
	if message == "" {
		return "Codex session failed; inspect saved agent output"
	}
	if len(message) > 4000 {
		message = message[:4000] + " (truncated)"
	}
	return "Codex session failed: " + message
}
func (c *CLI) sandbox(ctx context.Context, dir string, command domain.VerificationCommand, network bool) (domain.TestRun, error) {
	rec := domain.TestRun{Command: command, Directory: dir, StartedAt: time.Now().UTC(), ExitCode: -1}
	args := []string{"sandbox", "--permission-profile", "forgeflow", "-C", dir}
	args = append(args, c.policy(false, network)...)
	args = append(args, "--", command.Program)
	args = append(args, command.Arguments...)
	cmd := exec.CommandContext(ctx, c.Binary, args...)
	cmd.Dir = dir
	env, err := workspaceEnvironmentForNetwork(dir, network)
	if err != nil {
		rec.FinishedAt = time.Now().UTC()
		return rec, err
	}
	cmd.Env = env
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	out := &bounded{limit: 256 << 10}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	rec.FinishedAt = time.Now().UTC()
	if cmd.ProcessState != nil {
		rec.ExitCode = cmd.ProcessState.ExitCode()
	}
	rec.Output = out.String()
	rec.Truncated = out.truncated
	if ctx.Err() != nil {
		return rec, ctx.Err()
	}
	return rec, err
}
func (c *CLI) Command(ctx context.Context, dir string, command domain.VerificationCommand, network bool) (domain.TestRun, error) {
	if c.VerificationImage != "" {
		if err := ValidateContainerCommand(command); err != nil {
			return domain.TestRun{}, err
		}
		rec, err := c.containerCommand(ctx, dir, boundedGoTest(command), network)
		rec.Command = command
		return rec, err
	}
	if err := ValidateCommand(command); err != nil {
		return domain.TestRun{}, err
	}
	resolved, err := resolveCommand(boundedGoTest(command), dir)
	if err != nil {
		return domain.TestRun{}, err
	}
	rec, err := c.sandbox(ctx, dir, resolved, network)
	rec.Command = command
	rec.ExecutedCommand = &resolved
	return rec, err
}

// Check proves both an allowed workspace write and denied writes to sibling metadata
// and the application directory, using a harmless marker before any repository script runs.
func (c *CLI) CheckTimeout() time.Duration {
	if c.VerificationImage != "" {
		return 45*time.Second + containerCheckTimeout
	}
	return 45 * time.Second
}

func (c *CLI) Check(ctx context.Context, dir, appRoot string) error {
	nativeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	err := c.checkWithNetwork(nativeCtx, dir, appRoot, false)
	cancel()
	if err != nil {
		return err
	}
	if c.VerificationImage != "" {
		containerCtx, stop := context.WithTimeout(ctx, containerCheckTimeout)
		defer stop()
		return c.checkContainer(containerCtx, dir)
	}
	return nil
}

func (c *CLI) checkWithNetwork(ctx context.Context, dir, appRoot string, network bool) error {
	if c.Binary == "" {
		return errors.New("Codex CLI unavailable; install it and run codex login")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return errors.New("active workspace must be a Git repository")
	}
	marker := fmt.Sprintf(".forgeflow-sandbox-probe-%d", time.Now().UnixNano())
	inside := filepath.Join(dir, marker)
	outside := filepath.Join(filepath.Dir(dir), marker)
	control := filepath.Join(appRoot, ".cache", marker)
	if err := os.MkdirAll(filepath.Dir(control), 0700); err != nil {
		return err
	}
	defer os.Remove(inside)
	var probe domain.VerificationCommand
	if runtime.GOOS == "windows" {
		// Cmdlets also work in Windows constrained language mode. Positive and negative
		// outcomes are checked independently; a runtime error cannot masquerade as isolation.
		quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
		script := "Set-Content -LiteralPath " + quote(inside) + " -Value allowed -ErrorAction Stop; try { Set-Content -LiteralPath " + quote(outside) + " -Value forbidden -ErrorAction Stop; exit 9 } catch {}; try { Set-Content -LiteralPath " + quote(control) + " -Value forbidden -ErrorAction Stop; exit 9 } catch {}; exit 0"
		probe = domain.VerificationCommand{Program: "powershell.exe", Arguments: []string{"-NoProfile", "-NonInteractive", "-Command", script}}
	} else {
		probe = domain.VerificationCommand{Program: "python3", Arguments: []string{"-c", "import os,sys;open(sys.argv[1],'w').write('allowed');\nfor path in sys.argv[2:]:\n try: open(path,'w').write('forbidden');sys.exit(9)\n except PermissionError: pass", inside, outside, control}}
	}
	rec, err := c.sandbox(ctx, dir, probe, network)
	if err != nil || rec.ExitCode != 0 {
		if err != nil {
			return fmt.Errorf("sandbox environment check could not run: %w; %s", err, rec.Output)
		}
		return errors.New("sandbox environment check could not run: " + rec.Output)
	}
	if _, err = os.Stat(inside); err != nil {
		return errors.New("sandbox could not write inside active workspace")
	}
	for _, path := range []string{outside, control} {
		if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("sandbox permitted writes outside active workspace; execution refused")
		}
	}
	return nil
}

// ProbeGitShell is a short capability check after an actual Git-shell failure.
// It is not a global prerequisite: repositories that do not use MSYS can run.
func (c *CLI) ProbeGitShell(ctx context.Context, dir string) (domain.TestRun, error) {
	return c.probeGitShellWithNetwork(ctx, dir, false)
}

func (c *CLI) probeGitShellWithNetwork(ctx context.Context, dir string, network bool) (domain.TestRun, error) {
	probe := domain.VerificationCommand{Program: "git", Arguments: []string{"-c", "core.fsmonitor=false", "-c", "alias.forgeflow-runtime-probe=!exit 0", "forgeflow-runtime-probe"}}
	if c.VerificationImage != "" {
		return c.containerCommand(ctx, dir, probe, network)
	}
	return c.sandbox(ctx, dir, probe, network)
}

// CommandValidationError gives the planner actionable feedback without ever
// executing a rejected command or dropping a required check.
type CommandValidationError struct {
	Program  string
	Argument int
	Reason   string
}

func (e *CommandValidationError) Error() string {
	return fmt.Sprintf("invalid verification command %q argument %d: %s", e.Program, e.Argument, e.Reason)
}

func ValidateCommand(c domain.VerificationCommand) error {
	if err := validateCommandTimeout(c); err != nil {
		return err
	}
	allowed := map[string]bool{"go": true, "python": true, "python3": true, "pytest": true, "npm": true, "node": true, "mvn": true, "./mvnw": true, "gradle": true, "./gradlew": true}
	if !allowed[c.Program] || len(c.Arguments) > 30 {
		return &CommandValidationError{c.Program, -1, "program must be allowlisted and have at most 30 arguments"}
	}
	for i, a := range c.Arguments {
		unsafe := a
		if c.Program == "go" && len(c.Arguments) > 0 && c.Arguments[0] == "test" {
			pattern, filter := "", false
			for _, flag := range []string{"-run", "-bench", "-skip"} {
				if i > 0 && c.Arguments[i-1] == flag {
					pattern, filter = a, true
				} else if strings.HasPrefix(a, flag+"=") {
					pattern, filter = strings.TrimPrefix(a, flag+"="), true
				} else if a == flag && i+1 == len(c.Arguments) {
					return &CommandValidationError{c.Program, i, "Go test filter requires a regular expression"}
				}
			}
			if filter {
				if _, err := regexp.Compile(pattern); err != nil {
					return &CommandValidationError{c.Program, i, "invalid Go test regular expression: " + err.Error()}
				}
				// Go receives native argv, so alternation cannot start a shell pipeline.
				unsafe = strings.ReplaceAll(a, "|", "")
			}
		}
		if len(a) > 300 || strings.ContainsAny(unsafe, "\r\n\x00&|<>%!`") || ((c.Program == "./mvnw" || c.Program == "./gradlew" || c.Program == "mvn" || c.Program == "gradle") && strings.ContainsAny(a, "^$")) {
			return &CommandValidationError{c.Program, i, "argument exceeds 300 characters or contains forbidden shell/control characters"}
		}
	}
	return nil
}

func validateCommandTimeout(c domain.VerificationCommand) error {
	if c.TimeoutSeconds < 0 || c.TimeoutSeconds > 7200 {
		return &CommandValidationError{c.Program, -1, "timeout_seconds must be zero (default) or at most 7200"}
	}
	return nil
}
func resolveCommand(c domain.VerificationCommand, dir string) (domain.VerificationCommand, error) {
	if runtime.GOOS == "windows" && c.Program == "npm" {
		node, err := exec.LookPath("node.exe")
		if err != nil {
			return c, errors.New("Node is unavailable")
		}
		candidates := []string{os.Getenv("npm_execpath"), filepath.Join(filepath.Dir(node), "node_modules", "npm", "bin", "npm-cli.js")}
		for _, path := range candidates {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return domain.VerificationCommand{Program: node, Arguments: append([]string{path}, c.Arguments...)}, nil
			}
		}
		return c, errors.New("npm CLI could not be resolved")
	}
	if runtime.GOOS == "windows" && (c.Program == "./mvnw" || c.Program == "./gradlew") {
		script := strings.TrimPrefix(c.Program, "./") + ".cmd"
		if c.Program == "./gradlew" {
			script = "gradlew.bat"
		}
		script = `.\` + script // Explicit workspace path also works when CMD excludes cwd from executable lookup.
		return domain.VerificationCommand{Program: "cmd.exe", Arguments: append([]string{"/d", "/c", script}, c.Arguments...)}, nil
	}
	return c, nil
}
func environment() []string {
	all := []string{}
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		key := strings.ToUpper(k)
		if strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") || strings.Contains(key, "PASSWORD") || strings.HasSuffix(key, "_KEY") || key == "SSH_AUTH_SOCK" || strings.HasPrefix(key, "GIT_") || key == "FORGEFLOW_CODEX_MODEL" || strings.HasPrefix(key, "FORGEFLOW_TEST_") {
			continue
		}
		all = append(all, e)
	}
	return all
}
func workspaceEnvironment(dir string) []string {
	env, _ := workspaceEnvironmentForNetwork(dir, false)
	return env
}

// PrepareWorkspaceRuntime verifies that child tools can actually write their
// temporary files before a contribution spends time planning or coding.
func PrepareWorkspaceRuntime(dir string) error {
	if _, err := workspaceEnvironmentForNetwork(dir, false); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Join(dir, ".forgeflow-runtime", "tmp"), "preflight-*")
	if err != nil {
		return fmt.Errorf("workspace runtime cache is not writable: %w", err)
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}
func workspaceEnvironmentForNetwork(dir string, network bool) ([]string, error) {
	base := filepath.Join(dir, ".forgeflow-runtime")
	if err := prepareWorkspaceCaches(dir); err != nil {
		return nil, err
	}
	overrides := map[string]string{"GRADLE_USER_HOME": filepath.Join(base, "gradle"), "TMP": filepath.Join(base, "tmp"), "TEMP": filepath.Join(base, "tmp"), "TMPDIR": filepath.Join(base, "tmp"), "GOTMPDIR": filepath.Join(base, "tmp"), "GOCACHE": filepath.Join(base, "go-cache"), "GOMODCACHE": filepath.Join(base, "go-mod"), "GOPATH": filepath.Join(base, "go"), "npm_config_cache": filepath.Join(base, "npm"), "PIP_CACHE_DIR": filepath.Join(base, "python"), "UV_CACHE_DIR": filepath.Join(base, "python"), "XDG_CACHE_HOME": filepath.Join(base, "xdg"), "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0"}
	result := []string{}
	for _, entry := range approvedNetworkEnvironment(environment(), network) {
		key, _, _ := strings.Cut(entry, "=")
		skip := false
		for name := range overrides {
			if strings.EqualFold(name, key) {
				skip = true
			}
		}
		if !skip {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result, nil
}

func prepareWorkspaceCaches(dir string) error {
	paths := []string{filepath.Join(dir, ".forgeflow-runtime")}
	for _, part := range []string{"tmp", "go-cache", "go-mod", "go", "npm", "python", "xdg", "gradle"} {
		paths = append(paths, filepath.Join(dir, ".forgeflow-runtime", part))
	}
	for _, path := range paths {
		if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return errors.New("workspace runtime cache is not writable")
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("workspace runtime cache must be an independent directory")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(path)) {
			return errors.New("workspace runtime cache crosses a link boundary")
		}
	}
	return nil
}

type bounded struct {
	mu        sync.Mutex
	buf       strings.Builder
	limit     int
	truncated bool
	tail      []byte
}

func (b *bounded) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	// Preserve the final error/timeout stack after verbose build output. Input
	// remains bounded; discarded middle output is explicitly labeled.
	tailLimit := b.limit / 2
	if tailLimit > 0 {
		if n >= tailLimit {
			b.tail = append(b.tail[:0], p[n-tailLimit:]...)
		} else {
			excess := len(b.tail) + n - tailLimit
			if excess > 0 {
				copy(b.tail, b.tail[excess:])
				b.tail = b.tail[:len(b.tail)-excess]
			}
			b.tail = append(b.tail, p...)
		}
	}
	left := b.limit - b.buf.Len()
	if len(p) > left {
		p = p[:left]
		b.truncated = true
	}
	b.buf.Write(p)
	return n, nil
}
func (b *bounded) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.truncated && b.limit >= 128 {
		return b.buf.String()[:b.limit/2] + "\n[output truncated; showing beginning and end]\n" + string(b.tail)
	}
	return b.buf.String()
}

var _ io.Writer = (*bounded)(nil)
