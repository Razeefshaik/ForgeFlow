package storage

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "forgeflow/internal/contributions"
 "forgeflow/internal/domain"
 "time"
)

// SaveExecutionCheckpoint commits the workflow state, execution evidence and
// audit events together. A crash cannot persist only half of a phase change.
func (s *Store) SaveExecutionCheckpoint(ctx context.Context, v domain.ExecutionRecord, state, actor, message string) error {
 return s.transact(ctx, func(tx *sql.Tx) error {
  var raw string
  if err := tx.QueryRowContext(ctx, "SELECT data FROM contributions WHERE id=?", v.ContributionID).Scan(&raw); err != nil {
   if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }; return err
  }
  var c domain.Contribution
  if err := json.Unmarshal([]byte(raw), &c); err != nil { return err }
  from := c.State
  if from != state {
   if err := contributions.Validate(from, state, c.PreviousState); err != nil { return err }
   c.PreviousState = ""
   if state == "PAUSED" || state == "BLOCKED" { c.PreviousState = from }
   c.State = state
  }
  v.UpdatedAt = time.Now().UTC()
  c.UpdatedAt = v.UpdatedAt
  if v.Message != "" { c.Message = v.Message }
  body, err := json.Marshal(c); if err != nil { return err }
  if _, err = tx.ExecContext(ctx, "UPDATE contributions SET data=? WHERE id=?", string(body), c.ID); err != nil { return err }
  body, err = json.Marshal(v); if err != nil { return err }
  if _, err = tx.ExecContext(ctx, "INSERT INTO executions VALUES (?,?) ON CONFLICT(contribution_id) DO UPDATE SET data=excluded.data", v.ContributionID, string(body)); err != nil { return err }
  if from != state {
   if err = s.event(ctx, tx, "ContributionStateChanged", c.ID, "system", from+" → "+state, map[string]string{"from":from,"to":state}); err != nil { return err }
  }
  return s.event(ctx, tx, "ExecutionUpdated", c.ID, actor, message, v)
 })
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
