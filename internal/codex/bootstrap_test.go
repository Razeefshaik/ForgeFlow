package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnlyTheBootstrapImageIsRebuilt(t *testing.T) {
	for ref, want := range map[string]bool{BootstrapImage: true, "forgeflow-verification:custom": true, "ubuntu:24.04": false, "sha256:" + strings.Repeat("a", 64): false, "": false} {
		if bootstrapTag(ref) != want {
			t.Errorf("%q: bootstrapTag=%v", ref, !want)
		}
	}
	// Without an image configured there is nothing to ensure, and no Docker call.
	if built, err := (&CLI{}).EnsureBootstrapImage(context.Background()); built || err != nil {
		t.Fatal(built, err)
	}
}

// TestRealBootstrapImageRebuild removes nothing: it asks Docker for the
// configured image and rebuilds it only when it is missing, exactly as the
// server does at startup. Opt in with FORGEFLOW_REAL_DOCKER_TEST=1.
func TestRealBootstrapImageRebuild(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_DOCKER_TEST") != "1" {
		t.Skip("opt in to real Docker")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	c := &CLI{VerificationImage: BootstrapImage, BootstrapDir: filepath.Join(root, "scripts", "execution-image")}
	start := time.Now()
	built, err := c.EnsureBootstrapImage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("built=%v in %s", built, time.Since(start).Round(time.Second))
	if _, err = localImage(context.Background(), BootstrapImage); err != nil {
		t.Fatal(err)
	}
	if again, err := c.EnsureBootstrapImage(context.Background()); again || err != nil {
		t.Fatalf("an available image was rebuilt: %v %v", again, err)
	}
}
