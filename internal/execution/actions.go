package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
)

func (s *Service) Retry(ctx context.Context, id, action string, approved bool) (domain.ExecutionRecord, error) {
	return s.retry(ctx, id, action, approved, nil)
}
func (s *Service) retry(ctx context.Context, id, action string, approved bool, network *bool) (domain.ExecutionRecord, error) {
	if !approved {
		return domain.ExecutionRecord{}, errors.New("explicit confirmation is required")
	}
	if action != "tests" && action != "review" {
		return domain.ExecutionRecord{}, errors.New("unknown verification action")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		return r, err
	}
	allowed := r.Network
	if network != nil {
		allowed = *network
	}
	return s.startLocked(ctx, id, allowed, true)
}
