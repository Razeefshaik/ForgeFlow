package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCacheConditionalRequestsBudgetAndCredentialIsolation(t *testing.T) {
	n := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if r.Method != "GET" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Error("invalid request")
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, `{"number":42}`)
	}))
	defer server.Close()
	c := New("test-secret")
	c.BaseURL = server.URL
	s := Session{Client: c, MaxRequests: 4}
	var v Issue
	for range 2 {
		if err := s.Get(context.Background(), "/issue", &v); err != nil {
			t.Fatal(err)
		}
	}
	if n != 1 || v.Number != 42 || s.Requests != 1 {
		t.Fatalf("cache ineffective: %d %+v", n, s)
	}
	c.mu.Lock()
	for k, e := range c.cache {
		e.Until = time.Now().Add(-time.Second)
		c.cache[k] = e
	}
	c.mu.Unlock()
	if err := s.Get(context.Background(), "/issue", &v); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatal("conditional request missing")
	}
	c.Token = "different-secret"
	if err := s.Get(context.Background(), "/issue", &v); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("cache crossed credential scope")
	}
	s.MaxRequests = s.Requests
	if err := s.Get(context.Background(), "/other", &v); !errors.Is(err, ErrBudget) {
		t.Fatal("request budget ignored", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Get(ctx, "/issue", &v); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled cache request accepted", err)
	}
}
func TestRateLimitBackoffAndErrorsNeverExposeResponseSecrets(t *testing.T) {
	n := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"message":"secret-token"}`)
	}))
	defer server.Close()
	c := New("secret-token")
	c.BaseURL = server.URL
	s := Session{Client: c, MaxRequests: 5}
	var v any
	for range 2 {
		err := s.Get(context.Background(), "/limited", &v)
		var api *APIError
		if !errors.As(err, &api) || api.RetryAt.IsZero() || strings.Contains(err.Error(), "secret-token") {
			t.Fatal(err)
		}
	}
	if n != 1 {
		t.Fatal("repeated request while rate limited")
	}
}
func TestRejectRedirectAndOversizeResponses(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL, 302)
			return
		}
		fmt.Fprint(w, strings.Repeat("x", (4<<20)+1))
	}))
	defer server.Close()
	c := New("secret")
	c.BaseURL = server.URL
	s := Session{Client: c, MaxRequests: 2}
	var v any
	if err := s.Get(context.Background(), "/redirect", &v); err == nil || leaked {
		t.Fatal("redirect followed")
	}
	if err := s.Get(context.Background(), "/large", &v); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatal("response limit ignored", err)
	}
}
func TestEnvironmentCredentialPrecedence(t *testing.T) {
	t.Setenv("GH_TOKEN", " test ")
	t.Setenv("GITHUB_TOKEN", "second")
	token, source := ResolveToken(context.Background())
	if token != "test" || source != "environment" {
		t.Fatal("credential precedence incorrect")
	}
}
