package main

import (
	"context"
	"flag"
	"fmt"
	"forgeflow/internal/api"
	"forgeflow/internal/codex"
	"forgeflow/internal/contributions"
	"forgeflow/internal/discovery"
	"forgeflow/internal/domain"
	"forgeflow/internal/execution"
	"forgeflow/internal/github"
	"forgeflow/internal/githubauth"
	"forgeflow/internal/project"
	"forgeflow/internal/seed"
	"forgeflow/internal/storage"
	"forgeflow/internal/workspace"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("ForgeFlow stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	demo := flag.Bool("demo", false, "use explicit demo mode and illustrative seed data")
	rootFlag := flag.String("root", "", "ForgeFlow project directory (default: locate from working directory or executable)")
	contributionRoot := flag.String("contributions-dir", "", "external workspace root (overrides local settings/environment)")
	db := flag.String("db", "", "SQLite path (default depends on mode)")
	listen := flag.String("listen", "127.0.0.1:8080", "loopback address")
	web := flag.String("web", "apps/web/dist", "built frontend directory")
	discoveryInterval := flag.Duration("discovery-interval", 15*time.Minute, "automatic GitHub scan interval (0 disables automatic scans)")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	root, err := project.Locate(*rootFlag, cwd, executable)
	if err != nil {
		return err
	}
	settings, err := project.LoadSettings(root)
	if err != nil {
		return err
	}
	if *contributionRoot != "" {
		settings.ContributionsDir = project.Resolve(root, *contributionRoot)
	}
	if stringsEqualPath(settings.ContributionsDir, root) {
		return fmt.Errorf("contribution root must not be the ForgeFlow application root")
	}
	if *discoveryInterval < 0 || (*discoveryInterval > 0 && *discoveryInterval < time.Minute) {
		return fmt.Errorf("discovery interval must be zero or at least one minute")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("server must bind a numeric loopback address")
	}
	if *db == "" {
		*db = "data/forgeflow.db"
		if *demo {
			*db = "data/demo.db"
		}
	}
	if *db != ":memory:" {
		*db = project.Resolve(root, *db)
	}
	*web = project.Resolve(root, *web)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := storage.Open(ctx, *db, *demo)
	if err != nil {
		return err
	}
	defer s.Close()
	if !*demo {
		cs, e := s.Contributions(ctx)
		if e != nil {
			return e
		}
		for _, c := range cs {
			if c.Demo || c.Workspace == "" || filepath.IsAbs(filepath.FromSlash(c.Workspace)) {
				continue
			}
			old := project.Resolve(root, filepath.FromSlash(c.Workspace))
			target, e := contributions.WorkspacePathAt(settings.ContributionsDir, c.ID)
			if e != nil {
				return e
			}
			if stringsEqualPath(old, target) {
				continue
			}
			if _, e = os.Stat(old); e == nil {
				return fmt.Errorf("existing contribution %s must be moved to configured root before starting", c.ID)
			}
			if info, e := os.Stat(target); e == nil && info.IsDir() {
				if e = s.RelocateWorkspace(ctx, c.ID, filepath.ToSlash(target)); e != nil {
					return e
				}
			}
		}
	}
	if !*demo {
		if err = s.RecoverDiscovery(ctx); err != nil {
			return err
		}
		if err = s.RecoverWorkspaces(ctx); err != nil {
			return err
		}
		if err = s.RecoverExecutions(ctx); err != nil {
			return err
		}
	}
	if *demo {
		if err = seed.Load(ctx, s); err != nil {
			return err
		}
	}
	webDir := ""
	if info, e := os.Stat(*web); e == nil && info.IsDir() {
		webDir = *web
	}
	token, auth := "", "disabled in demo"
	if !*demo {
		token, auth = github.ResolveToken(ctx)
	}
	client := github.New(token)
	var account *githubauth.Service
	if !*demo {
		account, err = githubauth.New(ctx, root, client, auth)
		if err != nil {
			return err
		}
		defer account.Close()
		auth = account.Status().Source
	}
	workspaces := workspace.New(ctx, s, client, root)
	workspaces.Base = settings.ContributionsDir
	workspaces.DefaultModel = settings.CodexModel
	cli := codex.Resolve(settings.CodexBinary, settings.CodexModel)
	var operatorAI codex.Runner
	if !*demo && cli.Binary != "" {
		operatorAI = cli
	}
	executor := execution.New(ctx, s, cli, client, root, settings.ContributionsDir)
	workspaces.Prepared = func(startCtx context.Context, c domain.Contribution) error {
		_, e := executor.Start(startCtx, c.ID, true, false)
		return e
	}
	defer func() { cancel(); executor.Wait() }()
	defer func() { cancel(); workspaces.Wait() }()
	discover := discovery.New(ctx, s, client, auth, *discoveryInterval)
	if err = discover.RestoreSchedule(); err != nil {
		return err
	}
	if account != nil {
		account.SetChanged(func(source string) {
			discover.CredentialChanged(source)
			if e := s.WorkspaceEvent(ctx, "github-account", "GitHubAuthenticationChanged", "GitHub access updated", map[string]string{"source": source}); e != nil {
				slog.Error("GitHub account audit event failed", "error", e)
			}
		})
	}
	discover.Schedule()
	if err = s.ExpireConfigs(ctx, time.Now()); err != nil {
		return err
	}
	expiryDone := make(chan struct{})
	go func() {
		defer close(expiryDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if e := s.ExpireConfigs(ctx, time.Now()); e != nil {
					slog.Error("temporary config expiration failed", "error", e)
				}
			}
		}
	}()
	defer func() { cancel(); <-expiryDone }()
	defer func() { cancel(); discover.Cancel(); discover.Wait() }()
	server := &http.Server{Addr: *listen, Handler: (api.Server{Auth: account, Store: s, WebDir: webDir, Discovery: discover, Workspaces: workspaces, Execution: executor, OperatorAI: operatorAI, Runtime: map[string]any{"contributions_root": settings.ContributionsDir, "codex_available": cli.Binary != "", "codex_model": settings.CodexModel, "mode": map[bool]string{true: "demo", false: "live"}[*demo]}}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan error, 1)
	go func() {
		slog.Info("ForgeFlow listening", "address", *listen, "demo", *demo)
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		return server.Shutdown(shutdownCtx)
	}
}
func stringsEqualPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
