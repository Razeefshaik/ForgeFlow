package api

import (
	"forgeflow/internal/operator"
	"net/http"
	"strconv"
)

func (s Server) arnyxRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/arnyx/conversations", func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		v, e := s.Store.ArnyxConversations(r.Context(), offset)
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/arnyx/conversations", func(w http.ResponseWriter, r *http.Request) {
		var b struct{}
		if !decode(w, r, &b) {
			return
		}
		v, e := s.Store.NewArnyxConversation(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/arnyx/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		// Read tasks first: a terminal task commits its final message atomically.
		// This order ensures polling cannot stop before receiving that message.
		tasks, e := s.Store.ArnyxTasks(r.Context(), r.PathValue("id"))
		if e != nil {
			respond(w, nil, e)
			return
		}
		messages, e := s.Store.ArnyxMessages(r.Context(), r.PathValue("id"), before)
		if e != nil {
			respond(w, nil, e)
			return
		}
		respond(w, map[string]any{"messages": messages, "tasks": tasks}, e)
	})
	m.HandleFunc("POST /api/arnyx/conversations/{id}/tasks", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Message   string        `json:"message"`
			Directory string        `json:"directory"`
			Mode      string        `json:"mode"`
			Network   bool          `json:"network"`
			View      operator.View `json:"view"`
		}
		if !decode(w, r, &b) {
			return
		}
		if s.Assistant == nil {
			write(w, 503, map[string]string{"error": "restart ForgeFlow to enable workspace tools"})
			return
		}
		v, e := s.Assistant.Prepare(r.Context(), r.PathValue("id"), b.Message, b.Directory, b.Mode, b.Network, b.View)
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/arnyx/tasks/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Approved bool `json:"approved"`
		}
		if !decode(w, r, &b) {
			return
		}
		if s.Assistant == nil {
			write(w, 503, map[string]string{"error": "workspace tools are unavailable"})
			return
		}
		switch r.PathValue("action") {
		case "approve":
			v, e := s.Assistant.Approve(r.Context(), r.PathValue("id"), b.Approved)
			respond(w, v, e)
		case "stop":
			v, e := s.Assistant.Stop(r.Context(), r.PathValue("id"))
			respond(w, v, e)
		default:
			write(w, 404, map[string]string{"error": "unknown workspace task action"})
		}
	})
}
