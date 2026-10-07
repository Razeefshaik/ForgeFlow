package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type agentProgress struct {
	mu   sync.Mutex
	last time.Time
}

func (p *agentProgress) activity() { p.mu.Lock(); p.last = time.Now(); p.mu.Unlock() }
func (s *Service) heartbeat(ctx context.Context, id, runID, role string, p *agentProgress) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	started := time.Now()
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.mu.Lock()
				last := p.last
				p.mu.Unlock()
				quiet := time.Since(last).Round(time.Second)
				message := fmt.Sprintf("%s process running for %s; no new output for %s", role, time.Since(started).Round(time.Second), quiet)
				_ = s.Store.WorkspaceEvent(ctx, id, "AgentHeartbeat", message, map[string]any{"run_id": runID, "role": role, "elapsed_seconds": int(time.Since(started).Seconds()), "last_output_at": last.UTC()})
			}
		}
	}()
	return func() { cancel(); <-done }
}
func agentActivityMessage(role string, raw json.RawMessage) string {
	var e struct {
		Type string `json:"type"`
		Item struct {
			Type     string `json:"type"`
			Command  string `json:"command"`
			ExitCode *int   `json:"exit_code"`
		} `json:"item"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return role + " activity"
	}
	switch e.Type {
	case "thread.started":
		return role + " session connected"
	case "turn.started":
		return role + " turn started"
	case "turn.completed":
		return role + " turn completed"
	case "turn.failed", "error":
		return role + " reported an error"
	}
	if e.Item.Type == "command_execution" {
		command := strings.Join(strings.Fields(e.Item.Command), " ")
		if len(command) > 240 {
			command = command[:240] + "…"
		}
		if e.Type == "item.started" {
			return role + " started command: " + command
		}
		if e.Type == "item.completed" && e.Item.ExitCode != nil {
			return fmt.Sprintf("%s command finished (exit %d): %s", role, *e.Item.ExitCode, command)
		}
	}
	if e.Item.Type != "" {
		return role + ": " + strings.ReplaceAll(e.Item.Type, "_", " ")
	}
	return role + " activity"
}
