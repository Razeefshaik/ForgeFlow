package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type healthCheck struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	Message   string  `json:"message"`
	FreeBytes *uint64 `json:"free_bytes,omitempty"`
}

// Checks are local and read-only; availability is never evidence of authentication
// or a passing contribution. No remote requests or execution are triggered.
func (s Server) health(w http.ResponseWriter, r *http.Request, detailed bool) {
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	checks := []healthCheck{}
	status := "ok"
	code := http.StatusOK
	add := func(c healthCheck) {
		checks = append(checks, c)
		if c.Status == "failed" {
			status = "failed"
			code = http.StatusServiceUnavailable
		} else if c.Status == "warning" && status == "ok" {
			status = "warning"
		}
	}
	if err := s.Store.Ping(ctx); err != nil {
		add(healthCheck{ID: "database", Name: "Database", Status: "failed", Message: "Database is unavailable; do not start new work."})
	} else {
		add(healthCheck{ID: "database", Name: "Database", Status: "ok", Message: "Database connection responded."})
	}
	if detailed {
		if _, err := exec.LookPath("git"); err != nil {
			add(healthCheck{ID: "git", Name: "Git", Status: "warning", Message: "Git is unavailable. Install Git before preparing contributions."})
		} else {
			add(healthCheck{ID: "git", Name: "Git", Status: "ok", Message: "Git executable is available."})
		}
		if available, ok := s.Runtime["codex_available"].(bool); !ok || !available {
			add(healthCheck{ID: "runner", Name: "Execution tools", Status: "warning", Message: "Execution tool availability has not been confirmed."})
		} else {
			add(healthCheck{ID: "runner", Name: "Execution tools", Status: "ok", Message: "Execution tool detected at startup. Sign-in and individual runs require their own checks."})
		}
		root, _ := s.Runtime["contributions_root"].(string)
		for _, disk := range []struct{ id, name, path string }{{"workspace_disk", "Workspace storage", root}, {"temp_disk", "Temporary storage", os.TempDir()}} {
			path := disk.path
			if path == "" {
				continue
			}
			// The configured workspace root may not exist before the first contribution.
			for {
				if _, err := os.Stat(path); err == nil {
					break
				}
				parent := filepath.Dir(path)
				if parent == path {
					break
				}
				path = parent
			}
			free, err := diskFree(path)
			c := healthCheck{ID: disk.id, Name: disk.name, Status: "ok", Message: "Storage space is available.", FreeBytes: &free}
			if err != nil {
				c.Status = "warning"
				c.Message = "Could not inspect available disk space."
				c.FreeBytes = nil
			} else if free < 512<<20 {
				c.Status = "warning"
				c.Message = "Less than 512 MiB is free. Free disk space before starting long tasks."
			}
			add(c)
		}
		cs, err := s.Store.Contributions(ctx)
		if err != nil {
			add(healthCheck{ID: "workspaces", Name: "Saved workspaces", Status: "failed", Message: "Saved workspaces could not be read."})
		} else if s.Execution != nil && !s.Store.Demo {
			missing, blocked := 0, 0
			for _, c := range cs {
				if c.State == "BLOCKED" || c.State == "PAUSED" {
					blocked++
				}
				if c.Workspace != "" && c.State != "SELECTED" && c.State != "PREPARING" {
					if _, _, err := s.Execution.Paths(c); err != nil {
						missing++
					}
				}
			}
			c := healthCheck{ID: "workspaces", Name: "Saved workspaces", Status: "ok", Message: fmt.Sprintf("%d contributions; %d paused or blocked. Resume requires your approval.", len(cs), blocked)}
			if missing > 0 {
				c.Status = "warning"
				c.Message = fmt.Sprintf("%d saved workspaces are missing or fail boundary checks. Inspect their paths before resuming.", missing)
			}
			add(c)
		}
	}
	response := map[string]any{"status": status, "mode": s.mode(), "checked_at": time.Now().UTC()}
	if detailed {
		response["checks"] = checks
	}
	write(w, code, response)
}
