package prerequisites

import (
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Some projects accept outside pull requests only for issues they have marked.
// cli/cli states: "We accept external pull requests only for issues labelled
// `help wanted` with explicit Acceptance Criteria", and "Do not proceed on an
// issue labelled `core`". ForgeFlow offered a plain bug there; the run stopped,
// correctly, before coding. A pull request on such an issue is closed however
// good the patch, so the rule is read during screening and applied to issues.

var (
	policyLabel    = regexp.MustCompile("(?i)label(?:l?ed)?\\s+(?:as\\s+)?\\[?[`\"']([^`\"'\\n]{2,40})[`\"']|[`\"']([^`\"'\\n]{2,40})[`\"']\\]?(?:\\[[^\\]]*\\])?\\s+label")
	policyRequire  = regexp.MustCompile(`(?i)\b(only|must|requires?|required|unless|without|not be accepted|won't be accepted|will not be accepted)\b`)
	policyForbid   = regexp.MustCompile(`(?i)\b(do not|don't|never|please avoid)\b[^.\n]*\b(proceed|work|open|submit|contribute|pick)\b`)
	policyPull     = regexp.MustCompile(`(?i)pull requests?|\bPRs?\b|contribution|proceed|implement|work on`)
	policyCriteria = regexp.MustCompile(`(?i)acceptance criteria`)
)

func labelTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// LabelMatches reports whether a repository's label means the wanted label:
// "kind/bug", "type: bug" and "good-first-issue" count; "debug" does not.
func LabelMatches(label, wanted string) bool {
	have, want := labelTokens(label), labelTokens(wanted)
	if len(want) == 0 {
		return false
	}
	for i := 0; i+len(want) <= len(have); i++ {
		ok := true
		for j := range want {
			if have[i+j] != want[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// contributionPolicy reads label rules from contribution instructions only
// (CONTRIBUTING, AGENTS, development documents), never from a README or CI.
func contributionPolicy(texts map[string]string) *domain.ContributionPolicy {
	files := make([]string, 0, len(texts))
	for f := range texts {
		if verificationDocument(f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	p := &domain.ContributionPolicy{}
	add := func(list *[]string, label string) {
		label = strings.TrimSpace(label)
		for _, have := range *list {
			if strings.EqualFold(have, label) {
				return
			}
		}
		*list = append(*list, label)
	}
	for _, file := range files {
		for i, line := range strings.Split(texts[file], "\n") {
			matches := policyLabel.FindAllStringSubmatch(line, -1)
			if len(matches) == 0 || !policyPull.MatchString(line) {
				continue
			}
			forbid := policyForbid.MatchString(line) && !strings.Contains(strings.ToLower(line), "without")
			if !forbid && !policyRequire.MatchString(line) {
				continue
			}
			for _, m := range matches {
				label := m[1]
				if label == "" {
					label = m[2]
				}
				if forbid {
					add(&p.ForbiddenLabels, label)
				} else {
					add(&p.RequiredLabels, label)
				}
			}
			if !forbid && policyCriteria.MatchString(line) {
				add(&p.RequiredText, "acceptance criteria")
			}
			if p.Source == "" {
				p.Source, p.Line = file, i+1
			}
		}
	}
	// A label cannot be both; a stated prohibition is the more specific rule.
	kept := p.RequiredLabels[:0]
	for _, label := range p.RequiredLabels {
		forbidden := false
		for _, f := range p.ForbiddenLabels {
			forbidden = forbidden || strings.EqualFold(f, label)
		}
		if !forbidden {
			kept = append(kept, label)
		}
	}
	p.RequiredLabels = kept
	if len(p.RequiredLabels) == 0 && len(p.ForbiddenLabels) == 0 {
		return nil
	}
	return p
}

// PolicyAllows applies a project's stated label rules to one issue. The reason
// is empty when the issue may be worked on.
func PolicyAllows(p *domain.ContributionPolicy, issue github.Issue) string {
	if p == nil {
		return ""
	}
	for _, l := range issue.Labels {
		for _, f := range p.ForbiddenLabels {
			if LabelMatches(l.Name, f) {
				return "the project's contribution rules exclude issues labelled " + f
			}
		}
	}
	if len(p.RequiredLabels) > 0 {
		found := false
		for _, l := range issue.Labels {
			for _, r := range p.RequiredLabels {
				found = found || LabelMatches(l.Name, r)
			}
		}
		if !found {
			return "the project accepts outside pull requests only on issues labelled " + strings.Join(p.RequiredLabels, " or ")
		}
	}
	body := strings.ToLower(issue.Body)
	for _, text := range p.RequiredText {
		if !strings.Contains(body, text) {
			return "the project requires the issue to state " + text
		}
	}
	return ""
}
