package discovery

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/prerequisites"
	"forgeflow/internal/storage"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrBusy = errors.New("a discovery scan is already running")

// scanDeadline bounds one scan. Reads are serial and take most of a second
// each, so a signed-in scan of 500 reads needs several minutes.
const scanDeadline = 8 * time.Minute

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
	PrerequisiteProfile prerequisites.Profile
	Store               *storage.Store
	Client              *github.Client
	AuthSource          string
	Interval            time.Duration
	ctx                 context.Context
	mu                  sync.Mutex
	running             bool
	automatic           bool
	next                time.Time
	retry               time.Time
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

func New(ctx context.Context, s *storage.Store, c *github.Client, auth string, interval time.Duration) *Service {
	return &Service{Store: s, Client: c, AuthSource: auth, Interval: interval, ctx: ctx, automatic: interval > 0, next: time.Now(), PrerequisiteProfile: prerequisites.LinuxProfile()}
}
func (s *Service) CredentialChanged(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AuthSource = source
	s.retry = time.Time{}
	s.next = time.Now()
	if s.cancel != nil {
		s.cancel()
	}
}
func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	out := Status{Available: !s.Store.Demo, Running: s.running, Automatic: s.automatic, Authentication: s.AuthSource, IntervalSeconds: int(s.Interval.Seconds()), Message: "Public repositories only. Scans start from established projects that invite outside help and continue from saved search progress; no repository code is executed."}
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
	scope := s.searchScope(cfg.Config)
	cursors, err := s.Store.DiscoveryCursors(s.ctx, scope)
	if err != nil {
		return domain.DiscoveryRun{}, err
	}
	nextCursors := map[string]domain.DiscoveryCursor{}
	for lang, c := range cursors {
		nextCursors[lang] = c
	}
	run := domain.DiscoveryRun{ID: storage.ID(), Status: "RUNNING", ConfigVersion: cfg.Version, StartedAt: time.Now().UTC(), Warnings: []string{}, SearchScope: scope, SearchCursors: cursors, NextSearchCursors: nextCursors}
	if err = s.Store.StartDiscovery(s.ctx, run); err != nil {
		return run, err
	}
	ctx, cancel := context.WithTimeout(s.ctx, scanDeadline)
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
