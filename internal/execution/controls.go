package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func (s *Service) SetConstraints(ctx context.Context, id, text string, approved bool) error {
	if !approved || len(text) > 4000 {
		return errors.New("confirm contribution constraints of at most 4000 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks[id] != nil || s.prBusy[id] {
		return errors.New("pause execution before changing constraints")
	}
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "PLANNING" && c.State != "PAUSED" && c.State != "BLOCKED" {
		return errors.New("constraints can change only before execution or while paused/blocked")
	}
	r, err := s.Store.Execution(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		r = domain.ExecutionRecord{ContributionID: id, Status: "NOT_STARTED", Phase: "PLANNING", ChangedFiles: []string{}}
	} else if err != nil {
		return err
	}
	r.Constraints = strings.TrimSpace(text)
	return s.Store.SaveExecution(ctx, r, "user", "Human updated contribution constraints")
}

func (s *Service) beginPR(id string) error { return s.beginPROperation(id, false) }
func (s *Service) beginPROperation(id string, internal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return errors.New("server is stopping")
	}
	if !internal && s.tasks[id] != nil {
		return errors.New("wait for execution to finish before a PR operation")
	}
	if s.prBusy[id] {
		return errors.New("PR operation already in progress")
	}
	s.prBusy[id] = true
	return nil
}
func (s *Service) endPR(id string) { s.mu.Lock(); delete(s.prBusy, id); s.mu.Unlock() }
func (s *Service) reviewSlot(ctx context.Context, max int) (func(), error) {
	if max < 1 {
		return nil, errors.New("invalid reviewer concurrency")
	}
	for {
		s.mu.Lock()
		if s.reviewers < max {
			s.reviewers++
			s.mu.Unlock()
			return func() { s.mu.Lock(); s.reviewers--; s.mu.Unlock() }, nil
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// OpenWorkspace accepts only a persisted, boundary-validated workspace, never a command.
func (s *Service) OpenWorkspace(ctx context.Context, id string) error {
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return err
	}
	repo, _, err := s.Paths(c)
	if err != nil {
		return err
	}
	if err = s.Store.WorkspaceEvent(ctx, id, "WorkspaceOpened", "Human opened the contribution folder", nil); err != nil {
		return err
	}
	program := "xdg-open"
	if runtime.GOOS == "windows" {
		program = "explorer.exe"
	}
	if runtime.GOOS == "darwin" {
		program = "open"
	}
	cmd := exec.Command(program, repo)
	if err = cmd.Start(); err != nil {
		return errors.New("could not open the workspace in the file manager")
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
