package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestCredentialChangeDoesNotInheritInflightRateLimit(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer old" {
			close(entered)
			<-release
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			w.WriteHeader(403)
			return
		}
		if r.Header.Get("Authorization") != "Bearer new" {
			t.Error("new credential was not used")
		}
		w.Write([]byte(`{"login":"new-user"}`))
	}))
	defer server.Close()
	c := New("old")
	c.BaseURL = server.URL
	done := make(chan error, 1)
	go func() {
		var user User
		done <- (&Session{Client: c, MaxRequests: 1}).Get(context.Background(), "/user", &user)
	}()
	<-entered
	c.SetCredential("new")
	close(release)
	if <-done == nil {
		t.Fatal("old request should be rate limited")
	}
	var user User
	if err := (&Session{Client: c, MaxRequests: 1}).Get(context.Background(), "/user", &user); err != nil {
		t.Fatal("old credential poisoned new session:", err)
	}
	if user.Login != "new-user" {
		t.Fatal("wrong account data")
	}
}
