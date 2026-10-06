package analyzer

import (
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/estimator"
	"forgeflow/internal/github"
	"forgeflow/internal/ranking"
	"math"
	"path"
	"strings"
	"time"
)

func HasLabel(i github.Issue, name string) bool {
	for _, l := range i.Labels {
		if strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}
func Difficulty(i github.Issue) string {
	if HasLabel(i, "good first issue") || HasLabel(i, "easy") || HasLabel(i, "beginner") {
		return "easy"
	}
	if HasLabel(i, "hard") || HasLabel(i, "complex") {
		return "hard"
	}
	return "medium"
}
func Excluded(i github.Issue, c domain.Config) bool {
	// Conservative label-based exclusion: a backend issue mentioning documentation is not automatically documentation-only.
	for _, x := range c.Profile.Exclude {
		x = strings.ToLower(x)
		if HasLabel(i, x) {
			return true
		}
		if x == "documentation-only" && (HasLabel(i, "documentation") || HasLabel(i, "docs")) && !HasLabel(i, "bug") && !HasLabel(i, "enhancement") {
			return true
		}
		if x == "frontend-only" && (HasLabel(i, "frontend") || HasLabel(i, "ui")) {
			return true
		}
	}
	return false
}

// InspectTree records presence signals only. A truncated tree cannot establish absence.
func InspectTree(t github.Tree, e *domain.Evidence) {
	e.TreeComplete = !t.Truncated
	e.BuildFiles = []string{}
	found := map[string]bool{}
	for _, node := range t.Tree {
		p := strings.ToLower(node.Path)
		base := path.Base(p)
		if node.Type == "blob" {
			if base == "contributing.md" || base == "contributing" {
				found["guidelines"] = true
			}
			if base == "agents.md" {
				found["agents"] = true
			}
			if strings.HasPrefix(base, "code_of_conduct") {
				found["conduct"] = true
			}
			if strings.HasPrefix(p, ".github/workflows/") || base == ".gitlab-ci.yml" || base == "jenkinsfile" || strings.HasPrefix(p, ".circleci/") {
				found["ci"] = true
			}
			if strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "test.java") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.Contains(p, "/tests/") {
				found["tests"] = true
			}
			if !strings.Contains(p, "/") {
				switch base {
				case "go.mod", "pom.xml", "build.gradle", "build.gradle.kts", "pyproject.toml", "setup.py", "package.json", "cargo.toml", "makefile", "cmakelists.txt":
					e.BuildFiles = append(e.BuildFiles, node.Path)
				}
			}
		}
	}
	set := func(key string) *bool {
		if found[key] {
			v := true
			return &v
		}
		if t.Truncated {
			return nil
		}
		v := false
		return &v
	}
	e.Guidelines = set("guidelines")
	e.AgentRules = set("agents")
	e.CodeOfConduct = set("conduct")
	e.CI = set("ci")
	e.Tests = set("tests")
	if t.Truncated {
		e.Warnings = append(e.Warnings, "Repository tree truncated; missing files cannot establish absence.")
	}
}
func ProfileWeight(values map[string]float64, name string) float64 {
	for k, v := range values {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return 0
}
func Domain(r github.Repository, c domain.Config) (string, float64) {
	text := strings.ToLower(r.Description + " " + strings.Join(r.Topics, " "))
	best := "unclassified"
	weight := 0.0
	for name, w := range c.Profile.Domains {
		terms := []string{strings.ToLower(name), strings.ReplaceAll(strings.ToLower(name), "-", " ")}
		switch strings.ToLower(name) {
		case "backend":
			terms = append(terms, "server", "api", "backend")
		case "distributed-systems":
			terms = append(terms, "distributed", "consensus", "replication")
		case "databases":
			terms = append(terms, "database", "storage", "sql")
		case "ai-infrastructure":
			terms = append(terms, "machine learning", "llm", "mlops", "ai tooling")
		}
		for _, term := range terms {
			if strings.Contains(text, term) && (w > weight || (w == weight && name < best)) {
				best = name
				weight = w
			}
		}
	}
	return best, weight
}
func Analyze(r github.Repository, i github.Issue, e domain.Evidence, c domain.Config, now time.Time) (domain.Opportunity, error) {
	d, domainWeight := Domain(r, c)
	difficulty := Difficulty(i)
	factors := []domain.Factor{}
	add := func(key, label string, score, weight float64, reason string) {
		factors = append(factors, domain.Factor{Key: key, Label: label, Score: math.Max(0, math.Min(100, score)), Weight: weight, Reason: reason})
	}
	add("skill", "Language match", ProfileWeight(c.Profile.Languages, r.Language)*100, .15, fmt.Sprintf("Repository primary language is %s; configured profile weight %.2f. Secondary languages are not inspected.", r.Language, ProfileWeight(c.Profile.Languages, r.Language)))
	ds := domainWeight * 100
	dr := "No configured domain matched repository description/topics; neutral 50, domain remains unclassified."
	if ds == 0 {
		ds = 50
	} else {
		dr = fmt.Sprintf("Description/topics match %s; configured interest weight %.2f. This is a keyword heuristic.", d, domainWeight)
	}
	add("domain", "Domain fit", ds, .10, dr)
	health := 50.0
	hr := "Latest commit unavailable; neutral baseline."
	if e.LastCommit != nil {
		days := math.Max(0, now.Sub(*e.LastCommit).Hours()/24)
		health = math.Max(20, 100-days/float64(c.Repositories.RecentActivityDays)*40)
		hr = fmt.Sprintf("Latest default-branch commit observed %d days ago.", int(days))
	}
	for _, v := range []*bool{e.CI, e.Tests, e.Guidelines} {
		if v != nil && !*v {
			health -= 10
		}
	}
	add("health", "Repository health", health, .12, hr+" Missing observed CI, tests or guidelines each subtract 10; presence does not prove they pass.")
	maint := 50.0
	mr := "Maintainer participation unavailable; neutral 50."
	if e.MaintainerComments != nil {
		maint = 40
		if *e.MaintainerComments > 0 {
			maint = 80
		}
		mr = fmt.Sprintf("%d member/owner/collaborator comments in a bounded sample of %d issue comments; not a response-time guarantee.", *e.MaintainerComments, e.CommentsSampled)
	}
	add("maintainer", "Maintainer participation", maint, .09, mr)
	clarity := 35.0
	length := len(strings.TrimSpace(i.Body))
	if length >= 150 {
		clarity = 65
	}
	if length >= 500 {
		clarity = 80
	}
	if strings.Contains(strings.ToLower(i.Body), "reproduc") || strings.Contains(i.Body, "```") {
		clarity += 10
	}
	add("clarity", "Issue clarity", clarity, .12, fmt.Sprintf("Issue description has %d bytes; reproduction hints/code blocks increase this heuristic. Requirements have not been validated.", length))
	merge := 50.0
	mergeReason := "External PR acceptance unavailable; neutral 50."
	if e.ExternalPRsMerged != nil && e.MergedPRsSampled > 0 {
		merge = 40
		if *e.ExternalPRsMerged > 0 {
			merge = 75
		}
		mergeReason = fmt.Sprintf("%d externally authored merges observed among %d merged PRs in the latest 30 closed PRs; biased sample, not a merge probability.", *e.ExternalPRsMerged, e.MergedPRsSampled)
	}
	add("merge", "Acceptance signals", merge, .08, mergeReason)
	add("learning", "Learning fit", (ProfileWeight(c.Profile.Languages, r.Language)*100+ds)/2, .07, "Average language/domain fit is a learning heuristic; no personalized curriculum is inferred.")
	portfolio := math.Min(90, 40+math.Log10(float64(r.Stars)+1)*12)
	add("career", "Portfolio visibility", portfolio, .06, fmt.Sprintf("%d repository stars provide a logarithmic visibility proxy; no career outcome is promised.", r.Stars))
	usefulness := 55.0
	ur := "No bug/enhancement label; usefulness unknown, baseline 55."
	if HasLabel(i, "bug") || HasLabel(i, "enhancement") {
		usefulness = 80
		ur = "Bug/enhancement label signals requested work; user impact has not been measured."
	}
	add("usefulness", "Requested usefulness", usefulness, .08, ur)
	fit := 30.0
	for _, v := range c.Issues.Difficulty {
		if v == difficulty {
			fit = 90
		}
	}
	add("difficulty", "Difficulty suitability", fit, .06, "Label-derived difficulty is "+difficulty+"; checked against configured difficulty preferences. Code scope is still unknown.")
	competition := 50.0
	cr := "Competing PR search unavailable; neutral 50."
	if e.CompetitionChecked {
		competition = 90
		cr = "No open PR referencing this issue found in the bounded search; absence is not guaranteed."
		if len(e.CompetingPRs) > 0 {
			competition = 20
			cr = fmt.Sprintf("%d open PRs reference this issue in the search sample.", len(e.CompetingPRs))
		}
	}
	if len(e.CompetingPRs) > 0 {
		competition = 20
		cr = fmt.Sprintf("%d open PRs reference this issue; positive competition evidence applies even when the wider search is incomplete.", len(e.CompetingPRs))
	}
	if len(i.Assignees) > 0 {
		competition = math.Min(competition, 35)
		cr += fmt.Sprintf(" Issue has %d assignees.", len(i.Assignees))
	}
	add("competition", "Competition risk", competition, .07, cr)
	rank, err := ranking.Score(domain.Quality{Factors: factors})
	if err != nil {
		return domain.Opportunity{}, err
	}
	rank.Version = "github-evidence-v1"
	labels := []string{}
	for _, l := range i.Labels {
		labels = append(labels, l.Name)
	}
	risks := append([]string{}, e.Warnings...)
	risks = append(risks, "Scores are evidence-based heuristics; merge probability and exact code scope are unknown.")
	if len(i.Assignees) > 0 {
		risks = append(risks, "Issue is assigned; coordinate with maintainers before working.")
	}
	if len(e.CompetingPRs) > 0 {
		risks = append(risks, "An open PR references this issue; inspect competing work before proceeding.")
	}
	if now.Sub(i.UpdatedAt) > 180*24*time.Hour {
		risks = append(risks, "Issue has not been updated for over 180 days; it may be obsolete.")
	}
	est := estimator.Estimate(estimator.Signals{ScopeKnown: false})
	est.Explanation = append(est.Explanation, "File and iteration ranges are generic placeholders until workspace code analysis; issue text alone does not establish scope.")
	summary := strings.TrimSpace(i.Body)
	if len([]rune(summary)) > 600 {
		summary = string([]rune(summary)[:600]) + "…"
	}
	return domain.Opportunity{ID: fmt.Sprintf("github:%s:%d", r.FullName, i.Number), Repository: r.FullName, Number: i.Number, Title: i.Title, Summary: summary, Language: r.Language, Domain: d, Labels: labels, Difficulty: difficulty, MergeLikelihood: "unknown", Risks: risks, Ranking: rank, Estimate: est, Evidence: &e}, nil
}
