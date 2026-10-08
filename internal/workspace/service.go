// Package workspace prepares only repositories explicitly approved by a human.
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/codex"
	"forgeflow/internal/contributions"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var repositoryName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var commitSHA = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
var ErrChanged = errors.New("issue, repository or configuration changed; review the fresh approval preview again")

type Service struct {
	Store        *storage.Store
	Client       *github.Client
	Root         string
	Base         string
	DefaultModel string
	ctx          context.Context
	mu           sync.Mutex
	running      int
	wg           sync.WaitGroup
	// Git is resolved on the server, never supplied by API callers.
	Git            string
	selfRepository string
	Prepared       func(context.Context, domain.Contribution) error
}

func New(ctx context.Context, s *storage.Store, c *github.Client, root string) *Service {
	git, _ := exec.LookPath("git")
	manager := &Service{Store: s, Client: c, Root: root, Base: filepath.Join(root, "contributions"), ctx: ctx, Git: git}
	if git != "" {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		probe := exec.CommandContext(probeCtx, git, "-C", root, "config", "--get", "remote.origin.url")
		probe.Env = gitEnvironment("")
		if out, err := probe.Output(); err == nil {
			remote := strings.TrimSuffix(strings.TrimSpace(string(out)), ".git")
			for _, prefix := range []string{"https://github.com/", "http://github.com/", "git@github.com:", "ssh://git@github.com/"} {
				if strings.HasPrefix(remote, prefix) {
					manager.selfRepository = strings.TrimPrefix(remote, prefix)
					break
				}
			}
		}
	}
	return manager
}
func (s *Service) Wait() { s.wg.Wait() }
func (s *Service) Preview(ctx context.Context, id string) (domain.WorkspaceApproval, error) {
	return s.PreviewForModel(ctx, id, "")
}
func (s *Service) PreviewForModel(ctx context.Context, id, requestedModel string) (domain.WorkspaceApproval, error) {
	var a domain.WorkspaceApproval
	if err := codex.ValidateModelID(requestedModel); err != nil {
		return a, err
	}
	model := requestedModel
	if model == "" {
		model = s.DefaultModel
	}
	if err := codex.ValidateModelID(model); err != nil {
		return a, err
	}
	if s.Store.Demo {
		return a, errors.New("demo issues cannot create external workspaces")
	}
	if s.Git == "" {
		return a, errors.New("Git executable is unavailable on the server")
	}
	o, err := s.Store.Opportunity(ctx, id)
	if err != nil {
		return a, err
	}
	if o.Demo || o.Evidence == nil || !repositoryName.MatchString(o.Repository) || o.Number < 1 {
		return a, errors.New("a real discovered GitHub issue is required")
	}
	if s.selfRepository != "" && strings.EqualFold(o.Repository, s.selfRepository) {
		return a, errors.New("ForgeFlow itself cannot be allocated as its own contribution workspace")
	}
	cfg, err := s.Store.CurrentConfig(ctx)
	if err != nil {
		return a, err
	}
	session := github.Session{Client: s.Client, MaxRequests: 5, Fresh: true}
	endpoint := "/repos/" + o.Repository
	var repo github.Repository
	var issue github.Issue
	var commits []github.Commit
	if err = session.Get(ctx, endpoint, &repo); err != nil {
		return a, err
	}
	if !strings.EqualFold(repo.FullName, o.Repository) || repo.Private || repo.Archived || repo.Disabled {
		return a, errors.New("repository is renamed, private, archived or disabled; rediscover before proceeding")
	}
	if err = session.Get(ctx, endpoint+"/issues/"+strconv.Itoa(o.Number), &issue); err != nil {
		return a, err
	}
	if issue.Number != o.Number || issue.State != "open" || (len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null") {
		return a, errors.New("issue is no longer an open contribution opportunity")
	}
	if !strings.EqualFold(issue.HTMLURL, fmt.Sprintf("https://github.com/%s/issues/%d", o.Repository, o.Number)) {
		return a, errors.New("GitHub issue identity changed; rediscover before proceeding")
	}
	if err = session.Get(ctx, endpoint+"/commits?per_page=1", &commits); err != nil {
		return a, err
	}
	if len(commits) != 1 || !commitSHA.MatchString(commits[0].SHA) {
		return a, errors.New("repository default branch commit unavailable")
	}
	// Never present discovery's older competition observations as fresh.
	var competition github.IssueSearch
	if err = session.Get(ctx, github.SearchPath("issues", fmt.Sprintf("repo:%s is:pr is:open \"%s\"", o.Repository, issue.HTMLURL), 10), &competition); err != nil {
		return a, err
	}
	a = domain.WorkspaceApproval{Opportunity: o, Config: cfg, BaseCommit: commits[0].SHA, CodexModel: model, CheckedAt: time.Now().UTC(), Warnings: []string{"Preparation clones and branches the repository. Codex coding, repository tests and PR submission are not started by this approval."}}
	a.WorkspaceRoot = s.Base
	if s.Prepared != nil {
		a.Warnings = []string{"Approval starts isolated cloning, Codex coding, real verification and independent review. PR submission requires a later explicit approval."}
	}
	a.Issue, err = json.Marshal(issue)
	if err != nil {
		return a, err
	}
	a.Repository, err = json.Marshal(repo)
	if err != nil {
		return a, err
	}
	a.Competition, err = json.Marshal(competition)
	if err != nil {
		return a, err
	}
	if len(issue.Assignees) > 0 {
		a.Warnings = append(a.Warnings, "The issue has assignees; coordinate with maintainers before implementing.")
	}
	if competition.Total > 0 {
		a.Warnings = append(a.Warnings, fmt.Sprintf("%d open PR search matches reference this issue; check for competing work.", competition.Total))
	}
	if competition.Incomplete {
		a.Warnings = append(a.Warnings, "GitHub competition search was incomplete.")
	}
	// Include all immutable inputs and warnings but exclude the check timestamp.
	stable := a
	stable.CheckedAt = time.Time{}
	b, err := json.Marshal(stable)
	if err != nil {
		return a, err
	}
	hash := sha256.Sum256(b)
	a.Token = hex.EncodeToString(hash[:])
	return a, nil
}
func (s *Service) Proceed(ctx context.Context, id, token string, approved bool) (domain.Contribution, error) {
	return s.ProceedWithExecution(ctx, id, token, approved, false)
}
func (s *Service) ProceedWithExecution(ctx context.Context, id, token string, approved, execute bool) (domain.Contribution, error) {
	return s.ProceedWithExecutionForModel(ctx, id, token, approved, execute, "")
}
func (s *Service) ProceedWithExecutionForModel(ctx context.Context, id, token string, approved, execute bool, model string) (domain.Contribution, error) {
	var c domain.Contribution
	if !approved || len(token) != 64 {
		return c, errors.New("explicit human approval and a reviewed preview token are required")
	}
	if err := codex.ValidateModelID(model); err != nil {
		return c, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return c, errors.New("server is stopping")
	}
	if previous, err := s.Store.ApprovedContribution(ctx, token); err == nil {
		selected := model
		if selected == "" {
			selected = s.DefaultModel
		}
		if previous.OpportunityID != id || previous.CodexModel != selected {
			return c, ErrChanged
		}
		return previous, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return c, err
	}
	a, err := s.PreviewForModel(ctx, id, model)
	if err != nil {
		return c, err
	}
	if a.Token != token {
		return c, ErrChanged
	}
	if s.running >= a.Config.Config.Agents.MaxContributors {
		return c, errors.New("workspace preparation concurrency limit reached")
	}
	cid := "contribution-" + storage.ID()
	repoPath, err := contributions.WorkspacePathAt(s.Base, cid)
	if err != nil {
		return c, err
	}
	workspacePath := repoPath
	if s.Base == filepath.Join(s.Root, "contributions") {
		workspacePath = filepath.Join("contributions", cid, "repo")
	}
	c = domain.Contribution{ID: cid, OpportunityID: id, Repository: a.Opportunity.Repository, Title: a.Opportunity.Title, State: "SELECTED", Branch: fmt.Sprintf("autopilot/issue-%d", a.Opportunity.Number), ConfigVersion: a.Config.Version, CodexModel: a.CodexModel, UpdatedAt: time.Now().UTC(), Workspace: filepath.ToSlash(workspacePath), BaseCommit: a.BaseCommit, Message: "Human approved; preparing isolated workspace"}
	c.ExecutionApproved = execute
	if err = s.Store.ApproveWorkspace(ctx, c, a); err != nil {
		return c, err
	}
	s.running++
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { s.mu.Lock(); s.running--; s.mu.Unlock() }()
		s.prepare(c, a, repoPath)
	}()
	return c, nil
}
func (s *Service) prepare(c domain.Contribution, a domain.WorkspaceApproval, repoPath string) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()
	err := s.prepareWorkspace(ctx, c, a, repoPath)
	// Retain partial clones and all records for inspection; never delete on failure.
	auditCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err != nil {
		if e := s.Store.WorkspaceMessage(auditCtx, c.ID, err.Error()); e != nil {
			slog.Error("cannot record preparation failure", "contribution", c.ID, "error", e)
		}
		state := "FAILED"
		if ctx.Err() != nil {
			state = "BLOCKED"
		}
		if e := s.Store.Transition(auditCtx, c.ID, state); e != nil {
			slog.Error("cannot record preparation state", "contribution", c.ID, "error", e)
		}
	}
	if e := s.exportEvents(auditCtx, c.ID, filepath.Join(filepath.Dir(repoPath), ".autopilot")); e != nil {
		slog.Warn("cannot export workspace events; SQLite remains authoritative", "contribution", c.ID, "error", e)
	}
}
func (s *Service) prepareWorkspace(ctx context.Context, c domain.Contribution, a domain.WorkspaceApproval, repoPath string) error {
	if err := s.Store.Transition(ctx, c.ID, "PREPARING"); err != nil {
		return err
	}
	parent := filepath.Dir(repoPath)
	if err := createAt(s.Base, parent); err != nil {
		return errors.New("cannot create an isolated contribution directory; inspect contributions for links or existing directories")
	}
	meta := filepath.Join(parent, ".autopilot")
	artifacts := filepath.Join(parent, "artifacts")
	for _, dir := range []string{meta, artifacts, filepath.Join(meta, "empty-hooks")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return errors.New("cannot create contribution metadata directories")
		}
	}
	files := map[string]any{"metadata.json": c, "issue.json": json.RawMessage(a.Issue), "repository.json": json.RawMessage(a.Repository), "configuration-snapshot.json": a.Config, "approval.json": a}
	for name, v := range files {
		if err := writeJSON(filepath.Join(meta, name), v); err != nil {
			return errors.New("cannot persist workspace snapshots")
		}
	}
	// Empty templates/hooks and a clean environment prevent clone/checkout from executing local hooks or global filters.
	template := filepath.Join(meta, "empty-template")
	if err := os.Mkdir(template, 0700); err != nil {
		return errors.New("cannot create Git template directory")
	}
	base := []string{"-c", "core.hooksPath=" + filepath.Join(meta, "empty-hooks"), "-c", "credential.helper=", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "submodule.recurse=false", "-c", "core.symlinks=false"}
	remote := "https://github.com/" + c.Repository + ".git"
	if _, err := s.command(ctx, c.ID, meta, parent, append(append([]string{}, base...), "clone", "--no-checkout", "--no-local", "--template="+template, "--", remote, repoPath)); err != nil {
		return err
	}
	if _, err := s.command(ctx, c.ID, meta, repoPath, append(append([]string{}, base...), "checkout", "-b", c.Branch, c.BaseCommit, "--")); err != nil {
		return err
	}
	result, err := s.command(ctx, c.ID, meta, repoPath, append(append([]string{}, base...), "rev-parse", "HEAD"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Output) != c.BaseCommit {
		return errors.New("clone commit did not match the approved snapshot")
	}
	if err = s.Store.Transition(ctx, c.ID, "ANALYZING_REPOSITORY"); err != nil {
		return err
	}
	inspected := map[string]string{}
	for _, name := range []string{"AGENTS.md", "CONTRIBUTING.md", "README.md", "go.mod", "package.json", "Makefile", "pyproject.toml", "pom.xml"} {
		path := filepath.Join(repoPath, name)
		info, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return errors.New("repository inspection failed")
		}
		if !info.Mode().IsRegular() || info.Size() > 128<<10 {
			continue
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return errors.New("repository inspection failed")
		}
		inspected[name] = string(b)
	}
	if err = writeJSON(filepath.Join(meta, "inspection.json"), inspected); err != nil {
		return errors.New("cannot persist repository inspection")
	}
	if err = s.Store.WorkspaceEvent(ctx, c.ID, "RepositoryInspected", "Read repository instructions and manifests; no repository code or tests executed", map[string]any{"files": inspected, "base_commit": c.BaseCommit}); err != nil {
		return err
	}
	if err = s.Store.Transition(ctx, c.ID, "PLANNING"); err != nil {
		return err
	}
	if s.Prepared != nil && c.ExecutionApproved {
		if err = s.Store.WorkspaceMessage(ctx, c.ID, "Workspace ready; starting approved Codex workflow"); err != nil {
			return err
		}
		return s.Prepared(ctx, c)
	}
	if err = s.Store.WorkspaceMessage(ctx, c.ID, "Repository cloned and branch verified. Awaiting explicit approval to start Codex coding, real verification and independent review."); err != nil {
		return err
	}
	return s.Store.Transition(ctx, c.ID, "BLOCKED")
}
func createIsolated(root, parent string) error {
	return createAt(filepath.Join(root, "contributions"), parent)
}
func createAt(base, parent string) error {
	base = filepath.Clean(base)
	var err error
	if err = os.MkdirAll(base, 0700); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(resolved), base) {
		return errors.New("contributions must not be a link")
	}
	if filepath.Dir(parent) != base {
		return errors.New("workspace outside contributions")
	}
	return os.Mkdir(parent, 0700) // Exclusive, never reuse an existing directory or junction.
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func (s *Service) exportEvents(ctx context.Context, id, meta string) error {
	var all strings.Builder
	cursor := int64(0)
	for {
		events, err := s.Store.Events(ctx, cursor, id, 100)
		if err != nil {
			return err
		}
		for _, e := range events {
			b, err := json.Marshal(e)
			if err != nil {
				return err
			}
			all.Write(b)
			all.WriteByte('\n')
			cursor = e.ID
		}
		if len(events) < 100 {
			break
		}
	}
	return os.WriteFile(filepath.Join(meta, "events.jsonl"), []byte(all.String()), 0600)
}

type limitedOutput struct {
	mu        sync.Mutex
	b         strings.Builder
	truncated bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := (128 << 10) - b.b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	b.b.Write(p)
	return n, nil
}
func gitEnvironment(meta string) []string {
	result := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || upper == "GH_TOKEN" || upper == "GITHUB_TOKEN" {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1", "GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_SYSTEM="+os.DevNull)
}
func (s *Service) command(ctx context.Context, id, meta, dir string, args []string) (domain.CommandRecord, error) {
	rec := domain.CommandRecord{Arguments: append([]string{"git"}, args...), StartedAt: time.Now().UTC(), ExitCode: -1}
	if err := s.Store.WorkspaceEvent(ctx, id, "CommandStarted", "Running Git in isolated workspace", rec); err != nil {
		return rec, err
	}
	cmd := exec.CommandContext(ctx, s.Git, args...)
	cmd.Dir = dir
	cmd.Env = gitEnvironment(meta)
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	output := &limitedOutput{}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	rec.FinishedAt = time.Now().UTC()
	if cmd.ProcessState != nil {
		rec.ExitCode = cmd.ProcessState.ExitCode()
	}
	rec.Output = output.b.String()
	rec.Truncated = output.truncated
	auditCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if e := s.Store.WorkspaceEvent(auditCtx, id, "CommandFinished", "Git command finished", rec); e != nil {
		return rec, errors.New("cannot persist command outcome")
	}
	file, e := os.OpenFile(filepath.Join(meta, "commands.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return rec, errors.New("cannot persist command log")
	}
	e = json.NewEncoder(file).Encode(rec)
	closeErr := file.Close()
	if e != nil || closeErr != nil {
		return rec, errors.New("cannot persist command log")
	}
	if err != nil {
		return rec, fmt.Errorf("Git command failed (exit %d); inspect the contribution command log", rec.ExitCode)
	}
	return rec, nil
}

var _ io.Writer = (*limitedOutput)(nil)
