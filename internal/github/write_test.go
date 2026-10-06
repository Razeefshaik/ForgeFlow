package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmissionWriteAuthAllowlistAndBoundedResponse(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing authenticated POST")
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"html_url":"https://github.com/example/repo/pull/1"}`))
	}))
	defer server.Close()
	client := New("")
	client.BaseURL = server.URL
	if err := client.Write(context.Background(), "/repos/example/repo/pulls", map[string]string{}, nil); err == nil || calls != 0 {
		t.Fatal("unauthenticated write was not rejected")
	}
	client.SetCredential("fixture-token")
	for _, p := range []string{"/repos/example/repo/issues", "/repos/example/repo/pulls/1", "/repos/../repo/pulls", "/user"} {
		if err := client.Write(context.Background(), p, map[string]string{}, nil); err == nil {
			t.Fatal("unapproved endpoint", p)
		}
	}
	if calls != 0 {
		t.Fatal("invalid endpoints reached server")
	}
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := client.Write(context.Background(), "/repos/example/repo/pulls", map[string]string{"title": "fixture"}, &result); err != nil || result.HTMLURL == "" || calls != 1 {
		t.Fatal("valid submission failed", err)
	}
}
