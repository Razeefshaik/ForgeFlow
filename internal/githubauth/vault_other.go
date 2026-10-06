//go:build !windows

package githubauth

import "errors"

func protect([]byte) ([]byte, error) {
	return nil, errors.New("persistent browser sign-in currently requires Windows credential protection")
}
func unprotect([]byte) ([]byte, error) {
	return nil, errors.New("persistent browser sign-in currently requires Windows credential protection")
}
