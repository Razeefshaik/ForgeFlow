package main

import (
	"context"
	"flag"
	"fmt"
	"forgeflow/internal/api"
	"forgeflow/internal/discovery"
	"forgeflow/internal/github"
	"forgeflow/internal/seed"
	"forgeflow/internal/storage"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	db := flag.String("db", "", "SQLite path (default depends on mode)")
	listen := flag.String("listen", "127.0.0.1:8080", "loopback address")
	web := flag.String("web", "apps/web/dist", "built frontend directory")
	discoveryInterval := flag.Duration("discovery-interval", 15*time.Minute, "automatic GitHub scan interval (0 disables automatic scans)")
	flag.Parse()
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := storage.Open(ctx, *db, *demo)
	if err != nil {
		return err
	}
	defer s.Close()
	if !*demo {
		if err = s.RecoverDiscovery(ctx); err != nil {
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
	discover := discovery.New(ctx, s, github.New(token), auth, *discoveryInterval)
	if err = discover.RestoreSchedule(); err != nil {
		return err
	}
	discover.Schedule()
	defer func() { cancel(); discover.Cancel(); discover.Wait() }()
	server := &http.Server{Addr: *listen, Handler: (api.Server{Store: s, WebDir: webDir, Discovery: discover}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
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
