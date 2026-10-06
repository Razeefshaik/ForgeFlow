package githubauth

import (
	"context"
	"encoding/json"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDeviceLoginPersistenceRefreshLogout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI requires Windows")
	}
	stage := "authorization_pending"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			r.ParseForm()
			if r.Form.Get("scope") != "public_repo offline_access" {
				t.Error("unexpected scopes")
			}
			if r.Form.Get("client_secret") != "" {
				t.Error("device flow must not use a client secret")
			}
			json.NewEncoder(w).Encode(oauthResponse{DeviceCode: "private-device-code", UserCode: "TEST-CODE", URI: "https://github.com/login/device", Expires: 900, Interval: 5})
		case "/login/oauth/access_token":
			r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" {
				if r.Form.Get("refresh_token") != "refresh-one" {
					t.Error("wrong refresh token")
				}
				json.NewEncoder(w).Encode(oauthResponse{Token: "token-two", Refresh: "refresh-two", Expires: 28800})
				return
			}
			if r.Form.Get("device_code") != "private-device-code" {
				t.Error("missing server-side device code")
			}
			if stage != "" {
				json.NewEncoder(w).Encode(oauthResponse{Error: stage})
				return
			}
			json.NewEncoder(w).Encode(oauthResponse{Token: "token-one", Refresh: "refresh-one", Expires: 28800})
		case "/user":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer token-") {
				t.Error("identity verification unauthenticated")
			}
			w.Write([]byte(`{"login":"test-user"}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := github.New("")
	c.BaseURL = server.URL
	dir := t.TempDir()
	s, err := newAt(context.Background(), dir, c, "public unauthenticated")
	if err != nil {
		t.Fatal(err)
	}
	s.cancel()
	s.wg.Wait()
	s.ctx = context.Background() // Drive polling deterministically below.
	s.OAuthURL = server.URL
	if err = s.Configure("Ov23liForgeFlowTest"); err != nil {
		t.Fatal(err)
	}
	if err = s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s.Status())
	if strings.Contains(string(b), "private-device") || strings.Contains(string(b), "token-one") {
		t.Fatal("credential leaked in public status")
	}
	s.next = time.Now().Add(-time.Second)
	s.tick()
	if !s.Status().Pending {
		t.Fatal("pending challenge lost")
	}
	stage = "slow_down"
	s.next = time.Now().Add(-time.Second)
	s.tick()
	if s.interval != 10*time.Second {
		t.Fatal("slow_down was not honored")
	}
	stage = ""
	s.next = time.Now().Add(-time.Second)
	s.tick()
	if !s.Status().Connected || c.Credential() != "token-one" || s.Status().Login != "test-user" {
		t.Fatal("authorization did not connect")
	}
	b, err = os.ReadFile(filepath.Join(dir, "credential.dpapi"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "token-one") || strings.Contains(string(b), "refresh-one") {
		t.Fatal("credential saved in plaintext")
	}
	s.Close()
	restored, err := newAt(context.Background(), dir, github.New(""), "public unauthenticated")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.cancel()
	restored.wg.Wait()
	restored.ctx = context.Background()
	restored.OAuthURL = server.URL
	restored.client.BaseURL = server.URL
	if !restored.Status().Connected || restored.client.Credential() != "token-one" {
		t.Fatal("restart did not restore account")
	}
	restored.saved.Expires = time.Now().Add(time.Minute)
	restored.tick()
	if restored.client.Credential() != "token-two" || restored.saved.Refresh != "refresh-two" {
		t.Fatal("expiring credential was not refreshed")
	}
	if err = restored.Logout(); err != nil {
		t.Fatal(err)
	}
	if restored.client.Credential() != "" || restored.Status().Connected {
		t.Fatal("logout retained credential")
	}
	if _, err = os.Stat(filepath.Join(dir, "credential.dpapi")); !os.IsNotExist(err) {
		t.Fatal("logout retained saved credential")
	}
}
func TestDeviceDeniedAndCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(oauthResponse{Error: "access_denied"})
	}))
	defer server.Close()
	s, err := newAt(context.Background(), t.TempDir(), github.New(""), "public unauthenticated")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.cancel()
	s.wg.Wait()
	s.ctx = context.Background()
	s.OAuthURL = server.URL
	if err = s.Start(context.Background()); err == nil {
		t.Fatal("missing setup accepted")
	}
	if err = s.Configure("Ov23liForgeFlowTest"); err != nil {
		t.Fatal(err)
	}
	s.state.Pending = true
	s.device = "private"
	s.state.ExpiresAt = time.Now().Add(time.Minute)
	s.tick()
	if s.Status().Pending || !strings.Contains(s.Status().Message, "declined") {
		t.Fatal("denied authorization remained pending")
	}
	s.state.Pending = true
	s.device = "cancel-me"
	if err = s.Logout(); err != nil {
		t.Fatal(err)
	}
	if s.device != "" || s.Status().Pending {
		t.Fatal("cancel retained device flow")
	}
}
