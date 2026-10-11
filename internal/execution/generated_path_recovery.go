package execution

import (
	"context"
	"errors"
	"fmt"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os"
	"path/filepath"
	"strings"
)

// Quarantine preserves the directory and its ACL. Only an empty ignored output
// directory is eligible; no ACL edits, recursive moves or package exclusions.
func (s *Service) quarantineGeneratedDirectory(ctx context.Context, id, repo string, r *domain.ExecutionRecord, d *domain.ExecutionIncident) error {
	path, rel, err := generatedDirectory(repo, d.Path)
	if err != nil {
		return err
	}
	tracked, err := s.git(ctx, repo, "ls-files", "-z", "--", ":(literal)"+filepath.ToSlash(rel))
	if err != nil {
		return err
	}
	if tracked != "" {
		return errors.New("the path contains tracked files")
	}
	ignored, err := s.git(ctx, repo, "check-ignore", "--no-index", "--", filepath.ToSlash(rel))
	if err != nil || strings.TrimSpace(ignored) == "" {
		return errors.New("the directory is not ignored by Git")
	}
	if err = codex.PrepareWorkspaceRuntime(repo); err != nil {
		return err
	}
	// A unique destination directly in the validated runtime root cannot traverse
	// a pre-existing quarantine link planted by repository code.
	target := filepath.Join(repo, ".forgeflow-runtime", "quarantine-"+storage.ID())
	if _, err = os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return errors.New("quarantine destination already exists or is inaccessible")
	}
	if err = s.recoveryAction(ctx, r, d, "Preserving an empty generated directory before retrying package discovery"); err != nil {
		return err
	}
	evidence := map[string]string{"original_path": path, "quarantine_path": target, "test_run_id": d.TestRunID}
	if err = s.Store.WorkspaceEvent(ctx, id, "GeneratedDirectoryQuarantinePlanned", "Validated an empty ignored output directory; no files will be deleted", evidence); err != nil {
		return err
	}
	// Recheck after auditing and before moving; the contribution agent is stopped.
	if _, _, err = generatedDirectory(repo, d.Path); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(path, target); err != nil {
		return fmt.Errorf("could not preserve directory: %w", err)
	}
	return s.Store.WorkspaceEvent(ctx, id, "GeneratedDirectoryQuarantined", "Preserved the empty generated directory; rerunning the original verification command", evidence)
}

func generatedDirectory(repo, reported string) (string, string, error) {
	// Normalize separators for both Go diagnostics and platform-independent tests.
	reported = strings.ReplaceAll(reported, "\\", "/")
	if strings.ContainsAny(reported, "\x00\r\n") || strings.Contains(reported, ":") || strings.HasPrefix(reported, "/") {
		return "", "", errors.New("repair requires a relative workspace output path")
	}
	for _, part := range strings.Split(reported, "/") {
		if part == ".." || strings.EqualFold(part, ".git") {
			return "", "", errors.New("path crosses a protected boundary")
		}
	}
	rel := filepath.Clean(filepath.FromSlash(reported))
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 || !contains([]string{"bin", "build", "dist", "tmp", ".forgeflow-runtime"}, parts[0]) {
		return "", "", errors.New("path is not inside a recognized generated-output directory")
	}
	path := filepath.Join(repo, rel)
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("path is missing, linked or not a directory")
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(p)) {
			return "", "", errors.New("path crosses a link boundary")
		}
		if p == repo {
			break
		}
		if filepath.Dir(p) == p {
			return "", "", errors.New("path escaped workspace")
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", "", errors.New("directory contents cannot be inspected")
	}
	if len(entries) != 0 {
		return "", "", errors.New("directory is not empty; preserve its contents and inspect permissions")
	}
	return path, rel, nil
}
