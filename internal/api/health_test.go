package api

import (
	"context"
	"encoding/json"
	"forgeflow/internal/storage"
	"net/http/httptest"
	"testing"
)

func TestHealthReflectsDatabaseFailureAndToolUncertainty(t *testing.T) {
	store, err := storage.Open(context.Background(), ":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	s := Server{Store: store}
	handler := s.Handler()
	req := httptest.NewRequest("GET", "http://127.0.0.1/api/health/details", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	var result struct {
		Status string        `json:"status"`
		Checks []healthCheck `json:"checks"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || result.Status != "warning" || len(result.Checks) < 2 {
		t.Fatalf("unknown tool appeared healthy: %s", response.Body.String())
	}
	store.Close()
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1/api/health", nil))
	if response.Code != 503 {
		t.Fatalf("closed database healthy: %d", response.Code)
	}
}
