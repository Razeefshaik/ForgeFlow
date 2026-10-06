package api

import (
	"net/http"
)

func (s Server) executionAction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Approved    bool   `json:"approved"`
		Network     bool   `json:"network"`
		Token       string `json:"token"`
		Constraints string `json:"constraints"`
	}
	if !decode(w, r, &b) {
		return
	}
	if s.Execution == nil {
		write(w, 503, map[string]string{"error": "Execution adapter unavailable"})
		return
	}
	id := r.PathValue("id")
	action := r.PathValue("action")
	switch action {
	case "constraints":
		if e := s.Execution.SetConstraints(r.Context(), id, b.Constraints, b.Approved); e != nil {
			fail(w, e)
			return
		}
		write(w, 200, map[string]string{"status": "saved"})
	case "open-workspace":
		if e := s.Execution.OpenWorkspace(r.Context(), id); e != nil {
			fail(w, e)
			return
		}
		write(w, 200, map[string]string{"status": "opened"})
	case "start", "resume":
		v, e := s.Execution.Start(r.Context(), id, b.Approved, b.Network)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 202, v)
	case "tests", "review":
		v, e := s.Execution.Retry(r.Context(), id, action, b.Approved)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 202, v)
	case "pause", "stop", "abandon", "approve-plan":
		if e := s.Execution.Control(r.Context(), id, action, b.Approved); e != nil {
			fail(w, e)
			return
		}
		write(w, 202, map[string]string{"status": "accepted"})
	case "prepare-pr":
		v, e := s.Execution.PreparePR(r.Context(), id)
		respond(w, v, e)
	case "submit-pr":
		v, e := s.Execution.SubmitPR(r.Context(), id, b.Token, b.Approved)
		respond(w, v, e)
	default:
		write(w, 404, map[string]string{"error": "Unknown contribution action"})
	}
}
