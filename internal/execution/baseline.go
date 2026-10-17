package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Upstream repositories often carry failures that have nothing to do with an
// issue: flaky or environment-specific tests, lint debt, suites needing absent
// services. Without a measured baseline those failures send the contributor
// into fix loops that end at the iteration limit. ForgeFlow therefore runs the
// required commands once on the untouched approved base, before coding, and
// accepts a post-change failure only when real evidence shows the base failed
// the same way. Accepted failures are always disclosed to the reviewer, the
// report and the PR text; they are never reported as passes.

type baselineKey struct{}

func withBaseline(ctx context.Context) context.Context {
	return context.WithValue(ctx, baselineKey{}, true)
}

func isBaseline(ctx context.Context) bool {
	v, _ := ctx.Value(baselineKey{}).(bool)
	return v
}

func commandKey(c domain.VerificationCommand) string {
	return c.Program + "\x00" + strings.Join(c.Arguments, "\x00")
}

var (
	goFailure      = regexp.MustCompile(`(?m)^\s*--- FAIL: (\S+)`)
	pytestFailure  = regexp.MustCompile(`(?m)^(?:FAILED|ERROR) (\S+)`)
	unittestFail   = regexp.MustCompile(`(?m)^(?:FAIL|ERROR): (\S+) \(([^)\s]+)\)`)
	gradleFailure  = regexp.MustCompile(`(?m)^\s*([\w.$]+) > (.+?) FAILED\s*$`)
	surefireFail   = regexp.MustCompile(`(?m)^\[ERROR\]\s+([\w.$\[\]]+?)(?:\(([\w.$]+)\))?\s+(?:--\s+)?Time elapsed:[^\n]*<<<\s*(?:FAILURE|ERROR)!`)
	cargoFailure   = regexp.MustCompile(`(?m)^test (\S+) \.\.\. FAILED\s*$`)
	jestFailure    = regexp.MustCompile(`(?m)^\s*● (.+?)\s*$`)
	tapFailure     = regexp.MustCompile(`(?m)^\s*not ok \d+ - (.+?)\s*$`)
	volatileTokens = regexp.MustCompile(`\b\d+(?:\.\d+)?\s?(?:ns|us|µs|ms|s|sec|secs|seconds|m|min|h)\b|0x[0-9a-fA-F]+|\b\d{4}-\d{2}-\d{2}[T ][\d:.]+Z?\b|\b\d{2}:\d{2}:\d{2}(?:\.\d+)?\b|/tmp/[\w.-]+`)
	errorLine      = regexp.MustCompile(`(?i)\b(?:error|errors|fail|failed|failure|failures|exception|panic)\b|^\S+\.\w+:\d+(?::\d+)?: `)
)

const maxFailureNames = 200

// failedTestNames extracts failing test identifiers from common runners. The
// second result is false when nothing identifiable was found or the list was
// too long to compare reliably.
func failedTestNames(output string) ([]string, bool) {
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 300 {
			return
		}
		seen[name] = true
	}
	for _, m := range goFailure.FindAllStringSubmatch(output, -1) {
		add("go:" + m[1])
	}
	for _, m := range pytestFailure.FindAllStringSubmatch(output, -1) {
		add("pytest:" + m[1])
	}
	for _, m := range unittestFail.FindAllStringSubmatch(output, -1) {
		// Python 3.11+ prints the full id in parentheses; older versions the class.
		if strings.HasSuffix(m[2], "."+m[1]) {
			add("unittest:" + m[2])
		} else {
			add("unittest:" + m[2] + "." + m[1])
		}
	}
	for _, m := range gradleFailure.FindAllStringSubmatch(output, -1) {
		add("junit:" + m[1] + " > " + m[2])
	}
	for _, m := range surefireFail.FindAllStringSubmatch(output, -1) {
		if m[2] != "" {
			add("junit:" + m[2] + "." + m[1])
		} else {
			add("junit:" + m[1])
		}
	}
	for _, m := range cargoFailure.FindAllStringSubmatch(output, -1) {
		add("cargo:" + m[1])
	}
	for _, m := range jestFailure.FindAllStringSubmatch(output, -1) {
		add("jest:" + m[1])
	}
	for _, m := range tapFailure.FindAllStringSubmatch(output, -1) {
		add("tap:" + m[1])
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 || len(names) > maxFailureNames {
		return names, false
	}
	return names, true
}

func noTestsRan(run domain.TestRun) bool {
	if run.ExitCode == 0 {
		return false
	}
	text := strings.ToLower(run.Output)
	return hasAny(text, "no tests ran", "collected 0 items", "no tests found", "no test files found", "no tests to run")
}

// buildLevelFailure recognizes output where the command never reached its
// checks. Such failures cannot be accepted by signature, because accepting
// them would mean the patch was never verified at all.
func buildLevelFailure(output string) bool {
	text := strings.ToLower(output)
	return hasAny(text, "build failed", "compilation failed", "compilation error", "could not compile", "failed to compile", "cannot find symbol", "could not resolve", "could not find", "does not compile", "syntaxerror", "modulenotfounderror", "importerror", "no such file or directory", "command not found", "permission denied", "undefined:", "cannot use ", "error: linking", "execution failed for task", "unable to", "no space left")
}

func failureSignature(output string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || len(line) > 500 || !errorLine.MatchString(line) {
			continue
		}
		seen[volatileTokens.ReplaceAllString(line, "<v>")] = true
		if len(seen) > 400 {
			return nil
		}
	}
	lines := make([]string, 0, len(seen))
	for line := range seen {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

func subset(items, of []string) bool {
	set := map[string]bool{}
	for _, v := range of {
		set[v] = true
	}
	for _, v := range items {
		if !set[v] {
			return false
		}
	}
	return true
}

// preexistingReason decides, from two real runs, whether a post-change
// failure already existed on the base. An empty reason means it is not proven.
func preexistingReason(base domain.BaselineCommand, baseRun, run domain.TestRun) string {
	if base.Passed || run.ExitCode == 0 {
		return ""
	}
	if base.NoTests && noTestsRan(run) && run.ExitCode == base.ExitCode {
		return "No tests were collected, exactly as on the untouched base."
	}
	post, parsed := failedTestNames(run.Output)
	if parsed && len(base.FailedTests) > 0 && len(base.FailedTests) <= maxFailureNames {
		if subset(post, base.FailedTests) {
			return "Every failing test also failed on the untouched base."
		}
		return ""
	}
	if len(post) > 0 || len(base.FailedTests) > 0 {
		// Identifiable failures that cannot be compared are never accepted.
		return ""
	}
	if buildLevelFailure(baseRun.Output) || buildLevelFailure(run.Output) {
		return ""
	}
	baseSignature, postSignature := failureSignature(baseRun.Output), failureSignature(run.Output)
	if len(baseSignature) == 0 || len(postSignature) == 0 || !subset(postSignature, baseSignature) {
		return ""
	}
	return "Every reported error line also appeared on the untouched base (normalized output comparison)."
}

// bareTestName strips runner prefixes and containers: "junit:pkg.FooTest >
// bar()" becomes "bar", "pytest:tests/a.py::test_x" becomes "test_x".
func bareTestName(name string) string {
	if _, rest, ok := strings.Cut(name, ":"); ok {
		name = rest
	}
	for _, sep := range []string{" > ", "::", "/", "."} {
		if i := strings.LastIndex(name, sep); i >= 0 && i+len(sep) < len(name) {
			name = name[i+len(sep):]
		}
	}
	return strings.TrimSuffix(strings.TrimSpace(name), "()")
}

var (
	javaPackage = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;?\s*$`)
	jestFile    = regexp.MustCompile(`(?m)^\s*FAIL\s+(\S+\.(?:js|jsx|ts|tsx|mjs|cjs))\s*$`)
)

// sourcePackages finds JVM test sources named class (Gradle prints simple
// class names) and returns their declared packages. The walk is bounded and
// skips Git metadata, build outputs and dependency trees.
func sourcePackages(repo, class string) []string {
	if repo == "" || class == "" || strings.ContainsAny(class, `/\`) {
		return nil
	}
	packages := []string{}
	visited := 0
	_ = filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		visited++
		if visited > 50000 {
			return filepath.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".forgeflow-runtime", "build", "target", "node_modules", ".gradle", "out":
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		ext := filepath.Ext(base)
		if strings.TrimSuffix(base, ext) != class || (ext != ".java" && ext != ".kt" && ext != ".groovy" && ext != ".scala") {
			return nil
		}
		if info, e := d.Info(); e != nil || info.Size() > 1<<20 {
			return nil
		}
		if b, e := os.ReadFile(path); e == nil {
			if m := javaPackage.FindSubmatch(b); m != nil {
				packages = append(packages, string(m[1]))
			} else {
				packages = append(packages, "")
			}
		}
		return nil
	})
	return packages
}

// testLocations returns repository directories that a failing test belongs to,
// when the runner output or the repository sources identify them. A "*"
// prefix marks a package path matched as a directory suffix.
func testLocations(repo, repoModule string, names []string, output string) []string {
	dirs := []string{}
	for _, name := range names {
		kind, rest, _ := strings.Cut(name, ":")
		switch kind {
		case "pytest":
			file, _, _ := strings.Cut(rest, "::")
			if strings.HasSuffix(file, ".py") {
				dir := "."
				if i := strings.LastIndex(file, "/"); i >= 0 {
					dir = file[:i]
				}
				dirs = append(dirs, dir)
			}
		case "unittest":
			// pkg.tests.test_mod.Class.method lives in pkg/tests.
			parts := strings.Split(rest, ".")
			if len(parts) >= 3 {
				dir := strings.Join(parts[:len(parts)-3], "/")
				if dir == "" {
					dir = "."
				}
				dirs = append(dirs, dir)
			}
		case "junit":
			// The class is the last segment starting with an upper-case letter:
			// com.example.FooTest, com.example.FooTest.method or FooTest.
			class, _, _ := strings.Cut(rest, " > ")
			parts := strings.Split(class, ".")
			k := -1
			for i, part := range parts {
				if part != "" && part[0] >= 'A' && part[0] <= 'Z' {
					k = i
				}
			}
			if k > 0 {
				dirs = append(dirs, "*"+strings.Join(parts[:k], "/"))
				continue
			}
			if k < 0 {
				continue
			}
			// Gradle prints simple class names; read the source's package.
			for _, pkg := range sourcePackages(repo, parts[0]) {
				if pkg == "" {
					dirs = append(dirs, ".")
				} else {
					dirs = append(dirs, "*"+strings.ReplaceAll(pkg, ".", "/"))
				}
			}
		}
	}
	for _, m := range jestFile.FindAllStringSubmatch(output, -1) {
		dir := "."
		if i := strings.LastIndex(m[1], "/"); i >= 0 {
			dir = m[1][:i]
		}
		dirs = append(dirs, dir)
	}
	if repoModule != "" {
		for _, m := range failedGoPackage.FindAllStringSubmatch(output, -1) {
			if m[1] == repoModule {
				dirs = append(dirs, ".")
			} else if strings.HasPrefix(m[1], repoModule+"/") {
				dirs = append(dirs, strings.TrimPrefix(m[1], repoModule+"/"))
			}
		}
	}
	return dirs
}

// relatedToPatch keeps acceptance to failures far from the change. A failure
// named by the issue or diff, or located where the patch edits, may be the
// very behavior under repair and must go through the normal fix loop. Names
// match whole words, so an unrelated test_parse is not "named" by a new
// test_parse_empty.
func relatedToPatch(names []string, output, context, repo, repoModule string, changed []string) bool {
	for _, name := range names {
		bare := bareTestName(name)
		if len(bare) < 4 {
			continue
		}
		if regexp.MustCompile(`(?i)(?:^|[^\w])` + regexp.QuoteMeta(bare) + `(?:$|[^\w])`).MatchString(context) {
			return true
		}
	}
	for _, dir := range testLocations(repo, repoModule, names, output) {
		for _, file := range changed {
			fileDir := "."
			if i := strings.LastIndex(file, "/"); i >= 0 {
				fileDir = file[:i]
			}
			if strings.HasPrefix(dir, "*") {
				if strings.HasSuffix("/"+fileDir, "/"+strings.TrimPrefix(dir, "*")) {
					return true
				}
			} else if fileDir == dir {
				return true
			}
		}
	}
	return false
}

func goModule(repo string) string {
	path := filepath.Join(repo, "go.mod")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := goModuleName.FindStringSubmatch(string(b)); len(m) == 2 {
		return m[1]
	}
	return ""
}

func latestRun(tests []domain.TestRun, command domain.VerificationCommand, baseline bool) (domain.TestRun, bool) {
	key := commandKey(command)
	for i := len(tests) - 1; i >= 0; i-- {
		if tests[i].Baseline == baseline && commandKey(tests[i].Command) == key {
			return tests[i], true
		}
	}
	return domain.TestRun{}, false
}

func testRunByID(tests []domain.TestRun, id string) (domain.TestRun, bool) {
	for _, run := range tests {
		if run.ID == id {
			return run, true
		}
	}
	return domain.TestRun{}, false
}

// baselineVerification runs once per approved base, before the first edit.
// Environment failures stop here, before any coding session is spent;
// repository failures are recorded and execution continues.
func (s *Service) baselineVerification(ctx context.Context, c domain.Contribution, r *domain.ExecutionRecord, repo string) error {
	if r.Baseline != nil && r.Baseline.BaseCommit == c.BaseCommit && r.Baseline.Status != "" {
		return nil
	}
	_, files, err := s.diff(ctx, repo, c.BaseCommit)
	if err != nil {
		return err
	}
	baseline := &domain.BaselineVerification{BaseCommit: c.BaseCommit, Commands: []domain.BaselineCommand{}}
	if len(files) > 0 {
		baseline.Status = "unavailable"
		baseline.Reason = "The workspace already differed from the approved base before coding, so the base could not be measured in place."
		baseline.RecordedAt = time.Now().UTC()
		r.Baseline = baseline
		return s.Store.SaveExecution(ctx, *r, "execution", "Baseline verification unavailable: workspace already modified")
	}
	commands, err := verificationCommands(repo, r.Plan, s.validateCommand)
	if err != nil {
		return err
	}
	r.Message = "Running required verification on the untouched approved base before coding"
	if err = s.Store.WorkspaceEvent(ctx, c.ID, "BaselineVerificationStarted", r.Message, map[string]any{"base_commit": c.BaseCommit, "commands": commands}); err != nil {
		return err
	}
	if err = s.Store.SaveExecution(ctx, *r, "execution", r.Message); err != nil {
		return err
	}
	var all strings.Builder
	bctx := withBaseline(ctx)
	failures := 0
	for _, command := range commands {
		passed, err := s.verifyWithRecovery(bctx, c.ID, repo, command, r, &all)
		if err != nil {
			return err
		}
		tests, err := s.Store.Tests(ctx, c.ID)
		if err != nil {
			return err
		}
		run, found := latestRun(tests, command, true)
		if !found {
			return fmt.Errorf("baseline evidence for %s was not saved", command.Program)
		}
		entry := domain.BaselineCommand{Command: command, TestRunID: run.ID, ExitCode: run.ExitCode, Passed: passed}
		if !passed {
			failures++
			entry.FailedTests, _ = failedTestNames(run.Output)
			entry.NoTests = noTestsRan(run)
		}
		baseline.Commands = append(baseline.Commands, entry)
	}
	baseline.Status = "passed"
	summary := "Baseline verification passed on the untouched approved base"
	if failures > 0 {
		baseline.Status = "preexisting_failures"
		summary = fmt.Sprintf("Baseline verification: %d of %d required commands already fail on the untouched approved base", failures, len(commands))
	}
	baseline.RecordedAt = time.Now().UTC()
	r.Baseline = baseline
	// A failure on the untouched base describes the repository, not a patch.
	if r.Incident != nil && (r.Incident.Kind == "code_failure" || r.Incident.Kind == "test_failure_timeout") {
		r.Incident = nil
	}
	r.Message = summary
	if err = s.Store.WorkspaceEvent(ctx, c.ID, "BaselineVerificationFinished", summary, baseline); err != nil {
		return err
	}
	return s.Store.SaveExecution(ctx, *r, "execution", summary)
}

// acceptPreexisting records a post-change failure as pre-existing when the
// baseline proves it. It never alters the saved run or its exit code.
func (s *Service) acceptPreexisting(ctx context.Context, id string, command domain.VerificationCommand, r *domain.ExecutionRecord) (bool, error) {
	if r.Baseline == nil || r.Baseline.Status != "preexisting_failures" {
		return false, nil
	}
	var base *domain.BaselineCommand
	for i := range r.Baseline.Commands {
		if commandKey(r.Baseline.Commands[i].Command) == commandKey(command) {
			base = &r.Baseline.Commands[i]
		}
	}
	if base == nil || base.Passed {
		return false, nil
	}
	tests, err := s.Store.Tests(ctx, id)
	if err != nil {
		return false, err
	}
	run, found := latestRun(tests, command, false)
	baseRun, baseFound := testRunByID(tests, base.TestRunID)
	if !found || !baseFound || run.FailureKind != "code_failure" {
		return false, nil
	}
	reason := preexistingReason(*base, baseRun, run)
	if reason == "" {
		return false, nil
	}
	failed, _ := failedTestNames(run.Output)
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return false, err
	}
	approval, err := s.Store.Approval(ctx, id)
	if err != nil {
		return false, err
	}
	repo, _, err := s.Paths(c)
	if err != nil {
		return false, err
	}
	diff, changed, err := s.diff(ctx, repo, c.BaseCommit)
	if err != nil {
		return false, err
	}
	if relatedToPatch(failed, run.Output, string(effectiveIssue(approval, r))+"\n"+diff, repo, goModule(repo), changed) {
		if err = s.Store.WorkspaceEvent(ctx, id, "PreexistingFailureRejected", "A failure that also occurs on the base is named by the issue or patch, or sits where the patch edits; it must be fixed", map[string]any{"command": command, "test_run_id": run.ID, "failed_tests": failed}); err != nil {
			return false, err
		}
		return false, nil
	}
	accepted := domain.PreexistingFailure{Command: command, TestRunID: run.ID, BaselineRun: base.TestRunID, ExitCode: run.ExitCode, FailedTests: failed, Reason: reason}
	r.Preexisting = append(r.Preexisting, accepted)
	if r.Incident != nil && r.Incident.TestRunID == run.ID {
		r.Incident = nil
	}
	if err = s.Store.WorkspaceEvent(ctx, id, "PreexistingFailureAccepted", "A required command fails exactly as it did on the untouched base; disclosed, not counted as a pass", accepted); err != nil {
		return false, err
	}
	return true, s.Store.SaveExecution(ctx, *r, "execution", "Recorded a pre-existing failure with baseline evidence")
}

func displayCommand(c domain.VerificationCommand) string {
	return strings.TrimSpace(c.Program + " " + strings.Join(c.Arguments, " "))
}

// baselinePrompt gives agents the measured base results so a contributor does
// not chase unrelated failures and a reviewer can challenge an acceptance.
func baselinePrompt(r *domain.ExecutionRecord, reviewer bool) string {
	if r.Baseline == nil || r.Baseline.Status == "" {
		return ""
	}
	type entry struct {
		Command     string   `json:"command"`
		Passed      bool     `json:"passed_on_base"`
		ExitCode    int      `json:"exit_code"`
		FailedTests []string `json:"failed_tests_on_base,omitempty"`
		NoTests     bool     `json:"no_tests_on_base,omitempty"`
	}
	entries := []entry{}
	for _, cmd := range r.Baseline.Commands {
		names := cmd.FailedTests
		if len(names) > 40 {
			names = append(append([]string{}, names[:40]...), fmt.Sprintf("... %d more in the saved baseline record", len(cmd.FailedTests)-40))
		}
		entries = append(entries, entry{displayCommand(cmd.Command), cmd.Passed, cmd.ExitCode, names, cmd.NoTests})
	}
	b, _ := json.Marshal(entries)
	text := "\nBaseline verification (real runs on the untouched approved base " + promptExcerpt(r.Baseline.BaseCommit, 64) + ", before any edit; status " + r.Baseline.Status + "): " + string(b) + "\n"
	if r.Baseline.Status == "unavailable" {
		text += "Baseline unavailable: " + r.Baseline.Reason + "\n"
	}
	if reviewer {
		if len(r.Preexisting) > 0 {
			accepted, _ := json.Marshal(r.Preexisting)
			text += "Post-change failures accepted as pre-existing (they fail the same way on the base; they are not passes): " + string(accepted) + "\nChallenge each acceptance: return REQUEST_CHANGES if the patch plausibly causes or masks any of these failures.\n"
		}
	} else if r.Baseline.Status == "preexisting_failures" {
		text += "Failures that already exist on the base are not yours to fix: do not change unrelated code or tests to make them pass. Your change must not add new failures. Implement the issue and add meaningful tests that pass.\n"
	}
	return text
}

func baselineReport(r domain.ExecutionRecord) string {
	if r.Baseline == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Baseline on the approved base\nStatus: %s\n", r.Baseline.Status)
	if r.Baseline.Reason != "" {
		b.WriteString(r.Baseline.Reason + "\n")
	}
	for _, cmd := range r.Baseline.Commands {
		outcome := "passed"
		if !cmd.Passed {
			outcome = fmt.Sprintf("failed (exit %d)", cmd.ExitCode)
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", displayCommand(cmd.Command), outcome)
	}
	if len(r.Preexisting) > 0 {
		b.WriteString("\n## Pre-existing failures (not counted as passes)\n")
		for _, p := range r.Preexisting {
			fmt.Fprintf(&b, "- `%s` exit %d: %s", displayCommand(p.Command), p.ExitCode, p.Reason)
			if len(p.FailedTests) > 0 {
				names := p.FailedTests
				if len(names) > 20 {
					names = append(append([]string{}, names[:20]...), fmt.Sprintf("and %d more", len(p.FailedTests)-20))
				}
				fmt.Fprintf(&b, " Failing tests: %s.", strings.Join(names, ", "))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
