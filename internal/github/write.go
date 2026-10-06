package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Write is used only after the application's explicit PR submission approval.
func (c *Client) Write(ctx context.Context, path string, body, target any) error {
	token := c.Credential()
	if token == "" {
		return errors.New("authenticated GitHub credential required for submission")
	}
	if !strings.HasPrefix(path, "/repos/") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
		return errors.New("invalid GitHub write endpoint")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 4 || (parts[3] != "forks" && parts[3] != "pulls") || parts[1] == "" || parts[2] == "" {
		return errors.New("GitHub write endpoint is not allowlisted")
	}
	b, e := json.Marshal(body)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(b))
	if e != nil {
		return errors.New("invalid GitHub write request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "ForgeFlow")
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return errors.New("GitHub write request failed; inspect repository before retrying")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 202 {
		return &APIError{Status: resp.StatusCode}
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if e != nil || len(raw) > 4<<20 {
		return errors.New("GitHub write response exceeded limit")
	}
	if target != nil {
		if e = json.Unmarshal(raw, target); e != nil {
			return errors.New("GitHub write returned invalid JSON")
		}
	}
	return nil
}
