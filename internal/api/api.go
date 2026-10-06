package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/codex"
	"forgeflow/internal/discovery"
	"forgeflow/internal/domain"
	"forgeflow/internal/execution"
	"forgeflow/internal/github"
	"forgeflow/internal/githubauth"
	"forgeflow/internal/operator"
	"forgeflow/internal/storage"
	"forgeflow/internal/workspace"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Auth       *githubauth.Service
	Store      *storage.Store
	WebDir     string
	Discovery  *discovery.Service
	Workspaces *workspace.Service
	Execution  *execution.Service
	Runtime    map[string]any
	OperatorAI codex.Runner
}

func (s Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/auth/github", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.Auth == nil {
			write(w, 200, map[string]any{"available": false, "message": "GitHub sign-in is unavailable in demo mode"})
			return
		}
		write(w, 200, s.Auth.Status())
	})
	m.HandleFunc("POST /api/auth/github/{action}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var b struct {
			ClientID string `json:"client_id"`
		}
		if !decode(w, r, &b) {
			return
		}
		if s.Auth == nil {
			write(w, 503, map[string]string{"error": "GitHub sign-in is unavailable in demo mode"})
			return
		}
		var err error
		switch r.PathValue("action") {
		case "configure":
			err = s.Auth.Configure(b.ClientID)
		case "login":
			err = s.Auth.Start(r.Context())
		case "logout", "cancel":
			err = s.Auth.Logout()
		default:
			write(w, 404, map[string]string{"error": "Unknown account action"})
			return
		}
		if err != nil {
			write(w, 400, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, s.Auth.Status())
	})
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"status": "ok", "mode": s.mode()})
	})
	m.HandleFunc("GET /api/overview", s.overview)
	m.HandleFunc("GET /api/runtime", func(w http.ResponseWriter, r *http.Request) { write(w, 200, s.Runtime) })
	m.HandleFunc("GET /api/discovery", func(w http.ResponseWriter, r *http.Request) {
		if s.Discovery == nil {
			write(w, 200, discovery.Status{Message: "Discovery adapter unavailable"})
			return
		}
		v, e := s.Discovery.Status(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/discovery/runs", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.DiscoveryRuns(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/discovery/{action}", func(w http.ResponseWriter, r *http.Request) {
		var b struct{}
		if !decode(w, r, &b) {
			return
		}
		if s.Discovery == nil {
			write(w, 503, map[string]string{"error": "Discovery adapter unavailable"})
			return
		}
		switch r.PathValue("action") {
		case "run":
			v, e := s.Discovery.Start()
			if e != nil {
				if errors.Is(e, discovery.ErrBusy) {
					write(w, 409, map[string]string{"error": e.Error()})
				} else {
					fail(w, e)
				}
				return
			}
			write(w, 202, v)
		case "pause", "resume":
			if e := s.Discovery.SetAutomatic(r.PathValue("action") == "resume"); e != nil {
				fail(w, e)
				return
			}
			v, e := s.Discovery.Status(r.Context())
			respond(w, v, e)
		case "cancel":
			if e := s.Discovery.RequestCancellation(r.Context()); e != nil {
				fail(w, e)
				return
			}
			write(w, 202, map[string]string{"status": "cancellation requested"})
		default:
			write(w, 404, map[string]string{"error": "Unknown discovery action"})
		}
	})
	m.HandleFunc("GET /api/opportunities", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Opportunities(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/opportunities/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Opportunity(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/opportunities/{id}/preview", func(w http.ResponseWriter, r *http.Request) {
		var b struct{}
		if !decode(w, r, &b) {
			return
		}
		if s.Workspaces == nil {
			write(w, 503, map[string]string{"error": "Workspace manager unavailable"})
			return
		}
		v, e := s.Workspaces.Preview(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/opportunities/{id}/proceed", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Approved bool   `json:"approved"`
			Token    string `json:"token"`
			Execute  bool   `json:"execute"`
		}
		if !decode(w, r, &b) {
			return
		}
		if s.Workspaces == nil {
			write(w, 503, map[string]string{"error": "Workspace manager unavailable"})
			return
		}
		v, e := s.Workspaces.ProceedWithExecution(r.Context(), r.PathValue("id"), b.Token, b.Approved, b.Execute)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 202, v)
	})
	m.HandleFunc("GET /api/contributions/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Contribution(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/contributions", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Contributions(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/agents", func(w http.ResponseWriter, r *http.Request) { v, e := s.Store.Agents(r.Context()); respond(w, v, e) })
	m.HandleFunc("GET /api/usage", func(w http.ResponseWriter, r *http.Request) { v, e := s.Store.Usage(r.Context()); respond(w, v, e) })
	m.HandleFunc("GET /api/events", s.events)
	m.HandleFunc("GET /api/contributions/{id}/execution", func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.Store.Contribution(r.Context(), r.PathValue("id")); e != nil {
			fail(w, e)
			return
		}
		v, e := s.Store.Execution(r.Context(), r.PathValue("id"))
		if errors.Is(e, storage.ErrNotFound) {
			write(w, 200, map[string]string{"status": "NOT_STARTED"})
			return
		}
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/contributions/{id}/tests", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Tests(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/contributions/{id}/{action}", s.executionAction)
	m.HandleFunc("GET /api/events/stream", s.stream)
	m.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.CurrentConfig(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/config/history", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.ConfigHistory(r.Context())
		respond(w, v, e)
	})
	m.HandleFunc("GET /api/config/proposals", func(w http.ResponseWriter, r *http.Request) { v, e := s.Store.Proposals(r.Context()); respond(w, v, e) })
	m.HandleFunc("POST /api/config/proposals", s.propose)
	m.HandleFunc("POST /api/config/proposals/{id}/apply", func(w http.ResponseWriter, r *http.Request) {
		var body struct{}
		if !decode(w, r, &body) {
			return
		}
		v, e := s.Store.Apply(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/config/proposals/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		var body struct{}
		if !decode(w, r, &body) {
			return
		}
		if e := s.Store.CancelProposal(r.Context(), r.PathValue("id")); e != nil {
			fail(w, e)
			return
		}
		write(w, 200, map[string]string{"status": "CANCELLED"})
	})
	m.HandleFunc("POST /api/config/rollback", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Version int64 `json:"version"`
			Base    int64 `json:"base_version"`
		}
		if !decode(w, r, &b) {
			return
		}
		v, e := s.Store.Rollback(r.Context(), b.Version, b.Base)
		respond(w, v, e)
	})
	m.HandleFunc("POST /api/operator/chat", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Message string `json:"message"`
		}
		if !decode(w, r, &b) {
			return
		}
		root := ""
		if s.Execution != nil {
			root = s.Execution.Root
		}
		v, e := (operator.Service{Store: s.Store, AI: s.OperatorAI, Execution: s.Execution, Root: root}).Chat(r.Context(), b.Message)
		respond(w, v, e)
	})
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		write(w, 404, map[string]string{"error": "API endpoint not found"})
	})
	m.HandleFunc("/", s.spa)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			write(w, 403, map[string]string{"error": "local host required"})
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Scheme != "http" || u.Host != r.Host {
					write(w, 403, map[string]string{"error": "same-origin request required"})
					return
				}
			}
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				write(w, 415, map[string]string{"error": "application/json required"})
				return
			}
		}
		m.ServeHTTP(w, r)
	})
}
func (s Server) mode() string {
	if s.Store.Demo {
		return "demo"
	}
	return "live"
}
func (s Server) overview(w http.ResponseWriter, r *http.Request) {
	os, err := s.Store.Opportunities(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	cs, err := s.Store.Contributions(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	c, err := s.Store.CurrentConfig(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	states := map[string]int{}
	repos := map[string]bool{}
	high := 0
	active := 0
	for _, o := range os {
		repos[o.Repository] = true
		if o.Ranking.Score >= 85 {
			high++
		}
	}
	for _, c := range cs {
		states[c.State]++
		if c.State != "READY" && c.State != "PR_PREPARED" && c.State != "PR_OPENED" && c.State != "ABANDONED" && c.State != "FAILED" {
			active++
		}
	}
	discoveryStatus := "unavailable"
	if s.Discovery != nil {
		v, e := s.Discovery.Status(r.Context())
		if e != nil {
			fail(w, e)
			return
		}
		discoveryStatus = "idle"
		if !v.Available {
			discoveryStatus = "demo disabled"
		} else if v.Running {
			discoveryStatus = "scanning"
		} else if v.RetryAt != nil {
			discoveryStatus = "rate limited"
		} else if v.LastRun != nil {
			discoveryStatus = strings.ToLower(v.LastRun.Status)
		}
	}
	write(w, 200, map[string]any{"mode": s.mode(), "repositories": len(repos), "opportunities": len(os), "high_quality": high, "active_contributions": active, "states": states, "config_version": c.Version, "discovery_status": discoveryStatus, "execution_available": !s.Store.Demo && s.Execution != nil})
}
func (s Server) events(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("after") == "" && r.URL.Query().Get("entity") == "" {
		v, e := s.Store.RecentEvents(r.Context())
		respond(w, v, e)
		return
	}
	after, e := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		after = 0
		e = nil
	}
	if e != nil || after < 0 {
		write(w, 400, map[string]string{"error": "invalid event cursor"})
		return
	}
	v, e := s.Store.Events(r.Context(), after, r.URL.Query().Get("entity"), 100)
	respond(w, v, e)
}
func (s Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		write(w, 500, map[string]string{"error": "streaming unavailable"})
		return
	}
	cursorText := r.Header.Get("Last-Event-ID")
	if cursorText == "" {
		cursorText = r.URL.Query().Get("after")
	}
	cursor := int64(0)
	if cursorText != "" {
		var err error
		cursor, err = strconv.ParseInt(cursorText, 10, 64)
		if err != nil || cursor < 0 {
			write(w, 400, map[string]string{"error": "invalid event cursor"})
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if _, err := io.WriteString(w, "retry: 2000\n\n"); err != nil {
		return
	}
	flusher.Flush()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		es, err := s.Store.Events(r.Context(), cursor, "", 100)
		if err != nil {
			return
		}
		for _, e := range es {
			b, err := json.Marshal(e)
			if err != nil {
				return
			}
			if _, err = fmt.Fprintf(w, "id: %d\nevent: activity\ndata: %s\n\n", e.ID, b); err != nil {
				return
			}
			cursor = e.ID
		}
		if len(es) > 0 {
			flusher.Flush()
			if len(es) == 100 {
				continue
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
func (s Server) propose(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Base   int64         `json:"base_version"`
		Config domain.Config `json:"config"`
		Reason string        `json:"reason"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, e := s.Store.Propose(r.Context(), b.Base, b.Config, b.Reason)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		write(w, 400, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		write(w, 400, map[string]string{"error": "only one JSON value is allowed"})
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("response write failed", "error", err)
	}
}
func respond(w http.ResponseWriter, v any, e error) {
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, v)
}
func fail(w http.ResponseWriter, e error) {
	status := 400
	message := e.Error()
	var upstream *github.APIError
	if errors.As(e, &upstream) && !upstream.RetryAt.IsZero() {
		status = 429
		seconds := int(time.Until(upstream.RetryAt).Seconds()) + 1
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	if errors.Is(e, storage.ErrNotFound) {
		status = 404
	}
	if errors.Is(e, storage.ErrConflict) {
		status = 409
	}
	if errors.Is(e, workspace.ErrChanged) {
		status = 409
	}
	// Do not expose SQL, filesystem paths or credentials in infrastructure errors.
	if strings.Contains(message, "SQL") || strings.Contains(message, "database") || strings.Contains(message, "context canceled") {
		status = 500
		message = "storage operation failed"
		slog.Error("storage operation failed", "error", e)
	}
	write(w, status, map[string]string{"error": message})
}
func (s Server) spa(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	if s.WebDir == "" {
		write(w, 404, map[string]string{"error": "Frontend not built; run npm run dev in apps/web or build it first."})
		return
	}
	// Serve only files inside the built asset tree; unknown routes return SPA entry.
	clean := filepath.Clean(filepath.FromSlash(r.URL.Path))
	rel := strings.TrimLeft(clean, "/\\")
	if strings.HasPrefix(rel, "..") {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.WebDir, rel)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	if rel != "" && rel != "." && strings.Contains(filepath.Base(rel), ".") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.WebDir, "index.html"))
}
