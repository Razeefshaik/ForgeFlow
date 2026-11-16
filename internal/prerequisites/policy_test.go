package prerequisites

import (
	"context"
	"forgeflow/internal/github"
	"strings"
	"testing"
)

// The wording of cli/cli on 2026-11-16, where ForgeFlow offered a plain bug
// (#14374) that the project does not accept outside pull requests for.
const cliContributing = "## Contributing\n\nWe accept external pull requests only for issues labelled `help wanted` with explicit Acceptance Criteria. We encourage issues and discussion posts for all other contributions.\n\nPlease do:\n* Open a pull request for an issue labelled [`help wanted`][hw] with explicit Acceptance Criteria. Issues labelled [`good first issue`][gfi] must also meet these requirements\n\nPlease avoid:\n* Open a pull request for issues without the `help wanted` label or explicit Acceptance Criteria\n"
const cliAgents = "1. Work only on issues that qualify.\n3. Do not proceed on an issue labelled `core`. Keep the change within the\n   eligible issue's acceptance criteria.\n"

func issue(body string, labels ...string) github.Issue {
	i := github.Issue{Number: 1, Body: body}
	for _, l := range labels {
		i.Labels = append(i.Labels, github.Label{Name: l})
	}
	return i
}

func TestProjectLabelPolicyIsReadFromContributionInstructions(t *testing.T) {
	p := contributionPolicy(map[string]string{".github/CONTRIBUTING.md": cliContributing, "AGENTS.md": cliAgents})
	if p == nil {
		t.Fatal("no policy read")
	}
	required := strings.Join(p.RequiredLabels, ",")
	if !strings.Contains(required, "help wanted") || strings.Join(p.ForbiddenLabels, ",") != "core" || strings.Join(p.RequiredText, ",") != "acceptance criteria" || p.Source == "" || p.Line == 0 {
		t.Fatalf("unexpected policy: %+v", p)
	}
	criteria := "## Acceptance Criteria\nGiven a closed PR, the comment is posted."
	for name, tc := range map[string]struct {
		issue   github.Issue
		allowed bool
	}{
		"plain bug (#14374)":             {issue("It discards the comment.", "bug", "priority-2", "gh-pr"), false},
		"help wanted without criteria":   {issue("It discards the comment.", "bug", "help wanted"), false},
		"help wanted with criteria":      {issue(criteria, "bug", "help wanted"), true},
		"hyphenated label":               {issue(criteria, "help-wanted"), true},
		"core is excluded":               {issue(criteria, "help wanted", "core"), false},
		"good first issue with criteria": {issue(criteria, "good first issue"), true},
	} {
		if reason := PolicyAllows(p, tc.issue); (reason == "") != tc.allowed {
			t.Errorf("%s: allowed=%v, reason %q", name, reason == "", reason)
		}
	}
	if PolicyAllows(nil, issue("x", "bug")) != "" {
		t.Fatal("a project without a stated policy restricted an issue")
	}
}

func TestOrdinaryInstructionsStateNoLabelPolicy(t *testing.T) {
	for name, texts := range map[string]map[string]string{
		"plain":             {"CONTRIBUTING.md": "Run go test ./... before opening a pull request.\nLook for issues labelled `good first issue` if you are new.\n"},
		"readme is ignored": {"README.md": "We accept pull requests only for issues labelled `approved`.\n"},
		"triage notes":      {"CONTRIBUTING.md": "Maintainers add the `bug` label after triage.\n"},
	} {
		if p := contributionPolicy(texts); p != nil {
			t.Errorf("%s: invented a policy: %+v", name, p)
		}
	}
}

func TestPolicyIsRecordedWithoutExcludingTheRepository(t *testing.T) {
	tree, session := inspectFixture(t, map[string]string{"CONTRIBUTING.md": cliContributing + "\nRun go test ./...\n", "go.mod": "module new/project"}, nil)
	a := Inspect(context.Background(), session, "new/project", strings.Repeat("a", 40), tree, LinuxProfile())
	if a.Status != "eligible" || a.Policy == nil || len(a.Policy.RequiredLabels) == 0 {
		t.Fatalf("policy must restrict issues, not the repository: %s %+v %+v", a.Status, a.Findings, a.Policy)
	}
}
