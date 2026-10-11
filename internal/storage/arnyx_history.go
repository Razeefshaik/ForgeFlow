package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type ArnyxConversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type ArnyxMessage struct {
	Sequence  int64  `json:"sequence"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}
type ArnyxTask struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	Directory      string           `json:"directory"`
	Mode           string           `json:"mode"`
	Prompt         string           `json:"prompt"`
	Network        bool             `json:"network"`
	Status         string           `json:"status"`
	Activity       string           `json:"activity"`
	Output         string           `json:"output,omitempty"`
	Error          string           `json:"error,omitempty"`
	SessionID      string           `json:"session_id,omitempty"`
	Usage          map[string]int64 `json:"usage,omitempty"`
	CreatedAt      string           `json:"created_at"`
	UpdatedAt      string           `json:"updated_at"`
	ViewRoute      string           `json:"view_route,omitempty"`
	ContributionID string           `json:"contribution_id,omitempty"`
}

func (s *Store) NewArnyxConversation(ctx context.Context) (ArnyxConversation, error) {
	c := ArnyxConversation{ID: ID(), Title: "New conversation", CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx, "INSERT INTO arnyx_conversations VALUES (?,?,?,?)", c.ID, c.Title, c.CreatedAt, c.UpdatedAt)
	return c, err
}
func (s *Store) ArnyxConversations(ctx context.Context, offset int) ([]ArnyxConversation, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,title,created_at,updated_at FROM arnyx_conversations ORDER BY updated_at DESC,id LIMIT 50 OFFSET ?", offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ArnyxConversation{}
	for rows.Next() {
		var c ArnyxConversation
		if err = rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
func (s *Store) ArnyxMessages(ctx context.Context, id string, before int64) ([]ArnyxMessage, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM arnyx_conversations WHERE id=?", id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT sequence,role,text,created_at FROM (SELECT sequence,role,text,created_at FROM arnyx_messages WHERE conversation_id=? AND (?=0 OR sequence<?) ORDER BY sequence DESC LIMIT 100) ORDER BY sequence", id, before, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ArnyxMessage{}
	for rows.Next() {
		var m ArnyxMessage
		if err = rows.Scan(&m.Sequence, &m.Role, &m.Text, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func arnyxAppend(ctx context.Context, tx *sql.Tx, id, role, text string) error {
	if role != "user" && role != "assistant" {
		return errors.New("invalid conversation role")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO arnyx_messages(conversation_id,role,text,created_at) VALUES (?,?,?,?)", id, role, text, now()); err != nil {
		return err
	}
	title := strings.Join(strings.Fields(text), " ")
	runes := []rune(title)
	if len(runes) > 70 {
		title = string(runes[:70]) + "…"
	}
	_, err := tx.ExecContext(ctx, "UPDATE arnyx_conversations SET updated_at=?,title=CASE WHEN title='New conversation' AND ?='user' THEN ? ELSE title END WHERE id=?", now(), role, title, id)
	return err
}
func (s *Store) AppendArnyxMessage(ctx context.Context, id, role, text string) error {
	return s.transact(ctx, func(tx *sql.Tx) error { return arnyxAppend(ctx, tx, id, role, text) })
}
func (s *Store) CreateArnyxTask(ctx context.Context, t ArnyxTask) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if err := arnyxAppend(ctx, tx, t.ConversationID, "user", t.Prompt); err != nil {
			return err
		}
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO arnyx_tasks VALUES (?,?,?)", t.ID, t.ConversationID, string(b)); err != nil {
			return err
		}
		return s.event(ctx, tx, "ArnyxTaskPrepared", t.ConversationID, "human", "Scoped Codex request prepared", map[string]any{"task_id": t.ID, "directory": t.Directory, "mode": t.Mode, "network": t.Network})
	})
}
func (s *Store) ArnyxTask(ctx context.Context, id string) (ArnyxTask, error) {
	var t ArnyxTask
	var data string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM arnyx_tasks WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	err = json.Unmarshal([]byte(data), &t)
	return t, err
}
func (s *Store) ArnyxTasks(ctx context.Context, id string) ([]ArnyxTask, error) {
	return listJSON[ArnyxTask](ctx, s.db, "SELECT data FROM arnyx_tasks WHERE conversation_id=? ORDER BY rowid DESC LIMIT 20", id)
}
func (s *Store) SaveArnyxTask(ctx context.Context, t ArnyxTask, finish bool) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, "UPDATE arnyx_tasks SET data=? WHERE id=?", string(b), t.ID)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return ErrNotFound
		}
		if finish {
			text := t.Output
			if t.Error != "" {
				text = "Codex task " + strings.ToLower(t.Status) + ": " + t.Error
			}
			if err = arnyxAppend(ctx, tx, t.ConversationID, "assistant", text); err != nil {
				return err
			}
		}
		return s.event(ctx, tx, "ArnyxTaskUpdated", t.ConversationID, "arnyx", t.Activity, map[string]any{"task_id": t.ID, "status": t.Status, "directory": t.Directory, "session_id": t.SessionID})
	})
}
func (s *Store) RecoverArnyxTasks(ctx context.Context) error {
	tasks, err := listJSON[ArnyxTask](ctx, s.db, "SELECT data FROM arnyx_tasks WHERE json_extract(data,'$.status') IN ('RUNNING','PREPARED')")
	if err != nil {
		return err
	}
	for _, t := range tasks {
		t.Status = "INTERRUPTED"
		t.Error = "ForgeFlow stopped before this task completed. Inspect the folder before requesting further edits."
		t.Activity = "Task interrupted by server shutdown"
		t.UpdatedAt = now()
		if err = s.SaveArnyxTask(ctx, t, true); err != nil {
			return err
		}
	}
	return nil
}
