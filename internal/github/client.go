// Package github is a read-only, bounded GitHub REST adapter. It never executes repository content.
package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrBudget = errors.New("GitHub request budget exhausted; scan is bounded")

type APIError struct {
	Status  int
	RetryAt time.Time
}

func (e *APIError) Error() string {
	if !e.RetryAt.IsZero() {
		return "GitHub rate limit reached; retry after " + e.RetryAt.UTC().Format(time.RFC3339)
	}
	switch e.Status {
	case 401:
		return "GitHub authentication rejected; check the server credential"
	case 403:
		return "GitHub access denied"
	case 404:
		return "GitHub resource unavailable"
	default:
		return fmt.Sprintf("GitHub request failed (HTTP %d)", e.Status)
	}
}

type entry struct {
	Body  []byte
	ETag  string
	Until time.Time
}
type Client struct {
	HTTP       *http.Client
	BaseURL    string
	token      string
	generation uint64
	mu         sync.Mutex
	cache      map[string]entry
	cacheBytes int
	retryAt    time.Time
}
type Session struct {
	Client      *Client
	Requests    int
	MaxRequests int
	Fresh       bool // Revalidate with GitHub even when the discovery cache is unexpired.
}

func New(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, BaseURL: "https://api.github.com", token: token, cache: map[string]entry{}}
}

// Credential returns a synchronized server-side credential snapshot.
func (c *Client) Credential() string { c.mu.Lock(); defer c.mu.Unlock(); return c.token }
func (c *Client) SetCredential(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
	c.generation++
	c.cache = map[string]entry{}
	c.cacheBytes = 0
	c.retryAt = time.Time{}
}
func (c *Client) Snapshot() *Client {
	n := New(c.Credential())
	n.HTTP = c.HTTP
	n.BaseURL = c.BaseURL
	return n
}

// ResolveToken never sends credentials to the browser, database or logs.
func ResolveToken(ctx context.Context) (string, string) {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token, "environment"
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com")
	if b, err := cmd.Output(); err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimSpace(string(b)), "GitHub CLI"
	}
	return "", "public unauthenticated"
}
func (s *Session) Get(ctx context.Context, path string, target any) error {
	c := s.Client
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return errors.New("invalid GitHub endpoint")
	}
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return errors.New("invalid GitHub endpoint")
	}
	// An auth-specific cache prevents reuse across credential changes. Secrets remain memory-only.
	c.mu.Lock()
	token, generation := c.token, c.generation
	c.mu.Unlock()
	scope := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(scope[:]) + ":" + u.String()
	c.mu.Lock()
	cached, exists := c.cache[key]
	retry := c.retryAt
	c.mu.Unlock()
	if exists && !s.Fresh && time.Now().Before(cached.Until) {
		return json.Unmarshal(cached.Body, target)
	}
	if time.Now().Before(retry) {
		return &APIError{Status: 429, RetryAt: retry}
	}
	if s.Requests >= s.MaxRequests {
		return ErrBudget
	}
	s.Requests++
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return errors.New("invalid GitHub request")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "ForgeFlow/0.2")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if exists && cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("GitHub network request failed")
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if n, e := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); e == nil {
			c.mu.Lock()
			if c.generation == generation {
				c.retryAt = time.Unix(n, 0)
			}
			c.mu.Unlock()
		}
	}
	if resp.StatusCode == 304 && exists {
		cached.Until = time.Now().Add(15 * time.Minute)
		c.mu.Lock()
		if c.generation == generation {
			c.cache[key] = cached
		}
		c.mu.Unlock()
		return json.Unmarshal(cached.Body, target)
	}
	if resp.StatusCode != 200 {
		e := &APIError{Status: resp.StatusCode}
		if resp.StatusCode == 429 || (resp.StatusCode == 403 && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "")) {
			e.RetryAt = time.Now().Add(time.Minute)
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n > 0 {
				e.RetryAt = time.Now().Add(time.Duration(n) * time.Second)
			} else if t, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil {
				e.RetryAt = t
			} else if n, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				e.RetryAt = time.Unix(n, 0)
			}
			c.mu.Lock()
			if c.generation == generation {
				c.retryAt = e.RetryAt
			}
			c.mu.Unlock()
		}
		return e
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return errors.New("GitHub response could not be read")
	}
	if len(b) > 4<<20 {
		return errors.New("GitHub response exceeds 4 MiB limit")
	}
	if err = json.Unmarshal(b, target); err != nil {
		return errors.New("GitHub returned invalid JSON")
	}
	c.mu.Lock()
	if c.generation != generation {
		c.mu.Unlock()
		return nil
	}
	if previous, ok := c.cache[key]; ok {
		c.cacheBytes -= len(previous.Body)
		delete(c.cache, key)
	}
	if len(c.cache) >= 256 || c.cacheBytes+len(b) > 16<<20 { // Bound both entry count and aggregate body size.
		for k, v := range c.cache {
			if time.Now().After(v.Until) {
				c.cacheBytes -= len(v.Body)
				delete(c.cache, k)
			}
		}
		for len(c.cache) >= 256 || c.cacheBytes+len(b) > 16<<20 {
			for k, v := range c.cache {
				c.cacheBytes -= len(v.Body)
				delete(c.cache, k)
				break
			}
		}
	}
	c.cache[key] = entry{Body: b, ETag: resp.Header.Get("ETag"), Until: time.Now().Add(15 * time.Minute)}
	c.cacheBytes += len(b)
	c.mu.Unlock()
	return nil
}

type User struct {
	Login string `json:"login"`
}
type Label struct {
	Name string `json:"name"`
}
type Issue struct {
	Number        int             `json:"number"`
	Title         string          `json:"title"`
	Body          string          `json:"body"`
	HTMLURL       string          `json:"html_url"`
	RepositoryURL string          `json:"repository_url"`
	State         string          `json:"state"`
	Labels        []Label         `json:"labels"`
	Assignees     []User          `json:"assignees"`
	Comments      int             `json:"comments"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	PullRequest   json.RawMessage `json:"pull_request"`
}
type Repository struct {
	FullName      string    `json:"full_name"`
	HTMLURL       string    `json:"html_url"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	Topics        []string  `json:"topics"`
	Stars         int       `json:"stargazers_count"`
	Forks         int       `json:"forks_count"`
	Archived      bool      `json:"archived"`
	Disabled      bool      `json:"disabled"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	PushedAt      time.Time `json:"pushed_at"`
}
type Comment struct {
	AuthorAssociation string    `json:"author_association"`
	CreatedAt         time.Time `json:"created_at"`
}
type Pull struct {
	HTMLURL           string     `json:"html_url"`
	Body              string     `json:"body"`
	Title             string     `json:"title"`
	AuthorAssociation string     `json:"author_association"`
	MergedAt          *time.Time `json:"merged_at"`
}
type Tree struct {
	Truncated bool `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}
type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}
type Release struct {
	PublishedAt *time.Time `json:"published_at"`
	Draft       bool       `json:"draft"`
	Prerelease  bool       `json:"prerelease"`
}
type IssueSearch struct {
	Items      []Issue `json:"items"`
	Incomplete bool    `json:"incomplete_results"`
	Total      int     `json:"total_count"`
}
type RepoSearch struct {
	Items      []Repository `json:"items"`
	Incomplete bool         `json:"incomplete_results"`
	Total      int          `json:"total_count"`
}

func SearchPath(kind, query string, limit int) string {
	return "/search/" + kind + "?" + url.Values{"q": {query}, "per_page": {strconv.Itoa(limit)}, "sort": {"updated"}, "order": {"desc"}}.Encode()
}
