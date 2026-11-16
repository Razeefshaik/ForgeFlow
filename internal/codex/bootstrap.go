package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// A Docker Desktop reset, an image prune or a new machine removes the local
// verification image. ForgeFlow used to print a warning and then stop every
// contribution at its first check. The bootstrap image is built from a
// Dockerfile in this project, so ForgeFlow rebuilds it instead.

// The build downloads two base images (over a gigabyte) and system packages;
// on a slow connection that took more than twenty minutes. Finished layers are
// cached, so a build cut short resumes where it stopped.
const bootstrapBuildTimeout = 60 * time.Minute

var bootstrapMu sync.Mutex

// bootstrapTag reports whether ref names the image built from
// scripts/execution-image, as opposed to an image the user supplied.
func bootstrapTag(ref string) bool {
	return strings.HasPrefix(ref, "forgeflow-verification:")
}

// EnsureBootstrapImage makes the configured verification image available. It
// returns built=true when the image was missing and has been rebuilt. An image
// the user supplied (any other name, or an immutable ID) is never built here.
func (c *CLI) EnsureBootstrapImage(ctx context.Context) (built bool, err error) {
	image := c.VerificationImage
	if image == "" {
		return false, nil
	}
	bootstrapMu.Lock()
	defer bootstrapMu.Unlock()
	if _, err = localImage(ctx, image); err == nil {
		return false, nil
	}
	if !errors.Is(err, imageMissing) {
		return false, fmt.Errorf("Docker is not reachable, so the verification image could not be checked. Start Docker Desktop and try again: %w", err)
	}
	if !bootstrapTag(image) || c.BootstrapDir == "" {
		return false, err
	}
	if _, statErr := os.Stat(filepath.Join(c.BootstrapDir, "Dockerfile")); statErr != nil {
		return false, fmt.Errorf("%w; its Dockerfile was not found in %s", err, c.BootstrapDir)
	}
	buildCtx, cancel := context.WithTimeout(ctx, bootstrapBuildTimeout)
	defer cancel()
	out, buildErr := dockerControl(buildCtx, "build", "--tag", image, c.BootstrapDir)
	if buildErr != nil {
		return false, fmt.Errorf("the verification image %s was missing and rebuilding it failed (the build downloads its base images, so it needs network access): %w; %s", image, buildErr, tail(out, 1500))
	}
	if _, err = localImage(ctx, image); err != nil {
		return false, err
	}
	return true, nil
}

// ensureBase rebuilds a missing bootstrap image before a command needs it. Any
// failure is left to the command's own error, which names the image.
func (c *CLI) ensureBase(ctx context.Context, image string) {
	if image == "" || image != c.VerificationImage && image != c.syncImage() {
		return
	}
	c.environmentMu.Lock()
	recent := time.Since(c.baseChecked) < time.Minute
	c.environmentMu.Unlock()
	if recent {
		return
	}
	if _, err := c.EnsureBootstrapImage(ctx); err == nil {
		c.environmentMu.Lock()
		c.baseChecked = time.Now()
		c.environmentMu.Unlock()
	}
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
