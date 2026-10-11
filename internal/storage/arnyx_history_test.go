package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestArnyxHistorySurvivesRestartAndPaginates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	s, e := Open(ctx, path, false)
	if e != nil {
		t.Fatal(e)
	}
	c, e := s.NewArnyxConversation(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.AppendArnyxMessage(ctx, c.ID, "user", "Investigate build failure"); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 110; i++ {
		if e = s.AppendArnyxMessage(ctx, c.ID, "assistant", "Evidence remains available"); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(ctx, path, false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	chats, e := s.ArnyxConversations(ctx, 0)
	if e != nil || len(chats) != 1 || chats[0].Title != "Investigate build failure" {
		t.Fatal(chats, e)
	}
	recent, e := s.ArnyxMessages(ctx, c.ID, 0)
	if e != nil || len(recent) != 100 {
		t.Fatal(len(recent), e)
	}
	older, e := s.ArnyxMessages(ctx, c.ID, recent[0].Sequence)
	if e != nil || len(older) != 11 || older[0].Text != "Investigate build failure" {
		t.Fatal(older, e)
	}
	if _, e = s.NewArnyxConversation(ctx); e != nil {
		t.Fatal(e)
	}
	retained, _ := s.ArnyxMessages(ctx, c.ID, 0)
	if len(retained) != 100 {
		t.Fatal("new chat erased history")
	}
}
func TestArnyxRestartMarksRunningTaskInterruptedWithoutExecuting(t *testing.T) {
	s := checkpointFixture(t)
	ctx := context.Background()
	c, e := s.NewArnyxConversation(ctx)
	if e != nil {
		t.Fatal(e)
	}
	task := ArnyxTask{ID: ID(), ConversationID: c.ID, Prompt: "inspect", Status: "RUNNING"}
	if e = s.CreateArnyxTask(ctx, task); e != nil {
		t.Fatal(e)
	}
	if e = s.RecoverArnyxTasks(ctx); e != nil {
		t.Fatal(e)
	}
	done, e := s.ArnyxTask(ctx, task.ID)
	if e != nil || done.Status != "INTERRUPTED" || done.Error == "" {
		t.Fatal(done, e)
	}
	messages, _ := s.ArnyxMessages(ctx, c.ID, 0)
	if len(messages) != 2 {
		t.Fatal(messages)
	}
	if e = s.RecoverArnyxTasks(ctx); e != nil {
		t.Fatal(e)
	}
	messages, _ = s.ArnyxMessages(ctx, c.ID, 0)
	if len(messages) != 2 {
		t.Fatal("restart duplicated terminal message")
	}
}
