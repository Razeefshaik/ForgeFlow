package prerequisites

import (
	"forgeflow/internal/domain"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var mandatory = regexp.MustCompile(`(?i)\b(must|required?|requires|mandatory|need(?:s)?|minimum|at least|prerequisite)\b`)
var optional = regexp.MustCompile(`(?i)\b(optional|not required|no credentials|do not need|does not require|without credentials)\b|\bno\b[^.\n]{0,60}\bis required\b`)
var dockerCommand = regexp.MustCompile(`(?i)\bdocker(?:-compose\b|\s+(?:compose|build|run|exec)\b)|/var/run/docker\.sock|--privileged\b`)
var signing = regexp.MustCompile(`(?i)\b(ed25519|private key|trusted signer|signed receipt|signing key|signing receipt)\b`)
var credentials = regexp.MustCompile(`(?i)\b(credentials|api[_ -]?key|access[_ -]?token|password|[A-Z][A-Z0-9_]*_(?:TOKEN|SECRET|PASSWORD|API_KEY))\b`)
var liveService = regexp.MustCompile(`(?i)\b(live|running|hosted)\b.*\b(server|service|instance|database|cluster)\b|\b(server|service|instance|database|cluster)\b.*\b(live|running)\b`)
var platform = regexp.MustCompile(`(?i)\b(xcode|macos[- ]only|windows[- ]only|ios simulator|android emulator|nvidia|cuda|gpu|kvm)\b`)
var ram = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(?:gb|gib)\s*(?:of\s+)?(?:ram|memory)|(?:ram|memory)\s*(?:of|:|>=|at least)?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:gb|gib)`)
var cores = regexp.MustCompile(`(?i)([0-9]+)\s*(?:cpu cores|cpus|cores)`)
var makeTarget = regexp.MustCompile(`\bmake\s+(?:-[a-zA-Z]+\s+)*([A-Za-z0-9_.-]+)`)
var makeHeader = regexp.MustCompile(`^([A-Za-z0-9_. -]+):([^=]*)$`)
var workflowJob = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)

func analyze(texts map[string]string, profile Profile) []domain.PrerequisiteFinding {
	files := make([]string, 0, len(texts))
	for f := range texts {
		files = append(files, f)
	}
	sort.Strings(files)
	result := []domain.PrerequisiteFinding{}
	seen := map[string]bool{}
	add := func(kind, file string, line int, reason string) {
		key := kind + ":" + file
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, domain.PrerequisiteFinding{Kind: kind, Source: file, Line: line, Reason: reason})
	}
	for _, file := range files {
		text := texts[file]
		if strings.EqualFold(path.Base(file), "makefile") {
			text = verificationRecipes(text, texts)
		}
		script := strings.HasSuffix(file, ".sh") || strings.HasSuffix(file, ".bash") || strings.HasSuffix(file, ".py") || strings.HasSuffix(file, ".js") || strings.HasSuffix(file, ".mjs") || strings.HasSuffix(file, ".ps1")
		build := strings.EqualFold(path.Base(file), "makefile") || script
		workflow := strings.HasPrefix(file, ".github/workflows/")
		verificationDoc := strings.Contains(strings.ToLower(file), "contribut") || strings.Contains(strings.ToLower(file), "develop") || strings.Contains(strings.ToLower(file), "test") || strings.EqualFold(path.Base(file), "AGENTS.md")
		sectionVerification := verificationDoc
		releaseJob := false
		for i, line := range strings.Split(text, "\n") {
			l := strings.ToLower(line)
			stripped := strings.TrimSpace(l)
			if stripped == "" || strings.HasPrefix(stripped, "#") && build || optional.MatchString(line) {
				continue
			}
			required := mandatory.MatchString(line)
			if workflow {
				if m := workflowJob.FindStringSubmatch(line); m != nil {
					job := strings.ToLower(m[1])
					releaseJob = strings.Contains(job, "release") || strings.Contains(job, "deploy") || strings.Contains(job, "publish")
				}
			}
			if strings.HasPrefix(stripped, "#") {
				sectionVerification = verificationDoc || strings.Contains(l, "test") || strings.Contains(l, "develop") || strings.Contains(l, "contribut") || strings.Contains(l, "verification") || strings.Contains(l, "local setup")
			}
			localRequirement := sectionVerification || strings.Contains(l, "test") || strings.Contains(l, "verif") || strings.Contains(l, "contribut")
			manifestScript := path.Base(file) == "package.json" && (strings.Contains(l, `"test`) || strings.Contains(l, `"verify`) || strings.Contains(l, `"check`) || strings.Contains(l, `"lint`) || strings.Contains(l, `"integration`))
			if signing.MatchString(line) && (required || strings.Contains(l, "exit-0") || strings.Contains(l, "gate run")) {
				add("trusted_signing", file, i+1, "Required trusted signing or receipt cannot be supplied by ForgeFlow's contribution runner; the existing trust anchor must not be replaced.")
			}
			if credentials.MatchString(line) && (required && localRequirement || build || manifestScript) {
				add("private_credentials", file, i+1, "Required service credentials are not available to contribution processes; repository-driven images cannot receive private credentials.")
			}
			if liveService.MatchString(line) && required && localRequirement {
				add("external_service", file, i+1, "Required live service provisioning is not supported by the current isolated verification runner.")
			}
			if dockerCommand.MatchString(line) && (build || manifestScript || required && localRequirement) {
				add("nested_docker", file, i+1, "Verification requires Docker/Compose execution or privileged containers; the runner exposes neither a Docker socket nor privileged service provisioning.")
			}
			if workflow && !releaseJob && (stripped == "services:" || strings.Contains(l, "secrets.") && !strings.Contains(l, "secrets.github_token")) && !strings.Contains(strings.ToLower(path.Base(file)), "release") && !strings.Contains(strings.ToLower(path.Base(file)), "deploy") && !strings.Contains(strings.ToLower(path.Base(file)), "publish") {
				add("needs_assessment", file, i+1, "Verification CI uses services or secrets whose availability and local requirement must be established before ranking.")
			}
			if platform.MatchString(line) && (required && localRequirement || build) {
				add("unsupported_platform", file, i+1, "Required platform, emulator or GPU capability is outside the configured Linux verification boundary.")
			}
			if (strings.Contains(l, "sudo ") || strings.Contains(l, "/etc/") || strings.Contains(l, "/usr/local/")) && (required && localRequirement || build) && (strings.Contains(l, "sudo ") || strings.Contains(l, "mkdir ") || strings.Contains(l, "tee ") || strings.Contains(l, " >") || strings.Contains(l, "write")) {
				add("runtime_privilege", file, i+1, "Verification requires elevated execution or system-directory writes; runtime containers have an unprivileged user and a read-only root.")
			}
			if required && localRequirement {
				if m := ram.FindStringSubmatch(line); m != nil {
					n := m[1]
					if n == "" {
						n = m[2]
					}
					gb, _ := strconv.ParseFloat(n, 64)
					if gb*1024 > float64(profile.MemoryMiB) {
						add("resource_limit", file, i+1, "Documented minimum verification memory exceeds the configured runner memory limit.")
					}
				}
				if m := cores.FindStringSubmatch(line); m != nil {
					n, _ := strconv.Atoi(m[1])
					if n > profile.CPUs {
						add("resource_limit", file, i+1, "Documented minimum CPU count exceeds the configured runner CPU limit.")
					}
				}
			}
		}
		// A mandatory standards gate together with a pinned signing key is a
		// requirement even when its instruction and trust policy occupy different lines.
		if isDocument(file) && strings.Contains(strings.ToLower(text), "standardsctl gate run") {
			for _, policy := range []string{".standards.yaml", ".standards.yml"} {
				p := strings.ToLower(texts[policy])
				if strings.Contains(p, "public_key") || strings.Contains(p, "public-key") || strings.Contains(p, "ed25519") {
					add("trusted_signing", file, 1, "Contribution instructions require a standards gate bound to a trusted signing key; ForgeFlow cannot generate the trusted receipt.")
				}
			}
		}
	}
	return result
}

// Preserve line numbers while following only verification targets and their
// dependency targets, not unrelated release/deployment Docker recipes.
func verificationRecipes(text string, documents map[string]string) string {
	lines := strings.Split(text, "\n")
	targets := map[string][]int{}
	deps := map[string][]string{}
	current := []string{}
	for i, line := range lines {
		if m := makeHeader.FindStringSubmatch(line); m != nil {
			current = strings.Fields(m[1])
			for _, t := range current {
				deps[t] = strings.Fields(m[2])
				targets[t] = append(targets[t], i)
			}
		} else if strings.HasPrefix(line, "\t") {
			for _, t := range current {
				targets[t] = append(targets[t], i)
			}
		} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			current = nil
		}
	}
	selected := map[string]bool{}
	var visit func(string)
	visit = func(t string) {
		if selected[t] {
			return
		}
		selected[t] = true
		for _, d := range deps[t] {
			visit(d)
		}
	}
	for t := range targets {
		l := strings.ToLower(t)
		if l == "test" || l == "check" || l == "lint" || strings.HasPrefix(l, "test-") || strings.HasPrefix(l, "verify") || strings.HasPrefix(l, "integration") || strings.HasSuffix(l, "-test") || strings.HasSuffix(l, "-check") {
			visit(t)
		}
	}
	for f, doc := range documents {
		if isDocument(f) {
			for _, m := range makeTarget.FindAllStringSubmatch(doc, -1) {
				l := strings.ToLower(m[1])
				if !strings.Contains(l, "release") && !strings.Contains(l, "deploy") {
					visit(m[1])
				}
			}
		}
	}
	keep := map[int]bool{}
	for t := range selected {
		for _, i := range targets[t] {
			keep[i] = true
		}
	}
	for i := range lines {
		if !keep[i] {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
