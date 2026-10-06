package contributions

import (
	"fmt"
	"path/filepath"
	"regexp"
)

var contributionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

// WorkspacePath names an external repository. It does not create or execute a contribution.
// The workspace manager additionally enforces resolved symlink/junction boundaries before creation.
func WorkspacePath(projectRoot, id string) (string, error) {
	if !filepath.IsAbs(projectRoot) {
		return "", fmt.Errorf("project root must be absolute")
	}
	if !contributionID.MatchString(id) {
		return "", fmt.Errorf("invalid contribution ID")
	}
	return filepath.Join(projectRoot, "contributions", id, "repo"), nil
}

func WorkspacePathAt(base, id string) (string, error) {
	if !filepath.IsAbs(base) || !contributionID.MatchString(id) {
		return "", fmt.Errorf("absolute contribution root and valid ID required")
	}
	return filepath.Join(filepath.Clean(base), id, "repo"), nil
}
