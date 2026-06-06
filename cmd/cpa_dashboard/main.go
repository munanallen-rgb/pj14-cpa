package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	cpadashboard "github.com/router-for-me/CLIProxyAPI/v7/internal/cpa_dashboard"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	cfg, errConfig := cpadashboard.LoadConfigFromEnv()
	if errConfig != nil {
		log.WithError(errConfig).Error("failed to load dashboard config")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, errStore := cpadashboard.NewStore(ctx, cfg.Database)
	if errStore != nil {
		log.WithError(errStore).Error("failed to initialize dashboard store")
		os.Exit(1)
	}
	defer store.Close()

	server := cpadashboard.NewServer(cfg, store)
	httpServer := &http.Server{
		Addr:    cfg.BindAddress,
		Handler: server.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		log.WithField("bind", cfg.BindAddress).Info("cpa dashboard started")
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cpadashboard.ShutdownTimeout)
		defer cancel()
		if errShutdown := httpServer.Shutdown(shutdownCtx); errShutdown != nil {
			log.WithError(errShutdown).Error("failed to shut down cpa dashboard")
			os.Exit(1)
		}
	case errServe := <-errCh:
		if errServe != nil && errServe != http.ErrServerClosed {
			log.WithError(errServe).Error("cpa dashboard stopped")
			os.Exit(1)
		}
	}
}
