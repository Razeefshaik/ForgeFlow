// Package githubauth implements GitHub's device authorization flow. Credentials
// remain server-side and are encrypted by Windows DPAPI for the current user.
package githubauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"forgeflow/internal/github"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Status struct {
	Configured      bool      `json:"configured"`
	ClientID        string    `json:"client_id"`
	Connected       bool      `json:"connected"`
	Source          string    `json:"source"`
	Login           string    `json:"login"`
	Pending         bool      `json:"pending"`
	UserCode        string    `json:"user_code,omitempty"`
	VerificationURI string    `json:"verification_uri,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Message         string    `json:"message"`
}
type credential struct {
	Token   string
	Refresh string
	Expires time.Time
	Login   string
}
type oauthResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	URI        string `json:"verification_uri"`
	Expires    int    `json:"expires_in"`
	Interval   int    `json:"interval"`
	Token      string `json:"access_token"`
	Refresh    string `json:"refresh_token"`
	Error      string `json:"error"`
}
type Service struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	client   *github.Client
	dir      string
	state    Status
	saved    credential
	device   string
	next     time.Time
	interval time.Duration
	HTTP     *http.Client
	OAuthURL string
	Changed  func(string)
}

func New(ctx context.Context, root string, client *github.Client, source string) (*Service, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return newAt(ctx, filepath.Join(base, "ForgeFlow", hex.EncodeToString(hash[:8])), client, source)
}
func newAt(ctx context.Context, dir string, client *github.Client, source string) (*Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, client: client, dir: dir, HTTP: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, OAuthURL: "https://github.com", state: Status{Source: source, Connected: client.Credential() != ""}}
	if b, err := os.ReadFile(filepath.Join(dir, "client-id")); err == nil {
		s.state.ClientID = strings.TrimSpace(string(b))
	}
	if id := strings.TrimSpace(os.Getenv("FORGEFLOW_GITHUB_CLIENT_ID")); id != "" {
		s.state.ClientID = id
	}
	s.state.Configured = s.state.ClientID != ""
	if b, err := os.ReadFile(filepath.Join(dir, "credential.dpapi")); err == nil {
		plain, e := unprotect(b)
		if e != nil || json.Unmarshal(plain, &s.saved) != nil {
			s.state.Message = "Saved sign-in could not be restored. Sign in again."
		} else if s.saved.Token != "" {
			s.client.SetCredential(s.saved.Token)
			s.state.Connected = true
			s.state.Source = "GitHub sign-in"
			s.state.Login = s.saved.Login
			if !s.saved.Expires.IsZero() && time.Now().After(s.saved.Expires) {
				s.client.SetCredential("")
				s.state.Connected = false
				s.state.Source = "public unauthenticated"
			}
		}
	}
	s.wg.Add(1)
	go s.loop()
	return s, nil
}
func (s *Service) SetChanged(fn func(string)) { s.mu.Lock(); defer s.mu.Unlock(); s.Changed = fn }
func (s *Service) Close()                     { s.cancel(); s.wg.Wait() }
func (s *Service) Status() Status             { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Service) Configure(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Pending || s.state.Connected {
		return errors.New("sign out or cancel sign-in before changing the OAuth app")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{10,100}$`).MatchString(id) || strings.HasPrefix(id, "ghp_") || strings.HasPrefix(id, "gho_") || strings.HasPrefix(id, "github_pat_") {
		return errors.New("enter the OAuth app Client ID, not a token or client secret")
	}
	if err := s.write("client-id", []byte(id)); err != nil {
		return err
	}
	s.state.ClientID = id
	s.state.Configured = true
	s.state.Message = "OAuth app saved. Sign in with GitHub."
	return nil
}
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Configured {
		return errors.New("register the ForgeFlow OAuth app and save its Client ID first")
	}
	if s.state.Connected {
		return errors.New("already connected; sign out before changing accounts")
	}
	if s.state.Pending {
		return errors.New("a GitHub sign-in is already pending")
	}
	var result oauthResponse
	if err := s.post(ctx, "/login/device/code", url.Values{"client_id": {s.state.ClientID}, "scope": {"public_repo offline_access"}}, &result); err != nil {
		return err
	}
	if result.Error != "" {
		return oauthError(result.Error)
	}
	if result.DeviceCode == "" || result.UserCode == "" || result.URI != "https://github.com/login/device" || result.Expires <= 0 || result.Expires > 1800 {
		return errors.New("GitHub returned an invalid authorization challenge")
	}
	s.device = result.DeviceCode
	s.state.Pending = true
	s.state.UserCode = result.UserCode
	s.state.VerificationURI = result.URI
	s.state.ExpiresAt = time.Now().Add(time.Duration(result.Expires) * time.Second)
	s.interval = time.Duration(max(1, result.Interval)) * time.Second
	s.next = time.Now().Add(s.interval)
	s.state.Message = "Enter this code on GitHub and approve ForgeFlow. Keep this page open."
	return nil
}
func (s *Service) Logout() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(filepath.Join(s.dir, "credential.dpapi")); err != nil && !os.IsNotExist(err) {
		return errors.New("could not remove saved GitHub credential")
	}
	s.saved = credential{}
	s.clearPending()
	s.client.SetCredential("")
	s.state.Connected = false
	s.state.Login = ""
	s.state.Source = "public unauthenticated"
	s.state.Message = "Signed out of ForgeFlow. You can also revoke access in GitHub settings."
	if s.Changed != nil {
		s.Changed(s.state.Source)
	}
	return nil
}
func (s *Service) clearPending() {
	s.device = ""
	s.state.Pending = false
	s.state.UserCode = ""
	s.state.VerificationURI = ""
	s.state.ExpiresAt = time.Time{}
}
func (s *Service) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}
func (s *Service) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Pending {
		if time.Now().After(s.state.ExpiresAt) {
			s.clearPending()
			s.state.Message = "Sign-in code expired. Start sign-in again."
			return
		}
		if time.Now().Before(s.next) {
			return
		}
		var result oauthResponse
		err := s.post(s.ctx, "/login/oauth/access_token", url.Values{"client_id": {s.state.ClientID}, "device_code": {s.device}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &result)
		s.next = time.Now().Add(s.interval)
		if err != nil {
			s.state.Message = err.Error()
			return
		}
		switch result.Error {
		case "authorization_pending":
			return
		case "slow_down":
			s.interval += 5 * time.Second
			s.next = time.Now().Add(s.interval)
			return
		case "":
		default:
			s.clearPending()
			s.state.Message = oauthError(result.Error).Error()
			return
		}
		if err = s.accept(result); err != nil {
			s.state.Message = err.Error()
			s.clearPending()
		}
		return
	}
	if s.saved.Refresh != "" && !s.saved.Expires.IsZero() && time.Now().After(s.saved.Expires.Add(-2*time.Minute)) && !time.Now().Before(s.next) {
		var result oauthResponse
		err := s.post(s.ctx, "/login/oauth/access_token", url.Values{"client_id": {s.state.ClientID}, "refresh_token": {s.saved.Refresh}, "grant_type": {"refresh_token"}}, &result)
		s.next = time.Now().Add(time.Minute)
		if err == nil && result.Error == "" {
			err = s.accept(result)
		}
		if err != nil || result.Error != "" {
			s.state.Message = "GitHub session refresh failed; retrying. Sign in again if access was revoked."
			if time.Now().After(s.saved.Expires) {
				s.client.SetCredential("")
				s.state.Connected = false
				s.state.Source = "public unauthenticated"
				if s.Changed != nil {
					s.Changed("public unauthenticated")
				}
			}
		}
	}
}
func (s *Service) accept(result oauthResponse) error {
	if result.Token == "" {
		return errors.New("GitHub did not return an access token")
	}
	c := github.New(result.Token)
	c.HTTP = s.client.HTTP
	c.BaseURL = s.client.BaseURL
	var user github.User
	if err := (&github.Session{Client: c, MaxRequests: 1, Fresh: true}).Get(s.ctx, "/user", &user); err != nil {
		return errors.New("GitHub account could not be verified; sign in again")
	}
	if user.Login == "" {
		return errors.New("GitHub account identity missing")
	}
	saved := credential{Token: result.Token, Refresh: result.Refresh, Login: user.Login}
	if result.Expires > 0 {
		saved.Expires = time.Now().Add(time.Duration(result.Expires) * time.Second)
	}
	b, _ := json.Marshal(saved)
	encrypted, err := protect(b)
	if err != nil {
		return err
	}
	if err = s.write("credential.dpapi", encrypted); err != nil {
		return err
	}
	s.saved = saved
	s.client.SetCredential(saved.Token)
	s.state.Connected = true
	s.state.Login = saved.Login
	s.state.Source = "GitHub sign-in"
	s.state.Message = "Connected. Your sign-in is saved for this Windows account."
	s.clearPending()
	if s.Changed != nil {
		s.Changed(s.state.Source)
	}
	return nil
}
func (s *Service) write(name string, data []byte) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return errors.New("could not create GitHub account storage")
	}
	temp, err := os.CreateTemp(s.dir, ".account-*")
	if err != nil {
		return errors.New("could not save GitHub account")
	}
	path := temp.Name()
	defer os.Remove(path)
	if _, err = temp.Write(data); err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(path, filepath.Join(s.dir, name))
	}
	if err != nil {
		return errors.New("could not save GitHub account")
	}
	return nil
}
func (s *Service) post(ctx context.Context, path string, body url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, "POST", s.OAuthURL+path, strings.NewReader(body.Encode()))
	if err != nil {
		return errors.New("invalid GitHub authorization request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return errors.New("GitHub authorization network request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("GitHub authorization request rejected; check OAuth app settings and retry later")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(b) > 65536 || json.Unmarshal(b, out) != nil {
		return errors.New("invalid GitHub authorization response")
	}
	return nil
}
func oauthError(code string) error {
	switch code {
	case "device_flow_disabled":
		return errors.New("enable Device Flow in your GitHub OAuth app settings")
	case "access_denied":
		return errors.New("GitHub sign-in was declined")
	case "expired_token", "token_expired":
		return errors.New("GitHub sign-in code expired; try again")
	case "incorrect_client_credentials":
		return errors.New("GitHub rejected the OAuth Client ID")
	}
	return errors.New("GitHub authorization failed; check OAuth app settings and try again")
}
