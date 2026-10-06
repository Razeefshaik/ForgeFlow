package execution

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) PreparePR(ctx context.Context, id string) (domain.ExecutionRecord, error) {
	if err := s.beginPR(id); err != nil {
		return domain.ExecutionRecord{}, err
	}
	defer s.endPR(id)
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return domain.ExecutionRecord{}, err
	}
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		return r, err
	}
	if c.State == "PR_PREPARED" {
		return r, nil
	}
	if c.State != "READY" || r.Review == nil || r.Review.Verdict != "APPROVE" {
		return r, errors.New("completed tests and an independent approval are required before PR preparation")
	}
	repo, _, err := s.Paths(c)
	if err != nil {
		return r, err
	}
	diff, _, err := s.diff(ctx, repo, c.BaseCommit)
	if err != nil {
		return r, err
	}
	if diff != r.Diff {
		return r, errors.New("working changes differ from reviewed changes; rerun testing and review")
	}
	current, err := s.git(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return r, err
	}
	if strings.TrimSpace(current) == c.BaseCommit {
		if _, err = s.git(ctx, repo, "add", "--all", "--", "."); err != nil {
			return r, err
		}
		if _, err = s.git(ctx, repo, "-c", "user.name=ForgeFlow", "-c", "user.email=forgeflow@localhost", "-c", "commit.gpgsign=false", "commit", "-m", r.PRTitle); err != nil {
			return r, err
		}
	} else {
		status, e := s.git(ctx, repo, "status", "--porcelain")
		if e != nil || strings.TrimSpace(status) != "" {
			return r, errors.New("unexpected Git history or uncommitted changes during PR preparation")
		}
		parent, e := s.git(ctx, repo, "rev-parse", "HEAD^")
		if e != nil || strings.TrimSpace(parent) != c.BaseCommit {
			return r, errors.New("prepared commit must have the approved base as its parent")
		}
	}
	sha, err := s.git(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return r, err
	}
	r.HeadCommit = strings.TrimSpace(sha)
	r.Status = "PR_PREPARED"
	r.SubmissionToken = submissionToken(r)
	if err = s.Store.SaveExecution(ctx, r, "user", "Prepared local commit and PR title/body; no push or submission"); err != nil {
		return r, err
	}
	if err = s.Store.Transition(ctx, id, "PR_PREPARED"); err != nil {
		return r, err
	}
	return r, nil
}
func submissionToken(r domain.ExecutionRecord) string {
	h := sha256.Sum256([]byte(r.ContributionID + "\x00" + r.HeadCommit + "\x00" + r.PRTitle + "\x00" + r.PRBody))
	return hex.EncodeToString(h[:])
}
func (s *Service) SubmitPR(ctx context.Context, id, token string, approved bool) (domain.ExecutionRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := s.beginPR(id); err != nil {
		return domain.ExecutionRecord{}, err
	}
	defer s.endPR(id)
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		return r, err
	}
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return r, err
	}
	if !approved || token == "" || token != r.SubmissionToken {
		return r, errors.New("explicit human submission approval for this commit and PR text is required")
	}
	if c.State == "PR_OPENED" && r.PRURL != "" {
		return r, nil
	}
	if c.State != "PR_PREPARED" || s.Client == nil || s.Client.Token == "" {
		return r, errors.New("prepare PR first and authenticate GitHub with fork/push/PR permissions")
	}
	approval, err := s.Store.Approval(ctx, id)
	if err != nil {
		return r, err
	}
	if err = s.freshIssue(ctx, approval); err != nil {
		return r, err
	}
	repo, _, err := s.Paths(c)
	if err != nil {
		return r, err
	}
	sha, err := s.git(ctx, repo, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(sha) != r.HeadCommit {
		return r, errors.New("prepared commit changed; inspect before submission")
	}
	status, err := s.git(ctx, repo, "status", "--porcelain")
	if err != nil || strings.TrimSpace(status) != "" {
		return r, errors.New("workspace has changes after preparation; submission refused")
	}
	if err = s.Store.WorkspaceEvent(ctx, id, "PRSubmissionApproved", "Human approved fork, branch push and PR submission", map[string]string{"commit": r.HeadCommit}); err != nil {
		return r, err
	}
	session := github.Session{Client: s.Client, MaxRequests: 20, Fresh: true}
	var user github.User
	if err = session.Get(ctx, "/user", &user); err != nil {
		return r, err
	}
	if !safeGitHubName(user.Login) {
		return r, errors.New("invalid authenticated GitHub account")
	}
	parts := strings.Split(c.Repository, "/")
	if len(parts) != 2 {
		return r, errors.New("invalid contribution repository")
	}
	forkName := user.Login + "/" + parts[1]
	if !strings.EqualFold(forkName, c.Repository) {
		createdFork := false
		var fork struct {
			FullName string `json:"full_name"`
			Parent   struct {
				FullName string `json:"full_name"`
			} `json:"parent"`
		}
		err = session.Get(ctx, "/repos/"+forkName, &fork)
		var apiError *github.APIError
		if errors.As(err, &apiError) && apiError.Status == 404 {
			err = s.Client.Write(ctx, "/repos/"+c.Repository+"/forks", map[string]any{}, &fork)
			createdFork = err == nil
		}
		if err != nil {
			return r, err
		}
		if !strings.EqualFold(fork.FullName, forkName) || !strings.EqualFold(fork.Parent.FullName, c.Repository) {
			return r, errors.New("existing repository is not the intended upstream fork")
		}
		if createdFork {
			var upstream github.Repository
			if err = json.Unmarshal(approval.Repository, &upstream); err != nil {
				return r, err
			}
			ready := false
			for attempt := 0; attempt < 6; attempt++ {
				var ref struct {
					Object struct {
						SHA string `json:"sha"`
					} `json:"object"`
				}
				err = session.Get(ctx, "/repos/"+forkName+"/git/ref/heads/"+upstream.DefaultBranch, &ref)
				if err == nil && len(ref.Object.SHA) == 40 {
					ready = true
					break
				}
				if err != nil && !(errors.As(err, &apiError) && apiError.Status == 404) {
					return r, err
				}
				select {
				case <-ctx.Done():
					return r, ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}
			if !ready {
				return r, errors.New("GitHub fork is still initializing; retry approved submission later")
			}
		}
	}
	header := "AUTHORIZATION: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+s.Client.Token))
	cmd := exec.CommandContext(ctx, "git", "-c", "credential.helper=", "-c", "core.hooksPath="+filepath.Join(filepath.Dir(repo), ".autopilot", "empty-hooks"), "push", "https://github.com/"+forkName+".git", "HEAD:refs/heads/"+c.Branch)
	cmd.Dir = repo
	cmd.Env = append(gitEnv(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0="+header)
	cmd.WaitDelay = 3 * time.Second
	codex.ConfigureProcess(cmd)
	if err = cmd.Run(); err != nil {
		return r, errors.New("push to approved fork failed; no force push was attempted")
	}
	var existing []struct {
		HTMLURL string `json:"html_url"`
	}
	if err = session.Get(ctx, "/repos/"+c.Repository+"/pulls?state=open&head="+user.Login+":"+c.Branch, &existing); err != nil {
		return r, err
	}
	if len(existing) > 0 {
		r.PRURL = existing[0].HTMLURL
	} else {
		var created struct {
			HTMLURL string `json:"html_url"`
		}
		approval, e := s.Store.Approval(ctx, id)
		if e != nil {
			return r, e
		}
		var repository github.Repository
		if err = json.Unmarshal(approval.Repository, &repository); err != nil {
			return r, err
		}
		err = s.Client.Write(ctx, "/repos/"+c.Repository+"/pulls", map[string]any{"title": r.PRTitle, "body": r.PRBody, "head": user.Login + ":" + c.Branch, "base": repository.DefaultBranch, "maintainer_can_modify": true}, &created)
		if err != nil {
			return r, err
		}
		r.PRURL = created.HTMLURL
	}
	if !strings.HasPrefix(r.PRURL, "https://github.com/"+c.Repository+"/pull/") {
		return r, errors.New("GitHub returned an invalid PR URL")
	}
	r.Status = "PR_OPENED"
	if err = s.Store.SaveExecution(ctx, r, "user", "Human approved PR submitted: "+r.PRURL); err != nil {
		return r, err
	}
	if err = s.Store.Transition(ctx, id, "PR_OPENED"); err != nil {
		return r, err
	}
	return r, nil
}
func safeGitHubName(v string) bool {
	if v == "" || len(v) > 100 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
