package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
)

func (s *Service) Retry(ctx context.Context, id, action string, approved bool) (domain.ExecutionRecord, error) {
	if !approved {
		return domain.ExecutionRecord{}, errors.New("explicit confirmation is required")
	}
	s.mu.Lock()
	if _, ok := s.tasks[id]; ok {
		s.mu.Unlock()
		return domain.ExecutionRecord{}, errors.New("pause execution before retrying")
	}
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return domain.ExecutionRecord{}, err
	}
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return r, err
	}
	if c.State == "PR_PREPARED" || c.State == "PR_OPENED" || c.State == "ABANDONED" || c.State == "FAILED" {
		s.mu.Unlock()
		return r, errors.New("cannot retry this contribution state")
	}
	resumeState := c.State
	if c.State == "PAUSED" || c.State == "BLOCKED" {
		resumeState = c.PreviousState
	}
	if resumeState != "READY" && resumeState != "CODING" && resumeState != "FIXING" && resumeState != "TESTING" && resumeState != "REVIEWING" {
		s.mu.Unlock()
		return r, errors.New("implementation must finish before retrying verification")
	}
	if c.State == "PAUSED" || c.State == "BLOCKED" {
		if err = s.Store.Transition(ctx, id, c.PreviousState); err != nil {
			s.mu.Unlock()
			return r, err
		}
		c.State = c.PreviousState
	}
	if c.State == "READY" || c.State == "REVIEWING" {
		if err = s.Store.Transition(ctx, id, "FIXING"); err != nil {
			s.mu.Unlock()
			return r, err
		}
		c.State = "FIXING"
	}
	if c.State == "CODING" || c.State == "FIXING" {
		if err = s.Store.Transition(ctx, id, "TESTING"); err != nil {
			s.mu.Unlock()
			return r, err
		}
		c.State = "TESTING"
	}
	if c.State != "TESTING" {
		s.mu.Unlock()
		return r, errors.New("implementation must finish before retrying verification")
	}
	r.Phase = "TESTING"
	r.Status = "BLOCKED"
	r.Review = nil
	r.ReviewCycles = 0
	if err = s.Store.SaveExecution(ctx, r, "user", "Human requested fresh tests and independent review"); err != nil {
		s.mu.Unlock()
		return r, err
	}
	s.mu.Unlock()
	return s.Start(ctx, id, true, r.Network)
}
