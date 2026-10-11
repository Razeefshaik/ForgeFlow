package api

import "net/http"

func (s Server) environmentStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Environment == nil || s.Store.Demo {
		write(w, 200, map[string]any{"available": false, "status": "unavailable", "message": "Managed sandbox setup is unavailable in this instance.", "checks": []any{}})
		return
	}
	write(w, 200, s.Environment.Status())
}
func (s Server) environmentAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		Approved bool   `json:"approved"`
		Mode     string `json:"mode"`
	}
	if !decode(w, r, &body) {
		return
	}
	if s.Environment == nil || s.Store.Demo {
		write(w, 503, map[string]string{"error": "Managed sandbox setup is unavailable in this instance"})
		return
	}
	status, err := s.Environment.Start(r.PathValue("action"), body.Mode, body.Approved)
	if err != nil {
		write(w, 409, map[string]string{"error": err.Error()})
		return
	}
	write(w, http.StatusAccepted, status)
}
