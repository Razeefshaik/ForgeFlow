package prerequisites

import (
	"context"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Prerequisite screening only rejected requirements ForgeFlow is forbidden to
// supply (credentials, signing, live services). It never asked whether ForgeFlow
// can build and verify the repository at all, so discovery offered scaffold
// templates without a test suite and very large multi-module builds whose
// suites outlast every deadline. These checks use evidence that is cheap to
// obtain before any file content is read: the pinned tree and recent GitHub
// Actions timing. They are static evidence, not a promise that tests pass.

var templatePath = regexp.MustCompile(`\{\{|\$\{\{|\{%`)

func treeFinding(kind, source, reason string) domain.PrerequisiteFinding {
	return domain.PrerequisiteFinding{Kind: kind, Source: source, Reason: reason}
}

func testFile(p string) bool {
	l := strings.ToLower(p)
	b := path.Base(l)
	switch {
	case strings.HasSuffix(b, "_test.go"), strings.HasPrefix(b, "test_") && strings.HasSuffix(b, ".py"), strings.HasSuffix(b, "_test.py"),
		strings.HasSuffix(b, "test.java"), strings.HasSuffix(b, "tests.java"), strings.HasSuffix(b, "it.java"), strings.HasSuffix(b, "test.kt"),
		strings.Contains(b, ".test.") || strings.Contains(b, ".spec."), strings.HasSuffix(b, "_spec.rb"), strings.HasSuffix(b, "_test.rs"):
		return true
	}
	return strings.HasPrefix(l, "tests/") || strings.HasPrefix(l, "test/") || strings.Contains(l, "/tests/") || strings.Contains(l, "/src/test/")
}

// ScreenTree applies build-shape checks to a complete pinned tree. It makes no
// request. A truncated tree is left to the existing needs-assessment path.
func ScreenTree(tree github.Tree, profile Profile) []domain.PrerequisiteFinding {
	if tree.Truncated {
		return nil
	}
	findings := []domain.PrerequisiteFinding{}
	files, jvm, goModules, packages, pythonProjects := 0, 0, 0, 0, 0
	tests := false
	rootManifest := ""
	rootFiles := map[string]bool{}
	rootDirs := map[string]bool{}
	heavy := map[string]string{}
	template := ""
	for _, n := range tree.Tree {
		p := n.Path
		l := strings.ToLower(p)
		b := path.Base(l)
		top, _, nested := strings.Cut(l, "/")
		if nested {
			rootDirs[top] = true
		}
		if n.Type != "blob" {
			if strings.HasSuffix(l, ".xcodeproj") {
				heavy["xcode"] = p
			}
			continue
		}
		files++
		if !nested {
			rootFiles[b] = true
		}
		if template == "" && templatePath.MatchString(p) {
			template = p
		}
		if strings.Contains(l, "node_modules/") || strings.Contains(l, "/vendor/") || strings.HasPrefix(l, "vendor/") || strings.Contains(l, "testdata/") {
			continue
		}
		switch b {
		case "pom.xml", "build.gradle", "build.gradle.kts", "build.sbt":
			jvm++
		case "go.mod":
			goModules++
		case "package.json":
			packages++
		case "pyproject.toml", "setup.py":
			pythonProjects++
		case "androidmanifest.xml":
			heavy["android"] = p
		}
		if strings.HasSuffix(b, ".gwt.xml") {
			heavy["gwt"] = p
		}
		if !tests && testFile(p) {
			tests = true
		}
	}
	for _, f := range []string{"workspace", "workspace.bazel", "module.bazel", "build.bazel", ".bazelrc"} {
		if rootFiles[f] {
			heavy["bazel"] = f
		}
	}
	if rootFiles["pants.toml"] {
		heavy["pants"] = "pants.toml"
	}
	if rootFiles[".buckconfig"] {
		heavy["buck"] = ".buckconfig"
	}
	for _, f := range []string{"go.mod", "pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "pom.xml", "build.gradle", "build.gradle.kts", "package.json", "cargo.toml", "gemfile", "composer.json"} {
		if rootFiles[f] {
			rootManifest = f
			break
		}
	}
	for _, f := range []string{"cookiecutter.json", "cookiecutter.yaml", "copier.yml", "copier.yaml"} {
		if rootFiles[f] {
			template = f
		}
	}
	if template == "" && rootDirs["skeleton"] {
		for f := range rootFiles {
			if strings.HasPrefix(f, "template") && (strings.HasSuffix(f, ".yaml") || strings.HasSuffix(f, ".yml")) {
				template = f
			}
		}
	}
	if template != "" {
		findings = append(findings, treeFinding("template_repository", template, "This repository is a project template or scaffold (templated paths or scaffolder files). Its files are rendered elsewhere, so ForgeFlow cannot build and verify a contribution the way the generated project would."))
	}
	names := map[string]string{"gwt": "GWT/Vaadin widgetset compilation", "android": "an Android build", "bazel": "Bazel", "pants": "Pants", "buck": "Buck", "xcode": "Xcode"}
	kinds := make([]string, 0, len(heavy))
	for k := range heavy {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		findings = append(findings, treeFinding("heavy_toolchain", heavy[k], fmt.Sprintf("Verification requires %s, which is too slow or unsupported in ForgeFlow's verification container.", names[k])))
	}
	if jvm > profile.MaxJVMBuildFiles {
		findings = append(findings, treeFinding("build_too_large", "repository tree", fmt.Sprintf("The build has %d JVM build files (multi-module build). ForgeFlow verifies at most %d; larger builds outlast verification deadlines.", jvm, profile.MaxJVMBuildFiles)))
	}
	if goModules > profile.MaxGoModules {
		findings = append(findings, treeFinding("build_too_large", "repository tree", fmt.Sprintf("The repository has %d Go modules. ForgeFlow verifies at most %d.", goModules, profile.MaxGoModules)))
	}
	if packages > 15 || pythonProjects > 10 {
		findings = append(findings, treeFinding("build_too_large", "repository tree", fmt.Sprintf("The repository is a large monorepo (%d package.json, %d Python projects).", packages, pythonProjects)))
	}
	if files > profile.MaxTreeFiles {
		findings = append(findings, treeFinding("repository_too_large", "repository tree", fmt.Sprintf("The repository has %d files; ForgeFlow clones and verifies repositories with at most %d.", files, profile.MaxTreeFiles)))
	}
	if !tests {
		findings = append(findings, treeFinding("no_test_suite", "repository tree", "No test files were found. Without a test suite ForgeFlow cannot prove a change works, so the contribution could not reach review."))
	}
	if rootManifest == "" {
		findings = append(findings, treeFinding("unsupported_build", "repository tree", "No supported root build manifest (go.mod, pyproject.toml, setup.py, requirements.txt, pom.xml, build.gradle, package.json, Cargo.toml, Gemfile or composer.json) was found."))
	}
	return findings
}

type workflowRun struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	HeadBranch   string    `json:"head_branch"`
	Event        string    `json:"event"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	CreatedAt    time.Time `json:"created_at"`
	RunStartedAt time.Time `json:"run_started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Workflows that never run the project's checks: releases, deployment,
// documentation sites, security scanners and repository housekeeping.
// The words match whole name parts: "sync" must not hide "Build Syncthing",
// nor "lock" a "Deadlock tests" workflow.
var nonVerificationWorkflow = regexp.MustCompile(`(?i)(^|[^a-z0-9])(releas\w*|deploy\w*|publish\w*|pages|docs?|document\w*|website|codeql|scorecards?|stale|label\w*|greet\w*|welcome|dependabot|renovate|sync|mirror|nightly|benchmark\w*|docker\w*|images?|lock|triage|cla|backport\w*|cleanup|changelog|semantic|pr-title|size|assign\w*|milestones?|issues?|discussions?|translat\w*|spell\w*)([^a-z0-9]|$)`)

func summarizeRuns(runs []workflowRun, branch string, now time.Time) *domain.CIEvidence {
	type stats struct {
		durations []time.Duration
		recent    []string // newest first, default-branch completed runs
	}
	by := map[string]*stats{}
	order := []string{}
	// Integration, end-to-end and similar workflows test a deployed system. A
	// focused change is verified by the build and unit suites, so those slower
	// workflows size nothing unless they are the only verification there is.
	unitWorkflows := false
	for _, r := range runs {
		if r.Status == "completed" && r.Event != "schedule" && strings.HasPrefix(r.Path, ".github/workflows/") && !nonVerificationWorkflow.MatchString(path.Base(r.Path)+" "+r.Name) && !integrationWorkflow.MatchString(path.Base(r.Path)+" "+r.Name) {
			unitWorkflows = true
			break
		}
	}
	for _, r := range runs {
		// GitHub-managed "dynamic/..." workflows (Copilot review, Dependabot
		// updates) are not the repository's CI.
		if r.Status != "completed" || !strings.HasPrefix(r.Path, ".github/workflows/") || nonVerificationWorkflow.MatchString(path.Base(r.Path)+" "+r.Name) || r.Event == "schedule" {
			continue
		}
		if unitWorkflows && integrationWorkflow.MatchString(path.Base(r.Path)+" "+r.Name) {
			continue
		}
		st := by[r.Path]
		if st == nil {
			st = &stats{}
			by[r.Path] = st
			order = append(order, r.Path)
		}
		start := r.RunStartedAt
		if start.IsZero() {
			start = r.CreatedAt
		}
		// A re-run long after the first attempt reports the whole gap.
		if r.Conclusion == "success" && r.UpdatedAt.After(start) && r.UpdatedAt.Sub(start) <= 6*time.Hour {
			st.durations = append(st.durations, r.UpdatedAt.Sub(start))
		}
		if r.HeadBranch == branch && r.Conclusion != "cancelled" && r.Conclusion != "skipped" && r.Conclusion != "neutral" {
			st.recent = append(st.recent, r.Conclusion)
		}
	}
	ci := &domain.CIEvidence{Workflows: []string{}, ObservedAt: now}
	latest := time.Time{}
	for _, r := range runs {
		if r.Status == "completed" && r.HeadBranch == branch && by[r.Path] != nil && r.Conclusion != "cancelled" && r.Conclusion != "skipped" && r.UpdatedAt.After(latest) {
			latest, ci.LatestConclusion = r.UpdatedAt, r.Conclusion
		}
	}
	for _, p := range order {
		st := by[p]
		ci.Workflows = append(ci.Workflows, p)
		ci.SuccessfulRuns += len(st.durations)
		if len(st.durations) == 0 {
			continue
		}
		sort.Slice(st.durations, func(i, j int) bool { return st.durations[i] < st.durations[j] })
		median := st.durations[len(st.durations)/2]
		if int(median.Seconds()) > ci.MedianSeconds {
			ci.MedianSeconds, ci.SlowestWorkflow = int(median.Seconds()), p
		}
	}
	if len(ci.Workflows) == 0 {
		return nil
	}
	return ci
}

func persistentlyFailing(conclusions []string) bool {
	n := len(conclusions)
	if n > 3 {
		n = 3
	}
	if n == 0 {
		return false
	}
	for _, c := range conclusions[:n] {
		if c == "success" {
			return false
		}
	}
	return true
}

// ScreenCI turns CI evidence into findings. Missing evidence is incomplete,
// not proof of failure, so it withholds ranking without excluding.
func ScreenCI(ci *domain.CIEvidence, runs map[string][]string, profile Profile) []domain.PrerequisiteFinding {
	if ci == nil || ci.SuccessfulRuns == 0 {
		return []domain.PrerequisiteFinding{treeFinding("needs_assessment", "GitHub Actions", "No successful recent GitHub Actions verification run was found, so ForgeFlow cannot confirm the build passes or estimate how long verification takes.")}
	}
	findings := []domain.PrerequisiteFinding{}
	// One red workflow (often a linter) beside green ones leaves a working
	// baseline, and baseline verification records the failure. Only when every
	// verification workflow that ran on the default branch keeps failing is
	// there nothing to verify against.
	failing, ran, first := 0, 0, ""
	for _, p := range ci.Workflows {
		if len(runs[p]) == 0 {
			continue
		}
		ran++
		if persistentlyFailing(runs[p]) {
			failing++
			if first == "" {
				first = p
			}
		}
	}
	if ran > 0 && failing == ran {
		findings = append(findings, treeFinding("ci_failing", first, "The repository's own CI has failed on its default branch in each recent run; a contribution could not be verified against a working baseline."))
	}
	if profile.MaxCIMinutes > 0 && ci.MedianSeconds > profile.MaxCIMinutes*60 {
		findings = append(findings, treeFinding("build_too_slow", ci.SlowestWorkflow, fmt.Sprintf("The repository's build and test CI takes a median of %d minutes. ForgeFlow runs the suite several times in one session (baseline, after the change, after each fix), so it offers repositories whose CI finishes within %d minutes.", (ci.MedianSeconds+59)/60, profile.MaxCIMinutes)))
	}
	return findings
}

func recentConclusions(runs []workflowRun, branch string) map[string][]string {
	out := map[string][]string{}
	for _, r := range runs {
		if r.Status == "completed" && r.HeadBranch == branch && r.Conclusion != "cancelled" && r.Conclusion != "skipped" && r.Conclusion != "neutral" && r.Event != "schedule" {
			out[r.Path] = append(out[r.Path], r.Conclusion)
		}
	}
	return out
}

func blocking(findings []domain.PrerequisiteFinding) bool {
	for _, f := range findings {
		if f.Kind != "needs_assessment" {
			return true
		}
	}
	return false
}

func withFindings(a domain.RepositoryAssessment, findings []domain.PrerequisiteFinding) domain.RepositoryAssessment {
	for _, f := range findings {
		a.Findings = append(a.Findings, f)
		if f.Kind == "needs_assessment" {
			if a.Status == "eligible" {
				a.Status = "needs_assessment"
			}
		} else {
			a.Status = "excluded"
		}
	}
	sort.SliceStable(a.Findings, func(i, j int) bool {
		return a.Findings[i].Kind != "needs_assessment" && a.Findings[j].Kind == "needs_assessment"
	})
	return a
}

// Oversized is the assessment of a repository whose tree GitHub truncates or
// cannot return within the read limit. That is evidence of size, not a failed
// read, and a build of that size does not fit ForgeFlow's verification.
func Oversized(repo, head string, profile Profile) domain.RepositoryAssessment {
	return domain.RepositoryAssessment{Repository: repo, HeadSHA: head, Status: "excluded", Profile: profile.Fingerprint(), CheckedAt: time.Now().UTC(), Files: []string{}, Findings: []domain.PrerequisiteFinding{treeFinding("repository_too_large", "repository tree", fmt.Sprintf("The repository's file tree is too large to list (well over %d files); ForgeFlow does not clone and verify repositories of this size.", profile.MaxTreeFiles))}}
}

// InspectRepository is the complete eligibility decision used by discovery and
// approval: build shape first (no requests), then CI evidence (one request),
// then the bounded instruction and manifest reads. Cheap exclusions stop early.
func InspectRepository(ctx context.Context, session *github.Session, repo github.Repository, head string, tree github.Tree, profile Profile, onError ...func(error)) domain.RepositoryAssessment {
	if tree.Truncated || !shaPattern.MatchString(head) {
		return Inspect(ctx, session, repo.FullName, head, tree, profile, onError...)
	}
	base := domain.RepositoryAssessment{Repository: repo.FullName, HeadSHA: head, Status: "eligible", Profile: profile.Fingerprint(), CheckedAt: time.Now().UTC(), Files: []string{}, Findings: []domain.PrerequisiteFinding{}}
	shape := ScreenTree(tree, profile)
	if blocking(shape) {
		return withFindings(base, shape)
	}
	var page struct {
		Runs []workflowRun `json:"workflow_runs"`
	}
	ciFindings := []domain.PrerequisiteFinding{}
	if err := session.Get(ctx, "/repos/"+repo.FullName+"/actions/runs?exclude_pull_requests=true&per_page=100", &page); err != nil {
		if len(onError) > 0 {
			onError[0](err)
		}
		ciFindings = append(ciFindings, treeFinding("needs_assessment", "GitHub Actions", "GitHub Actions runs could not be read: "+err.Error()))
	} else {
		base.CI = summarizeRuns(page.Runs, repo.DefaultBranch, time.Now().UTC())
		ciFindings = ScreenCI(base.CI, recentConclusions(page.Runs, repo.DefaultBranch), profile)
	}
	if blocking(ciFindings) {
		return withFindings(base, append(shape, ciFindings...))
	}
	a := Inspect(ctx, session, repo.FullName, head, tree, profile, onError...)
	a.CI = base.CI
	return withFindings(a, append(shape, ciFindings...))
}
