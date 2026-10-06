package discovery

import (
	"context"
	"errors"
	"fmt"
	"forgeflow/internal/analyzer"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrBusy = errors.New("a discovery scan is already running")

type Status struct {
	Available       bool                 `json:"available"`
	Running         bool                 `json:"running"`
	Automatic       bool                 `json:"automatic"`
	Authentication  string               `json:"authentication"`
	IntervalSeconds int                  `json:"interval_seconds"`
	NextRun         *time.Time           `json:"next_run"`
	RetryAt         *time.Time           `json:"retry_at"`
	LastRun         *domain.DiscoveryRun `json:"last_run"`
	Message         string               `json:"message"`
}
type Service struct {
	Store      *storage.Store
	Client     *github.Client
	AuthSource string
	Interval   time.Duration
	ctx        context.Context
	mu         sync.Mutex
	running    bool
	automatic  bool
	next       time.Time
	retry      time.Time
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func New(ctx context.Context, s *storage.Store, c *github.Client, auth string, interval time.Duration) *Service {
	return &Service{Store: s, Client: c, AuthSource: auth, Interval: interval, ctx: ctx, automatic: interval > 0, next: time.Now()}
}
func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	out := Status{Available: !s.Store.Demo, Running: s.running, Automatic: s.automatic, Authentication: s.AuthSource, IntervalSeconds: int(s.Interval.Seconds()), Message: "Public repositories only. Bounded samples; no repository code is executed."}
	if s.automatic {
		t := s.next
		out.NextRun = &t
	}
	if time.Now().Before(s.retry) {
		t := s.retry
		out.RetryAt = &t
	}
	s.mu.Unlock()
	if s.Store.Demo {
		out.Available = false
		out.Automatic = false
		out.NextRun = nil
		out.Authentication = "disabled in demo"
		out.Message = "Demo uses illustrative data. Start live mode for GitHub discovery."
	}
	runs, err := s.Store.DiscoveryRuns(ctx)
	if err != nil {
		return out, err
	}
	if len(runs) > 0 {
		out.LastRun = &runs[0]
	}
	return out, nil
}
func (s *Service) Start() (domain.DiscoveryRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Store.Demo {
		return domain.DiscoveryRun{}, errors.New("start live mode to discover real GitHub issues")
	}
	if s.running {
		return domain.DiscoveryRun{}, ErrBusy
	}
	if s.ctx.Err() != nil {
		return domain.DiscoveryRun{}, errors.New("server is stopping")
	}
	if time.Now().Before(s.retry) {
		return domain.DiscoveryRun{}, &github.APIError{Status: 429, RetryAt: s.retry}
	}
	cfg, err := s.Store.CurrentConfig(s.ctx)
	if err != nil {
		return domain.DiscoveryRun{}, err
	}
	run := domain.DiscoveryRun{ID: storage.ID(), Status: "RUNNING", ConfigVersion: cfg.Version, StartedAt: time.Now().UTC(), Warnings: []string{}}
	if err = s.Store.StartDiscovery(s.ctx, run); err != nil {
		return run, err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Minute)
	s.cancel = cancel
	s.running = true
	s.next = time.Now().Add(s.Interval)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.scan(ctx, run, cfg)
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()
	return run, nil
}
func (s *Service) SetAutomatic(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Store.Demo {
		return errors.New("automatic live discovery is disabled in demo mode")
	}
	if enabled && s.Interval <= 0 {
		return errors.New("restart the server with a positive discovery interval to enable automatic scans")
	}
	if err := s.Store.SetDiscoveryAutomatic(s.ctx, enabled); err != nil {
		return err
	}
	s.automatic = enabled
	s.next = time.Now().Add(s.Interval)
	return nil
}
func (s *Service) RestoreSchedule() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Store.Demo {
		return nil
	}
	preference, err := s.Store.DiscoveryAutomatic(s.ctx)
	if err != nil {
		return err
	}
	if preference != nil {
		s.automatic = *preference && s.Interval > 0
	}
	return nil
}
func (s *Service) RequestCancellation(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return nil
	}
	if err := s.Store.RequestDiscoveryCancellation(ctx); err != nil {
		return err
	}
	s.cancel()
	return nil
}
func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}
func (s *Service) Wait() { s.wg.Wait() }
func (s *Service) Schedule() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				s.mu.Lock()
				due := s.automatic && !s.running && !s.Store.Demo && !time.Now().Before(s.next) && !time.Now().Before(s.retry)
				s.mu.Unlock()
				if due {
					_, _ = s.Start()
				}
			}
		}
	}()
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func RepositoryName(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Host != "api.github.com" || u.Scheme != "https" || !strings.HasPrefix(u.Path, "/repos/") {
		return ""
	}
	name := strings.TrimPrefix(u.Path, "/repos/")
	if !repoPattern.MatchString(name) {
		return ""
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." {
			return ""
		}
	}
	return name
}
func preferredLabels(labels []string) string {
	out := []string{}
	for _, label := range labels {
		if len(out) == 8 {
			break
		}
		out = append(out, `"`+strings.ReplaceAll(strings.ReplaceAll(label, `"`, ""), "\\", "")+`"`)
	}
	if len(out) == 0 {
		return ""
	}
	return " label:" + strings.Join(out, ",")
}
func (s *Service) scan(ctx context.Context, run domain.DiscoveryRun, cfg domain.ConfigVersion) {
	session := &github.Session{Client: s.Client, MaxRequests: 80}
	opps := []domain.Opportunity{}
	hadError := false
	warn := func(err error) {
		hadError = true
		run.Warnings = append(run.Warnings, err.Error())
		var api *github.APIError
		if errors.As(err, &api) && !api.RetryAt.IsZero() {
			s.mu.Lock()
			s.retry = api.RetryAt
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
	groups := [][]github.Issue{}
	for _, lang := range languages {
		var result github.IssueSearch
		q := `is:issue is:open is:public archived:false language:"` + strings.ReplaceAll(lang, `"`, "") + `"` + preferredLabels(cfg.Config.Issues.PreferredLabels)
		if err := session.Get(ctx, github.SearchPath("issues", q, 6), &result); err != nil {
			warn(err)
			break
		}
		if result.Incomplete || result.Total > len(result.Items) {
			run.Warnings = append(run.Warnings, "Search for "+lang+" is a bounded sample, not an exhaustive issue list.")
		}
		groups = append(groups, result.Items)
	}
	candidates := []github.Issue{}
	for index := 0; index < 6; index++ {
		for _, group := range groups {
			if index < len(group) {
				candidates = append(candidates, group[index])
			}
		}
	}
	maxRepos, maxPerRepo := 5, 3
	if s.Client.Token == "" {
		maxRepos, maxPerRepo = 2, 2
	}
	run.Warnings = append(run.Warnings, fmt.Sprintf("Scan limited to %d repositories and %d accepted issues per repository; discovery does not exhaust all matching issues.", maxRepos, maxPerRepo))
	type snapshot struct {
		repo github.Repository
		e    domain.Evidence
	}
	repos := map[string]snapshot{}
	blocked := map[string]bool{}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		name := RepositoryName(candidate.RepositoryURL)
		if name == "" || candidate.Number <= 0 || len(candidate.PullRequest) > 0 {
			continue
		}
		key := fmt.Sprintf("%s:%d", name, candidate.Number)
		if seen[key] || blocked[name] || counts[name] >= maxPerRepo {
			continue
		}
		seen[key] = true
		run.Candidates++
		snap, ok := repos[name]
		if !ok {
			if len(repos) >= maxRepos {
				continue
			}
			var repo github.Repository
			if err := session.Get(ctx, "/repos/"+name, &repo); err != nil {
				warn(err)
				blocked[name] = true
				continue
			}
			if repo.FullName != name || repo.Private || repo.Archived || repo.Disabled || repo.Stars < cfg.Config.Repositories.MinStars || analyzer.ProfileWeight(cfg.Config.Profile.Languages, repo.Language) == 0 || time.Since(repo.PushedAt) > time.Duration(cfg.Config.Repositories.RecentActivityDays)*24*time.Hour {
				blocked[name] = true
				continue
			}
			e := s.repositoryEvidence(ctx, session, repo, warn)
			snap = snapshot{repo: repo, e: e}
			repos[name] = snap
		}
		var issue github.Issue
		if err := session.Get(ctx, fmt.Sprintf("/repos/%s/issues/%d", name, candidate.Number), &issue); err != nil {
			warn(err)
			continue
		}
		if issue.State != "open" || len(issue.PullRequest) > 0 || analyzer.Excluded(issue, cfg.Config) {
			continue
		}
		if issue.Number != candidate.Number {
			warn(errors.New("GitHub returned mismatched issue metadata"))
			continue
		}
		difficulty := analyzer.Difficulty(issue)
		suitable := false
		for _, d := range cfg.Config.Issues.Difficulty {
			if d == difficulty {
				suitable = true
			}
		}
		if !suitable {
			continue
		}
		e := snap.e
		e.Warnings = append([]string{}, snap.e.Warnings...)
		e.IssueURL = fmt.Sprintf("https://github.com/%s/issues/%d", name, issue.Number)
		e.Description = issue.Body
		e.IssueState = issue.State
		e.IssueCreatedAt = issue.CreatedAt
		e.IssueUpdatedAt = issue.UpdatedAt
		e.CommentCount = issue.Comments
		e.Assignees = []string{}
		for _, u := range issue.Assignees {
			e.Assignees = append(e.Assignees, u.Login)
		}
		e.CompetingPRs = []string{}
		e.RunID = run.ID
		e.ConfigVersion = cfg.Version
		e.ObservedAt = time.Now().UTC()
		if issue.Comments == 0 {
			n := 0
			e.MaintainerComments = &n
		} else {
			var comments []github.Comment
			if err := session.Get(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=20", name, issue.Number), &comments); err != nil {
				e.Warnings = append(e.Warnings, "Issue comments unavailable: "+err.Error())
				warn(err)
			} else {
				n := 0
				for _, comment := range comments {
					if maintainer(comment.AuthorAssociation) {
						n++
					}
				}
				e.MaintainerComments = &n
				e.CommentsSampled = len(comments)
				if issue.Comments > len(comments) {
					e.Warnings = append(e.Warnings, "Maintainer participation uses the first 20 comments only.")
				}
			}
		}
		var prs github.IssueSearch
		q := fmt.Sprintf(`repo:%s is:pr is:open "%d" in:body`, name, issue.Number)
		if err := session.Get(ctx, github.SearchPath("issues", q, 10), &prs); err != nil {
			e.Warnings = append(e.Warnings, "Competing PR search unavailable: "+err.Error())
			warn(err)
		} else {
			e.CompetitionChecked = !prs.Incomplete && prs.Total <= len(prs.Items)
			ref := regexp.MustCompile(fmt.Sprintf(`(^|[^A-Za-z0-9])#%d([^0-9]|$)`, issue.Number))
			urlRef := regexp.MustCompile(regexp.QuoteMeta(e.IssueURL) + `([^0-9]|$)`)
			for _, pr := range prs.Items {
				if pr.Number > 0 && (ref.MatchString(pr.Body) || urlRef.MatchString(pr.Body)) {
					e.CompetingPRs = append(e.CompetingPRs, fmt.Sprintf("https://github.com/%s/pull/%d", name, pr.Number))
				}
			}
			if !e.CompetitionChecked {
				e.Warnings = append(e.Warnings, "Competing PR search is incomplete; no-competition conclusions are unavailable.")
			}
		}
		o, err := analyzer.Analyze(snap.repo, issue, e, cfg.Config, time.Now())
		if err != nil {
			warn(err)
			continue
		}
		opps = append(opps, o)
		counts[name]++
	}
	run.Accepted = len(opps)
	run.Requests = session.Requests
	run.Status = "SUCCEEDED"
	// Sampling notices are expected. Infrastructure failures make a scan partial rather than falsely successful.
	if hadError {
		run.Status = "PARTIAL"
	}
	if ctx.Err() != nil {
		run.Status = "CANCELLED"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			run.Status = "TIMED_OUT"
		}
		run.Warnings = append(run.Warnings, "Scan stopped; only completed observations were retained.")
	}
	if len(opps) == 0 && run.Status == "PARTIAL" {
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
	if e.HeadSHA != "" {
		var tree github.Tree
		if err := session.Get(ctx, "/repos/"+r.FullName+"/git/trees/"+url.PathEscape(e.HeadSHA)+"?recursive=1", &tree); err != nil {
			e.Warnings = append(e.Warnings, "Repository tree unavailable: "+err.Error())
			warn(err)
		} else {
			analyzer.InspectTree(tree, &e)
		}
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
