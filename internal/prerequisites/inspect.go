package prerequisites

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxFiles = 24
const MaxFileBytes = 128 << 10
const MaxTotalBytes = 768 << 10

var shaPattern = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)`)
var scriptReference = regexp.MustCompile(`(?:\./)?(?:scripts|tools|hack|tests|ci|\.github/scripts)/[A-Za-z0-9_./-]+\.(?:sh|bash|py|js|mjs|ps1)`)

type node struct {
	sha, mode string
	size      int
}

// Inspect pins every content read to the tree of the observed default-branch
// commit. Evidence that could not be read or verified holds the repository out
// of ranking. Evidence that was merely not reached (the read budget ran out on
// secondary files, a large README, a broken link) is recorded as a caution: it
// does not make an assessment incomplete, and treating it so had excluded most
// ordinary repositories.
func Inspect(ctx context.Context, session *github.Session, repo, head string, tree github.Tree, profile Profile, onError ...func(error)) domain.RepositoryAssessment {
	a := domain.RepositoryAssessment{Repository: repo, HeadSHA: head, Status: "eligible", Profile: profile.Fingerprint(), CheckedAt: time.Now().UTC(), Files: []string{}, Findings: []domain.PrerequisiteFinding{}}
	addUnknown := func(source, reason string) {
		a.Findings = append(a.Findings, domain.PrerequisiteFinding{Kind: "needs_assessment", Source: source, Reason: reason})
		if a.Status != "excluded" {
			a.Status = "needs_assessment"
		}
	}
	cautions := []domain.PrerequisiteFinding{}
	caution := func(source, reason string) {
		cautions = append(cautions, domain.PrerequisiteFinding{Kind: "caution", Source: source, Reason: reason})
	}
	// Scripts reached only through a mention in prose; see screen.
	weak := map[string]bool{}
	strong := map[string]bool{}
	if !profile.LinuxProvisioning {
		addUnknown("runner", "Repository-driven isolated Linux provisioning is not configured; configure a supported verification runner before ranking new repositories.")
	}
	if !shaPattern.MatchString(head) || tree.Truncated {
		addUnknown("repository tree", "A complete tree at an immutable commit is required for prerequisite screening.")
		return a
	}
	nodes := map[string]node{}
	queue := []string{}
	for _, n := range tree.Tree {
		if n.Type != "blob" || !safePath(n.Path) {
			continue
		}
		nodes[n.Path] = node{sha: n.SHA, mode: n.Mode, size: n.Size}
		if selectedFile(n.Path) {
			queue = append(queue, n.Path)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		pi, pj := priority(queue[i]), priority(queue[j])
		if pi != pj {
			return pi < pj
		}
		return queue[i] < queue[j]
	})
	if len(queue) == 0 {
		addUnknown("repository tree", "No contribution instructions or build/verification manifest was found.")
		return a
	}
	texts := map[string]string{}
	seen := map[string]bool{}
	total := 0
	for len(queue) > 0 {
		if ctx.Err() != nil {
			addUnknown("screening", "Prerequisite screening was interrupted before completion.")
			break
		}
		file := queue[0]
		queue = queue[1:]
		if seen[file] {
			continue
		}
		seen[file] = true
		n, exists := nodes[file]
		if !exists {
			// References are found by pattern; a broken link or a path built at
			// run time is not a requirement.
			continue
		}
		if len(a.Files) >= MaxFiles {
			caution(file, fmt.Sprintf("Screening read the %d highest-priority files (contribution instructions, build recipes, CI, root manifests); this file and any after it were not read.", MaxFiles))
			break
		}
		if n.mode == "120000" || !shaPattern.MatchString(n.sha) || n.size > MaxFileBytes || n.size < 0 {
			if priority(file) == 0 {
				addUnknown(file, "Contribution instructions are linked, oversized or lack a valid immutable blob identity.")
			} else {
				caution(file, "This file is a link or larger than the per-file screening limit and was not read.")
			}
			continue
		}
		var blob github.Blob
		if err := session.Get(ctx, "/repos/"+repo+"/git/blobs/"+url.PathEscape(n.sha), &blob); err != nil {
			if len(onError) > 0 {
				onError[0](err)
			}
			addUnknown(file, "Prerequisite content could not be read: "+err.Error())
			continue
		}
		body, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil || blob.Encoding != "base64" || blob.SHA != n.sha || blob.Size != len(body) || len(body) > MaxFileBytes || (n.size != 0 && n.size != len(body)) || !utf8.Valid(body) || strings.ContainsRune(string(body), 0) {
			addUnknown(file, "Prerequisite content failed identity, text or size validation.")
			continue
		}
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", len(body))
		h.Write(body)
		if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(n.sha) {
			addUnknown(file, "Prerequisite bytes do not match the pinned Git blob.")
			continue
		}
		total += len(body)
		if total > MaxTotalBytes {
			caution(file, "Screening reached its total size limit; this file and any after it were not read.")
			break
		}
		a.Files = append(a.Files, file)
		texts[file] = string(body)
		if isDocument(file) {
			for _, match := range markdownLink.FindAllStringSubmatch(string(body), -1) {
				u, e := url.Parse(match[1])
				if e != nil || u.Scheme != "" || u.Host != "" || u.Path == "" {
					continue
				}
				ref := path.Clean(path.Join(path.Dir(file), u.Path))
				if safePath(ref) && isDocument(ref) {
					queue = append(queue, ref)
				}
			}
		}
		if isDocument(file) || strings.EqualFold(path.Base(file), "makefile") || strings.HasSuffix(file, ".sh") || strings.HasSuffix(file, ".py") || strings.HasSuffix(file, ".js") || strings.HasSuffix(file, ".mjs") {
			refs := string(body)
			if strings.EqualFold(path.Base(file), "makefile") {
				refs = verificationRecipes(refs, texts)
			}
			// A script run by a build recipe or by another verification script
			// is part of verification. One merely named in prose may be anything.
			fromRecipe := !isDocument(file) && !weak[file]
			for _, ref := range scriptReference.FindAllString(refs, -1) {
				ref = path.Clean(ref)
				if _, ok := nodes[ref]; !ok {
					ref = path.Clean(path.Join(path.Dir(file), ref))
				}
				if fromRecipe {
					strong[ref] = true
					delete(weak, ref)
				} else if !strong[ref] && !seen[ref] {
					weak[ref] = true
				}
				queue = append(queue, ref)
			}
		}
		if path.Base(file) == "package.json" {
			for _, line := range strings.Split(string(body), "\n") {
				l := strings.ToLower(line)
				if strings.Contains(l, `"test`) || strings.Contains(l, `"verify`) || strings.Contains(l, `"check`) || strings.Contains(l, `"integration`) {
					for _, ref := range scriptReference.FindAllString(line, -1) {
						strong[path.Clean(ref)] = true
						delete(weak, path.Clean(ref))
						queue = append(queue, path.Clean(ref))
					}
				}
			}
		}
	}
	findings, noted := screen(texts, weak, profile)
	a.Requirements = append(dependencyRequirements(texts), append(noted, cautions...)...)
	for _, f := range findings {
		a.Findings = append(a.Findings, f)
		if f.Kind == "needs_assessment" {
			if a.Status != "excluded" {
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

func safePath(p string) bool {
	return p != "" && len(p) <= 1024 && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func isDocument(p string) bool {
	b := strings.ToLower(path.Base(p))
	return strings.HasSuffix(b, ".md") || b == "contributing" || b == "readme"
}
func priority(p string) int {
	b := strings.ToLower(path.Base(p))
	if b == "agents.md" || strings.HasPrefix(b, "contributing") {
		return 0
	}
	if b == "makefile" || b == ".standards.yaml" {
		return 1
	}
	// Root CI and manifests state how the project is verified. A README is
	// general, and nested manifests matter only to the packages they build.
	if strings.HasPrefix(p, ".github/workflows/") {
		return 2
	}
	if !strings.Contains(p, "/") && !isDocument(p) {
		return 3
	}
	if isDocument(p) {
		return 4
	}
	return 5
}
func selectedFile(p string) bool {
	l := strings.ToLower(p)
	b := path.Base(l)
	if strings.HasPrefix(l, ".github/workflows/") && (strings.HasSuffix(l, ".yml") || strings.HasSuffix(l, ".yaml")) {
		// Release, deployment, docs-site and housekeeping workflows are not
		// verification: their deploy secrets are not contribution requirements,
		// and reading them exhausted the bounded file budget (Skript's six docs
		// workflows held a passing repository out of ranking).
		return !nonVerificationWorkflow.MatchString(strings.TrimPrefix(l, ".github/workflows/"))
	}
	if strings.Contains(l, "/") {
		if b == "agents.md" {
			return true
		}
		if l == ".github/contributing.md" || l == "docs/contributing.md" || l == "docs/development.md" || l == "docs/testing.md" {
			return true
		}
		// Nested packages and documentation builds matter in monorepos and
		// scaffolder templates. Generated deployment workflows are not root CI.
		return dependencyManifest(b) || b == "makefile" || b == "mkdocs.yml" || b == "mkdocs.yaml"
	}
	switch b {
	case "agents.md", "contributing.md", "contributing", "readme.md", "readme", "makefile", ".standards.yaml", ".standards.yml", "go.mod", "package.json", "pyproject.toml", "pom.xml", "build.gradle", "build.gradle.kts", "cargo.toml", "gemfile", "composer.json", "cmakelists.txt", "dockerfile", "compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml", "requirements.txt", "global.json":
		return true
	}
	return dependencyManifest(b) || b == "mkdocs.yml" || b == "mkdocs.yaml"
}

func dependencyManifest(b string) bool {
	switch b {
	case "go.mod", "package.json", "pyproject.toml", "pipfile", "setup.py", "setup.cfg", "requirements.txt", "pom.xml", "build.gradle", "build.gradle.kts", "cargo.toml", "gemfile", "composer.json", "cmakelists.txt", "global.json":
		return true
	}
	return strings.HasSuffix(b, ".csproj") || strings.HasSuffix(b, ".sln")
}
