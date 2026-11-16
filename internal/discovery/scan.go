package discovery

import (
	"context"
	"errors"
	"fmt"
	"forgeflow/internal/analyzer"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/prerequisites"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Discovery searches repositories first, then their issues. Searching issues
// across all of GitHub by "recently updated" returned mostly days-old personal
// repositories: forty screened repositories produced one opportunity. A
// repository search ordered by stars, restricted to projects that advertise
// good-first or help-wanted issues, starts from established projects whose
// maintainers have asked for outside help.

// lane is one saved search position: a language and the kind of invitation.
type lane struct {
	key       string // cursor key, shown in the UI
	language  string
	qualifier string
}

func lanes(language string) []lane {
	return []lane{
		{language, language, "good-first-issues:>0"},
		{language + " (help wanted)", language, "help-wanted-issues:>0"},
	}
}

// searchFilters identifies the search strategy in the saved-progress scope, so
// positions saved for an older strategy are not resumed.
const searchFilters = "repository-first-v1 good-first-issues help-wanted-issues sort:stars"

// How long a saved decision stands before a repository is screened again.
const excludedFor, withheldFor = 7 * 24 * time.Hour, 24 * time.Hour

// minIssueDetail is the shortest description an issue can have and still tell
// a contributor what to change.
const minIssueDetail = 80

// Labels that say an issue is not ready to be worked on, whatever else it has.
var notActionable = regexp.MustCompile(`(?i)wont ?fix|won't ?fix|invalid|duplicate|question|blocked|on[- ]hold|needs?[- /:]*(info|information|triage|design|discussion|repro|reproduction|decision|investigation|confirmation|more)|discussion|not[- ]a[- ]bug|stale|upstream|rfc|proposal|epic|tracking|pending|unconfirmed|potential|awaiting|waiting|support|can'?t[- ]reproduce|works[- ]for[- ]me`)

// Labels with which maintainers invite outside contributors. An issue carrying
// one is offered at any age; a plain bug or enhancement only while it is recent.
var invitations = []string{"good first issue", "help wanted", "easy", "beginner", "starter", "up for grabs", "contributions welcome", "low hanging fruit"}

// uninvitedFor is how long a plain bug or enhancement stays worth offering. An
// old one that maintainers of an active project have neither fixed nor marked
// for outside help is usually hard, disputed or not reproducible. An invited
// issue lasts longer, but a "good first issue" open for many years has shown
// that it is not one.
const uninvitedFor, invitedFor = 365 * 24 * time.Hour, 3 * 365 * 24 * time.Hour

// longDiscussion is the comment count past which an issue is a debate or a
// standing thread ("who is using this?") rather than a task.
const longDiscussion = 40

func invited(issue github.Issue) bool {
	for _, l := range issue.Labels {
		for _, p := range invitations {
			if labelMatches(l.Name, p) {
				return true
			}
		}
	}
	return false
}

var notSoftwareName = regexp.MustCompile(`(?i)(^|[-_.])(awesome|interview|tutorials?|cheat-?sheets?|roadmaps?|books?|courses?|leetcode)([-_.]|$)`)

// notSoftware recognizes curated lists, tutorials and study material. They
// carry "good first issue" labels and pass every build check, but a
// contribution there is an edit to a list, not a verified code change.
func notSoftware(repo github.Repository) bool {
	_, name, _ := strings.Cut(repo.FullName, "/")
	if notSoftwareName.MatchString(name) {
		return true
	}
	for _, t := range repo.Topics {
		switch strings.ToLower(t) {
		case "awesome", "awesome-list", "awesome-lists", "list", "lists", "resources", "tutorial", "tutorials", "books", "education", "interview", "interview-questions", "roadmap", "cheatsheet", "learning", "course", "leetcode", "algorithms-and-data-structures":
			return true
		}
	}
	return false
}

func labelTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// labelMatches reports whether a repository's label means a preferred label:
// "kind/bug", "type: bug" and "good-first-issue" count; "debug" does not.
func labelMatches(label, preferred string) bool {
	have, want := labelTokens(label), labelTokens(preferred)
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

// preference is the position of the best preferred label an issue carries, or
// -1 when it carries none. With no preferred labels every issue qualifies.
func preference(issue github.Issue, preferred []string) int {
	if len(preferred) == 0 {
		return 0
	}
	best := -1
	for _, l := range issue.Labels {
		for i, p := range preferred {
			if labelMatches(l.Name, p) && (best < 0 || i < best) {
				best = i
			}
		}
	}
	return best
}

// offerable applies the issue-level rules. An empty reason with ok false means
// the issue is simply not a candidate (a pull request, or no preferred label)
// and is not worth reporting.
func offerable(issue github.Issue, cfg domain.Config) (ok bool, reason string) {
	if issue.Number <= 0 || len(issue.PullRequest) > 0 && strings.TrimSpace(string(issue.PullRequest)) != "null" {
		return false, ""
	}
	if issue.State != "" && issue.State != "open" {
		return false, ""
	}
	if preference(issue, cfg.Issues.PreferredLabels) < 0 {
		return false, ""
	}
	if len(issue.Assignees) > 0 {
		// Someone is already working on an assigned issue; a parallel PR is rarely merged.
		return false, "already assigned"
	}
	if issue.Locked {
		return false, "locked"
	}
	for _, l := range issue.Labels {
		if notActionable.MatchString(l.Name) {
			return false, "labelled as not ready to work on (blocked, question, needs information or similar)"
		}
	}
	if analyzer.Excluded(issue, cfg) {
		return false, "excluded by profile (for example documentation-only)"
	}
	difficulty := analyzer.Difficulty(issue)
	suitable := false
	for _, d := range cfg.Issues.Difficulty {
		suitable = suitable || d == difficulty
	}
	if !suitable {
		return false, "outside configured difficulty"
	}
	if len(strings.TrimSpace(issue.Body)) < minIssueDetail {
		return false, "too little description to plan a change from"
	}
	if issue.Comments > longDiscussion {
		return false, "a long discussion rather than a task"
	}
	if !issue.CreatedAt.IsZero() {
		age := time.Since(issue.CreatedAt)
		if !invited(issue) && age > uninvitedFor {
			return false, "older than a year and not marked for outside help"
		}
		if age > invitedFor {
			return false, "open for more than three years"
		}
	}
	return true, ""
}

// infrastructure reports whether an error means GitHub could not be used
// (rate limit, rejected sign-in, network) rather than that one repository's
// resource was missing or broken.
func infrastructure(err error) bool {
	var api *github.APIError
	if errors.As(err, &api) {
		return !api.RetryAt.IsZero() || api.Status == 401 || api.Status == 403 || api.Status == 429
	}
	return !errors.Is(err, github.ErrBudget) && !errors.Is(err, github.ErrTooLarge)
}

func (s *Service) scan(ctx context.Context, run domain.DiscoveryRun, cfg domain.ConfigVersion) {
	credential := s.Client.Credential()
	// A signed-in scan may read more: GitHub allows 5,000 requests an hour. A
	// repository rejected on its shape costs two reads, a fully read one about
	// thirty, and each offered issue two more.
	budget := 160
	if credential != "" {
		budget = 500
	}
	session := &github.Session{Client: s.Client, MaxRequests: budget}
	skipped := map[string]int{}
	opps := []domain.Opportunity{}
	failed := false
	warn := func(err error) {
		run.Warnings = append(run.Warnings, err.Error())
		if infrastructure(err) {
			failed = true
		}
		var api *github.APIError
		if errors.As(err, &api) && !api.RetryAt.IsZero() {
			s.mu.Lock()
			if credential == s.Client.Credential() {
				s.retry = api.RetryAt
			}
			s.mu.Unlock()
		}
	}
	languages := []string{}
	for l := range cfg.Config.Profile.Languages {
		languages = append(languages, l)
	}
	sort.Slice(languages, func(i, j int) bool {
		a, b := cfg.Config.Profile.Languages[languages[i]], cfg.Config.Profile.Languages[languages[j]]
		if a != b {
			return a > b
		}
		return languages[i] < languages[j]
	})
	if len(languages) > 6 {
		run.Warnings = append(run.Warnings, "Scan bounded to the six highest-weight languages.")
		languages = languages[:6]
	}
	type group struct {
		key                 string
		page, offset, total int
		incomplete          bool
		items               []github.Repository
	}
	groups := []group{}
	searched := 0
	since := time.Now().UTC().AddDate(0, 0, -cfg.Config.Repositories.RecentActivityDays).Format("2006-01-02")
search:
	for _, lang := range languages {
		for _, l := range lanes(lang) {
			var result github.RepoSearch
			cursor := run.SearchCursors[l.key]
			if cursor.Page == 0 {
				cursor = domain.DiscoveryCursor{Page: 1}
			}
			run.SearchCursors[l.key] = cursor
			run.NextSearchCursors[l.key] = cursor
			q := `language:"` + strings.ReplaceAll(l.language, `"`, "") + `" is:public archived:false pushed:>=` + since + " " + l.qualifier
			if cfg.Config.Repositories.MinStars > 0 {
				q += fmt.Sprintf(" stars:>=%d", cfg.Config.Repositories.MinStars)
			}
			if err := session.Get(ctx, github.RepositorySearchPath(q, searchPageSize, cursor.Page), &result); err != nil {
				warn(err)
				break search
			}
			searched++
			if len(result.Items) > searchPageSize {
				result.Items = result.Items[:searchPageSize]
			}
			if cursor.Offset >= len(result.Items) {
				run.NextSearchCursors[l.key] = nextPosition(cursor.Page, len(result.Items)-1, result.Total, len(result.Items), result.Incomplete)
				continue
			}
			groups = append(groups, group{key: l.key, page: cursor.Page, offset: cursor.Offset, total: result.Total, incomplete: result.Incomplete, items: result.Items})
		}
	}
	type positioned struct {
		repo github.Repository
		key  string
		next domain.DiscoveryCursor
	}
	candidates := []positioned{}
	for index := 0; index < searchPageSize; index++ {
		for _, g := range groups {
			position := g.offset + index
			if position < len(g.items) {
				candidates = append(candidates, positioned{repo: g.items[position], key: g.key, next: nextPosition(g.page, position, g.total, len(g.items), g.incomplete)})
			}
		}
	}
	maxRepos, maxPerRepo, maxChecked := 8, 3, 30
	if credential == "" {
		maxRepos, maxPerRepo, maxChecked = 2, 2, 8
	}
	run.Warnings = append(run.Warnings, fmt.Sprintf("Scan looks for issues in up to %d contributable repositories, screens at most %d repositories, and offers at most %d issues per repository. Saved search positions continue in the next scan.", maxRepos, maxChecked, maxPerRepo))
	known, err := s.Store.RepositoryAssessments(ctx)
	if err != nil {
		known = nil
	}
	profile := s.PrerequisiteProfile.Fingerprint()
	seen := map[string]bool{}
	checked, offered, remembered, unsuitable, lists := 0, 0, 0, 0, 0
	stopped := func() bool {
		if ctx.Err() != nil || session.Requests >= session.MaxRequests {
			return true
		}
		s.mu.Lock()
		limited := time.Now().Before(s.retry)
		s.mu.Unlock()
		return limited
	}
	for _, candidate := range candidates {
		if stopped() {
			if ctx.Err() == nil && session.Requests >= session.MaxRequests {
				run.Warnings = append(run.Warnings, "This scan's request allowance is used; the next scan continues from the saved positions.")
			}
			break
		}
		repo := candidate.repo
		name := repo.FullName
		before := run.NextSearchCursors[candidate.key]
		run.NextSearchCursors[candidate.key] = candidate.next
		if !repoPattern.MatchString(name) || strings.Contains(name, "..") || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		run.Candidates++
		if repo.Private || repo.Archived || repo.Disabled || repo.DefaultBranch == "" || repo.Stars < cfg.Config.Repositories.MinStars || analyzer.ProfileWeight(cfg.Config.Profile.Languages, repo.Language) == 0 || time.Since(repo.PushedAt) > time.Duration(cfg.Config.Repositories.RecentActivityDays)*24*time.Hour {
			unsuitable++
			continue
		}
		if notSoftware(repo) {
			lists++
			continue
		}
		// A repository excluded days ago under the same rules is still excluded;
		// screening it again would spend the allowance on a known answer.
		if prior, ok := known[strings.ToLower(name)]; ok && prior.Profile == profile {
			age := time.Since(prior.CheckedAt)
			if prior.Status == "excluded" && age < excludedFor || prior.Status == "needs_assessment" && age < withheldFor {
				remembered++
				continue
			}
		}
		if offered >= maxRepos || checked >= maxChecked {
			run.NextSearchCursors[candidate.key] = before
			run.Warnings = append(run.Warnings, "Repository scan limit reached; the next scan resumes at unprocessed candidates.")
			break
		}
		checked++
		e := s.repositoryEvidence(ctx, session, repo, warn)
		if stopped() {
			// The assessment may be cut short; neither record it nor skip the repository.
			run.NextSearchCursors[candidate.key] = before
			break
		}
		if e.Prerequisites != nil {
			run.RepositoryAssessments = append(run.RepositoryAssessments, *e.Prerequisites)
		}
		if e.Prerequisites == nil || e.Prerequisites.Status != "eligible" {
			if a := e.Prerequisites; a != nil {
				v := domain.RepositoryExclusion{Repository: name, HeadSHA: a.HeadSHA, Status: a.Status, DetectedAt: a.CheckedAt, Checks: a.Findings, NextAction: "Inspect the prerequisite evidence, then run discovery again after the requirements or runner capabilities change."}
				if len(a.Findings) > 0 {
					v.Kind = a.Findings[0].Kind
					v.Reason = a.Findings[0].Reason
				}
				run.ExcludedRepositories = append(run.ExcludedRepositories, v)
			}
			continue
		}
		issues := s.repositoryIssues(ctx, session, repo, cfg.Config, maxPerRepo, skipped, warn)
		accepted := 0
		for _, issue := range issues {
			if stopped() {
				break
			}
			o, ok := s.opportunity(ctx, session, repo, issue, e, run.ID, cfg, skipped, warn)
			if !ok {
				continue
			}
			opps = append(opps, o)
			accepted++
			if accepted == maxPerRepo {
				break
			}
		}
		if accepted > 0 {
			offered++
		} else if !stopped() {
			skipped["contributable repositories with no suitable open issue"]++
		}
	}
	if unsuitable > 0 {
		run.Warnings = append(run.Warnings, fmt.Sprintf("%d repositories did not match the profile (language, stars or recent activity).", unsuitable))
	}
	if lists > 0 {
		run.Warnings = append(run.Warnings, fmt.Sprintf("%d repositories are lists, tutorials or study material rather than software.", lists))
	}
	if remembered > 0 {
		run.Warnings = append(run.Warnings, fmt.Sprintf("%d repositories excluded by a recent assessment were not screened again.", remembered))
	}
	if len(skipped) > 0 {
		reasons := make([]string, 0, len(skipped))
		for reason, n := range skipped {
			reasons = append(reasons, fmt.Sprintf("%d %s", n, reason))
		}
		sort.Strings(reasons)
		run.Warnings = append(run.Warnings, "Not offered: "+strings.Join(reasons, "; ")+".")
	}
	run.Accepted = len(opps)
	run.Requests = session.Requests
	// A scan that screened repositories and found nothing new to offer has
	// succeeded. PARTIAL means GitHub cut it short; FAILED means it could not
	// search at all.
	run.Status = "SUCCEEDED"
	if failed {
		run.Status = "PARTIAL"
	}
	if ctx.Err() != nil {
		run.Status = "CANCELLED"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			run.Status = "TIMED_OUT"
		}
		run.Warnings = append(run.Warnings, "Scan stopped; only completed observations were retained.")
	} else if searched == 0 && len(languages) > 0 {
		run.Status = "FAILED"
	}
	t := time.Now().UTC()
	run.FinishedAt = &t
	// A cancelled client or server must not erase the durable audit completion record.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Store.FinishDiscovery(saveCtx, run, opps); err != nil { // No successful completion is recorded; recovery marks the durable RUNNING record interrupted.
		slog.Error("Discovery completion could not be persisted; automatic scans paused")
		s.mu.Lock()
		s.automatic = false
		s.mu.Unlock()
	}
}

// repositoryIssues returns the repository's open, unassigned issues worth
// offering, best first: by preferred-label order, then most recently updated.
func (s *Service) repositoryIssues(ctx context.Context, session *github.Session, repo github.Repository, cfg domain.Config, want int, skipped map[string]int, warn func(error)) []github.Issue {
	base := "/repos/" + repo.FullName + "/issues?state=open&assignee=none&sort=updated&direction=desc"
	found := map[int]bool{}
	out := []github.Issue{}
	collect := func(path string) bool {
		var page []github.Issue
		if err := session.Get(ctx, path, &page); err != nil {
			warn(err)
			return false
		}
		for _, issue := range page {
			if found[issue.Number] {
				continue
			}
			found[issue.Number] = true
			ok, reason := offerable(issue, cfg)
			if reason != "" {
				skipped[reason]++
			}
			if ok {
				out = append(out, issue)
			}
		}
		return true
	}
	if !collect(base + "&per_page=100") {
		return nil
	}
	// In a busy repository the hundred most recently updated issues may hold
	// none of the invited ones; ask for the first two preferred labels by name.
	for i := 0; i < len(cfg.Issues.PreferredLabels) && i < 2 && len(out) < want; i++ {
		collect(base + "&per_page=30&labels=" + url.QueryEscape(cfg.Issues.PreferredLabels[i]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		// An invitation outranks label order: it is the maintainers saying the
		// issue is ready for someone outside the project.
		if ia, ib := invited(out[i]), invited(out[j]); ia != ib {
			return ia
		}
		a, b := preference(out[i], cfg.Issues.PreferredLabels), preference(out[j], cfg.Issues.PreferredLabels)
		if a != b {
			return a < b
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	// Read a few more than wanted: some will turn out to have competing work.
	if len(out) > want*3 {
		out = out[:want*3]
	}
	return out
}

// opportunity completes one issue's evidence from its timeline (maintainer
// comments and pull requests that mention it) and ranks it.
func (s *Service) opportunity(ctx context.Context, session *github.Session, repo github.Repository, issue github.Issue, base domain.Evidence, runID string, cfg domain.ConfigVersion, skipped map[string]int, warn func(error)) (domain.Opportunity, bool) {
	name := repo.FullName
	e := base
	e.Warnings = append([]string{}, base.Warnings...)
	e.IssueURL = fmt.Sprintf("https://github.com/%s/issues/%d", name, issue.Number)
	e.Description = issue.Body
	e.IssueState = "open"
	e.IssueCreatedAt = issue.CreatedAt
	e.IssueUpdatedAt = issue.UpdatedAt
	e.CommentCount = issue.Comments
	e.Assignees = []string{}
	e.CompetingPRs = []string{}
	e.RunID = runID
	e.ConfigVersion = cfg.Version
	e.ObservedAt = time.Now().UTC()
	var events []github.TimelineEvent
	if err := session.Get(ctx, fmt.Sprintf("/repos/%s/issues/%d/timeline?per_page=100", name, issue.Number), &events); err != nil {
		e.Warnings = append(e.Warnings, "Issue timeline unavailable, so maintainer participation and competing pull requests are unknown: "+err.Error())
		warn(err)
	} else {
		n, comments := 0, 0
		for _, ev := range events {
			switch ev.Event {
			case "commented":
				comments++
				if maintainer(ev.AuthorAssociation) {
					n++
				}
			case "cross-referenced":
				if ev.Source != nil && ev.Source.Issue != nil && ev.Source.Issue.State == "open" && len(ev.Source.Issue.PullRequest) > 0 && strings.TrimSpace(string(ev.Source.Issue.PullRequest)) != "null" {
					e.CompetingPRs = append(e.CompetingPRs, ev.Source.Issue.HTMLURL)
				}
			}
		}
		e.MaintainerComments = &n
		e.CommentsSampled = comments
		e.CompetitionChecked = len(events) < 100
		if !e.CompetitionChecked {
			e.Warnings = append(e.Warnings, "The issue has a long history; only its first 100 events were checked for competing pull requests and maintainer comments.")
		}
	}
	if len(e.CompetingPRs) > 0 {
		skipped["an open pull request already mentions it"]++
		return domain.Opportunity{}, false
	}
	// Issue templates add "bug" to every report. Without an invitation, a
	// second label or a maintainer's reply, nobody on the project has yet
	// agreed that there is something to fix.
	if !invited(issue) && len(issue.Labels) < 2 && (e.MaintainerComments == nil || *e.MaintainerComments == 0) {
		skipped["a report no maintainer has confirmed yet"]++
		return domain.Opportunity{}, false
	}
	issue.State = "open"
	o, err := analyzer.Analyze(repo, issue, e, cfg.Config, time.Now())
	if err != nil {
		warn(err)
		return domain.Opportunity{}, false
	}
	return o, true
}

func maintainer(association string) bool {
	return association == "OWNER" || association == "MEMBER" || association == "COLLABORATOR"
}

func (s *Service) repositoryEvidence(ctx context.Context, session *github.Session, r github.Repository, warn func(error)) domain.Evidence {
	e := domain.Evidence{RepositoryURL: "https://github.com/" + r.FullName, Topics: r.Topics, Stars: r.Stars, Forks: r.Forks, Archived: r.Archived, DefaultBranch: r.DefaultBranch, BuildFiles: []string{}, Warnings: []string{}}
	var commits []github.Commit
	if err := session.Get(ctx, "/repos/"+r.FullName+"/commits?per_page=1", &commits); err != nil {
		e.Warnings = append(e.Warnings, "Latest commit unavailable: "+err.Error())
		warn(err)
	} else if len(commits) > 0 {
		e.HeadSHA = commits[0].SHA
		t := commits[0].Commit.Committer.Date
		e.LastCommit = &t
	}
	var tree github.Tree
	if e.HeadSHA != "" {
		err := session.Get(ctx, "/repos/"+r.FullName+"/git/trees/"+url.PathEscape(e.HeadSHA)+"?recursive=1", &tree)
		// GitHub truncates very large trees, and larger ones exceed the read
		// limit. Both are a size decision, not a failed scan.
		if errors.Is(err, github.ErrTooLarge) || err == nil && tree.Truncated {
			a := prerequisites.Oversized(r.FullName, e.HeadSHA, s.PrerequisiteProfile)
			e.Prerequisites = &a
			return e
		}
		if err != nil {
			tree.Truncated = true
			e.Warnings = append(e.Warnings, "Repository tree unavailable: "+err.Error())
			warn(err)
		} else {
			analyzer.InspectTree(tree, &e)
		}
	}
	a := prerequisites.InspectRepository(ctx, session, r, e.HeadSHA, tree, s.PrerequisiteProfile, warn)
	e.Prerequisites = &a
	if a.Status != "eligible" {
		return e
	}
	var releases []github.Release
	if err := session.Get(ctx, "/repos/"+r.FullName+"/releases?per_page=5", &releases); err != nil {
		e.Warnings = append(e.Warnings, "Release activity unavailable: "+err.Error())
		warn(err)
	} else {
		for _, release := range releases {
			if !release.Draft && !release.Prerelease && release.PublishedAt != nil {
				e.LatestRelease = release.PublishedAt
				break
			}
		}
		if e.LatestRelease == nil {
			e.Warnings = append(e.Warnings, "No stable release observed in the latest five releases; historical releases may exist.")
		}
	}
	var prs []github.Pull
	if err := session.Get(ctx, "/repos/"+r.FullName+"/pulls?state=closed&sort=updated&direction=desc&per_page=30", &prs); err != nil {
		e.Warnings = append(e.Warnings, "External PR acceptance unavailable: "+err.Error())
		warn(err)
	} else {
		n := 0
		for _, pr := range prs {
			if pr.MergedAt != nil {
				e.MergedPRsSampled++
				if !maintainer(pr.AuthorAssociation) {
					n++
				}
			}
		}
		e.ExternalPRsMerged = &n
	}
	return e
}
