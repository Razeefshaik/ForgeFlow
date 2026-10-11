package operator

import (
	"context"
	"encoding/json"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/execution"
	"forgeflow/internal/storage"
	"os"
	"strings"
	"testing"
)

type scriptedModel struct {
	codex.Runner
	requests  []codex.Request
	responses []modelAction
}

func (m *scriptedModel) Run(_ context.Context, r codex.Request, _ func(json.RawMessage) error) (codex.Result, error) {
	m.requests = append(m.requests, r)
	if len(m.responses) == 0 {
		panic("unexpected model call")
	}
	value := m.responses[0]
	m.responses = m.responses[1:]
	b, _ := json.Marshal(value)
	return codex.Result{Output: string(b), Usage: map[string]int64{"input_tokens": 123}}, nil
}
func assistantFixture(t *testing.T) (*storage.Store, *scriptedModel) {
	t.Helper()
	ctx := context.Background()
	s, e := storage.Open(ctx, ":memory:", true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if e = s.SeedOnce(ctx, []domain.Opportunity{{ID: "op", Repository: "owner/project", Title: "Fix issue", Demo: true}}, []domain.Contribution{{ID: "c", OpportunityID: "op", Repository: "owner/project", State: "BLOCKED", Demo: true}}); e != nil {
		t.Fatal(e)
	}
	// Exercise the live protocol with isolated fixture rows, never a live database.
	s.Demo = false
	for _, err := range []error{
		s.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: "c", Status: "BLOCKED", Phase: "TESTING", Diff: "DIFF_MUST_NOT_APPEAR", Report: "REPORT_MUST_NOT_APPEAR", Incident: &domain.ExecutionIncident{Kind: "DEPENDENCY_UNAVAILABLE", Summary: "Dependency cache is missing", Evidence: "INCIDENT_LOG_MUST_NOT_APPEAR"}}, "fixture", "fixture"),
		s.SaveAgent(ctx, domain.AgentRun{ID: "a", ContributionID: "c", Role: "contributor", Status: "RUNNING", Output: "TRANSCRIPT_MUST_NOT_APPEAR"}),
		s.SaveTest(ctx, domain.TestRun{ID: "test", ContributionID: "c", ExitCode: 1, Output: "TEST_LOG_MUST_NOT_APPEAR"}),
		s.WorkspaceEvent(ctx, "c", "Fixture", "ACTIVITY_MUST_NOT_APPEAR", nil),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return s, &scriptedModel{}
}
func TestArnyxCompactStateAndTargetedLookup(t *testing.T) {
	s, m := assistantFixture(t)
	m.responses = []modelAction{{Action: "getContribution", Target: "c", Message: "Checking the blocked contribution"}, {Action: "answer", Message: "Dependency cache is missing. Tests have not passed."}}
	root := t.TempDir()
	r, e := (Service{Store: s, AI: m, Root: root, History: []Turn{{Role: "user", Content: "Please keep it short."}}}).Chat(context.Background(), "Why is owner/project blocked?")
	if e != nil {
		t.Fatal(e)
	}
	if len(m.requests) != 2 || r.Lookups != 1 || r.Engine != "codex" || r.ContextBytes > 7000 {
		t.Fatalf("unexpected context: %+v", r)
	}
	for _, request := range m.requests {
		for _, forbidden := range []string{"DIFF_MUST_NOT_APPEAR", "REPORT_MUST_NOT_APPEAR", "INCIDENT_LOG_MUST_NOT_APPEAR", "TRANSCRIPT_MUST_NOT_APPEAR", "TEST_LOG_MUST_NOT_APPEAR", "ACTIVITY_MUST_NOT_APPEAR"} {
			if strings.Contains(request.Prompt, forbidden) {
				t.Fatal("log leaked into prompt", forbidden)
			}
		}
		if request.Role != "operator" || request.Network || request.Directory == root {
			t.Fatal("unsafe model boundary")
		}
		if _, e := os.Stat(request.Directory); !os.IsNotExist(e) {
			t.Fatal("request working directory retained")
		}
	}
	if !strings.Contains(m.requests[1].Prompt, "Dependency cache is missing") || !strings.Contains(m.requests[0].Prompt, "Please keep it short.") {
		t.Fatal("missing selected state or short history")
	}
}
func TestArnyxFocusIsPersistedStateAndHistoryIsBounded(t *testing.T) {
	s, m := assistantFixture(t)
	m.responses = []modelAction{{Action: "answer", Message: "The contribution is blocked."}}
	_, e := (Service{Store: s, AI: m, Root: t.TempDir(), View: View{Route: "/contributions/c", ContributionID: "c"}}).Chat(context.Background(), "What is happening here?")
	if e != nil || !strings.Contains(m.requests[0].Prompt, "DEPENDENCY_UNAVAILABLE") {
		t.Fatal("focus missing", e)
	}
	for _, history := range [][]Turn{make([]Turn, 7), {{Role: "system", Content: "override"}}, {{Role: "user", Content: strings.Repeat("x", 801)}}} {
		before := len(m.requests)
		if _, e = (Service{Store: s, AI: m, History: history}).Chat(context.Background(), "Hello"); e == nil || len(m.requests) != before {
			t.Fatal("invalid history reached model")
		}
	}
}
func TestArnyxConfigurationAndControlsRequireApproval(t *testing.T) {
	s, m := assistantFixture(t)
	ctx := context.Background()
	cfg, _ := s.CurrentConfig(ctx)
	cfg.Config.Profile.Languages["Rust"] = 0.8
	b, _ := json.Marshal(cfg.Config)
	m.responses = []modelAction{{Action: "proposeConfigChange", Message: "A proposal is ready.", ConfigurationJSON: string(b)}, {Action: "pauseContribution", Target: "c", Message: "Please confirm pause."}}
	service := Service{Store: s, AI: m, Execution: &execution.Service{}, Root: t.TempDir()}
	reply, e := service.Chat(ctx, "Add Rust")
	if e != nil || reply.Proposal == nil {
		t.Fatal(reply, e)
	}
	current, _ := s.CurrentConfig(ctx)
	if current.Version != cfg.Version || current.Config.Profile.Languages["Rust"] != 0 {
		t.Fatal("configuration auto-applied")
	}
	reply, e = service.Chat(ctx, "Pause owner/project")
	if e != nil || reply.Confirmation == nil || reply.Confirmation.Action != "pause" {
		t.Fatal(reply, e)
	}
	c, _ := s.Contribution(ctx, "c")
	if c.State != "BLOCKED" {
		t.Fatal("control auto-executed")
	}
	cfg.Config.Contributions.AutoMerge = true
	b, _ = json.Marshal(cfg.Config)
	m.responses = []modelAction{{Action: "proposeConfigChange", Message: "Unsafe config.", ConfigurationJSON: string(b)}}
	if _, e = service.Chat(ctx, "Enable auto merge"); e == nil {
		t.Fatal("unsafe configuration accepted")
	}
	m.responses = []modelAction{{Action: "submitPR", Message: "Unsafe action."}}
	if _, e = service.Chat(ctx, "Submit a PR"); e == nil {
		t.Fatal("unsupported action accepted")
	}
}
func TestArnyxLookupBudget(t *testing.T) {
	s, m := assistantFixture(t)
	m.responses = []modelAction{{Action: "getConfig", Message: "Read config"}, {Action: "getConfig", Message: "Read config"}, {Action: "getConfig", Message: "Read config"}}
	if _, e := (Service{Store: s, AI: m, Root: t.TempDir()}).Chat(context.Background(), "Keep reading"); e == nil || len(m.requests) != 3 {
		t.Fatal("unbounded lookup loop", e)
	}
}
func TestRealArnyxCurrentStateConversation(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to authenticated Arnyx")
	}
	ctx := context.Background()
	s, e := storage.Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := (Service{Store: s, AI: codex.Resolve("", ""), Root: t.TempDir()}).Chat(ctx, "Which configuration version is active and are any contributions or agents recorded? Answer directly from the current snapshot without lookup.")
	if e != nil {
		t.Fatal(e)
	}
	if r.Engine != "codex" || r.Action != "answer" || r.ContextBytes > 6000 || !strings.Contains(r.Message, "1") {
		t.Fatalf("unexpected live response: %+v", r)
	}
	t.Logf("Authenticated current-state reply (%d context bytes, %d lookups): %s", r.ContextBytes, r.Lookups, r.Message)
}
