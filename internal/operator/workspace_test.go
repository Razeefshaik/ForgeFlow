package operator

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type workspaceModel struct {
	codex.Runner
	requests chan codex.Request
	block    bool
	failure  error
}

func (m *workspaceModel) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	m.requests <- r
	if m.block {
		<-ctx.Done()
		return codex.Result{}, ctx.Err()
	}
	if err := event(json.RawMessage(`{"type":"item.started","item":{"type":"command_execution","command":"rg -n example README.md"}}`)); err != nil {
		return codex.Result{}, err
	}
	return codex.Result{SessionID: "fixture-session", Output: "Observed example in README.md", Usage: map[string]int64{"input_tokens": 14}}, m.failure
}
func workspaceFixture(t *testing.T) (context.Context, *storage.Store, *workspaceModel, *WorkspaceAssistant, string, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s, e := storage.Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	contributions := t.TempDir()
	model := &workspaceModel{requests: make(chan codex.Request, 10)}
	a := NewWorkspaceAssistant(ctx, s, model, root, contributions)
	t.Cleanup(func() { cancel(); a.Wait(); s.Close() })
	c, e := s.NewArnyxConversation(ctx)
	if e != nil {
		t.Fatal(e)
	}
	return ctx, s, model, a, root, c.ID
}
func TestWorkspaceEditRequiresBoundApprovalAndCannotBeReplayed(t *testing.T) {
	ctx, s, m, a, root, conversation := workspaceFixture(t)
	task, e := a.Prepare(ctx, conversation, "Fix the selected project", root, "edit", false, View{})
	if e != nil {
		t.Fatal(e)
	}
	if task.Status != "AWAITING_APPROVAL" || len(m.requests) != 0 {
		t.Fatal("editing ran without approval")
	}
	if _, e = a.Approve(ctx, task.ID, false); e == nil {
		t.Fatal("missing approval accepted")
	}
	if _, e = a.Approve(ctx, task.ID, true); e != nil {
		t.Fatal(e)
	}
	a.Wait()
	r := <-m.requests
	if r.Role != "assistant-editor" || r.Directory != root || r.Network {
		t.Fatal("incorrect approved scope", r)
	}
	if _, e = a.Approve(ctx, task.ID, true); e == nil {
		t.Fatal("approval replay allowed")
	}
	done, e := s.ArnyxTask(ctx, task.ID)
	if e != nil || done.Status != "SUCCEEDED" || done.SessionID != "fixture-session" {
		t.Fatal(done, e)
	}
	messages, e := s.ArnyxMessages(ctx, conversation, 0)
	if e != nil || len(messages) != 2 || messages[1].Text != "Observed example in README.md" {
		t.Fatal(messages, e)
	}
}
func TestWorkspaceInspectionIsReadOnlyAndUsesBoundedContext(t *testing.T) {
	ctx, s, m, a, root, conversation := workspaceFixture(t)
	for i := 0; i < 10; i++ {
		if e := s.AppendArnyxMessage(ctx, conversation, "assistant", strings.Repeat("Z", 90000)); e != nil {
			t.Fatal(e)
		}
	}
	task, e := a.Prepare(ctx, conversation, "Inspect the source and explain the blocker", root, "inspect", true, View{})
	if e != nil {
		t.Fatal(e)
	}
	a.Wait()
	r := <-m.requests
	if r.Role != "assistant-reader" || r.Directory != root || !r.Network || len(r.Prompt) > 35000 {
		t.Fatal("inspection scope/context incorrect", len(r.Prompt), r.Role)
	}
	done, _ := s.ArnyxTask(ctx, task.ID)
	if done.Status != "SUCCEEDED" {
		t.Fatal(done)
	}
}
func TestWorkspaceCancellationFailureAndContributionIsolation(t *testing.T) {
	ctx, s, m, a, root, conversation := workspaceFixture(t)
	child := filepath.Join(a.ContributionsRoot, "external", "repo")
	if e := os.MkdirAll(child, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Prepare(ctx, conversation, "Edit contribution", child, "edit", false, View{}); e == nil {
		t.Fatal("parallel contribution editor allowed")
	}
	if _, e := a.Prepare(ctx, conversation, "Read files", "missing-relative-path", "inspect", false, View{}); e == nil {
		t.Fatal("invalid folder accepted")
	}
	m.block = true
	task, e := a.Prepare(ctx, conversation, "Inspect", root, "inspect", false, View{})
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-m.requests:
	case <-time.After(5 * time.Second):
		t.Fatal("CLI not started")
	}
	if _, e = a.Prepare(ctx, conversation, "Edit while inspecting", root, "edit", false, View{}); e == nil {
		t.Fatal("concurrent workspace edits allowed")
	}
	if _, e = a.Stop(ctx, task.ID); e != nil {
		t.Fatal(e)
	}
	a.Wait()
	done, _ := s.ArnyxTask(ctx, task.ID)
	if done.Status != "CANCELLED" {
		t.Fatal(done)
	}
	m.block = false
	m.failure = errors.New("actual diagnostic failure")
	task, e = a.Prepare(ctx, conversation, "Inspect again", root, "inspect", false, View{})
	if e != nil {
		t.Fatal(e)
	}
	a.Wait()
	done, _ = s.ArnyxTask(ctx, task.ID)
	if done.Status != "FAILED" || done.Error != "actual diagnostic failure" {
		t.Fatal(done)
	}
}
func TestRealWorkspaceCodexInspectionAndApprovedEdit(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to authenticated local CLI inspection/edit in a temporary fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	s, e := storage.Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	root := t.TempDir()
	file := filepath.Join(root, "evidence.txt")
	if e = os.WriteFile(file, []byte("FORGEFLOW_LOCAL_EVIDENCE_729\n"), 0600); e != nil {
		t.Fatal(e)
	}
	a := NewWorkspaceAssistant(ctx, s, codex.Resolve("", ""), root, t.TempDir())
	defer func() { cancel(); a.Wait() }()
	conversation, e := s.NewArnyxConversation(ctx)
	if e != nil {
		t.Fatal(e)
	}
	task, e := a.Prepare(ctx, conversation.ID, "Read evidence.txt with a file or shell tool and report its exact contents. Do not modify any files.", root, "inspect", false, View{})
	if e != nil {
		t.Fatal(e)
	}
	a.Wait()
	done, e := s.ArnyxTask(ctx, task.ID)
	if e != nil || done.Status != "SUCCEEDED" || !strings.Contains(done.Output, "FORGEFLOW_LOCAL_EVIDENCE_729") {
		t.Fatal(done, e)
	}
	task, e = a.Prepare(ctx, conversation.ID, "Create approved.txt containing exactly APPROVED_WORKSPACE_EDIT_841 followed by a newline. Read it back and report the result. Leave evidence.txt unchanged.", root, "edit", false, View{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "approved.txt")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("file created before approval")
	}
	if _, e = a.Approve(ctx, task.ID, true); e != nil {
		t.Fatal(e)
	}
	a.Wait()
	done, e = s.ArnyxTask(ctx, task.ID)
	if e != nil || done.Status != "SUCCEEDED" {
		t.Fatal(done, e)
	}
	content, e := os.ReadFile(filepath.Join(root, "approved.txt"))
	if e != nil || strings.TrimSpace(string(content)) != "APPROVED_WORKSPACE_EDIT_841" {
		t.Fatal(string(content), e)
	}
	original, _ := os.ReadFile(file)
	if string(original) != "FORGEFLOW_LOCAL_EVIDENCE_729\n" {
		t.Fatal("unrequested file modified")
	}
	t.Log("Authenticated CLI inspected a real file and completed an approved file edit in the isolated fixture")
}
