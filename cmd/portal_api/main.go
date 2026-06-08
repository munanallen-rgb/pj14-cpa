package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	cpadashboard "github.com/router-for-me/CLIProxyAPI/v7/internal/cpa_dashboard"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/portal"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	cfg, errConfig := portal.LoadConfigFromEnv()
	if errConfig != nil {
		log.WithError(errConfig).Error("failed to load portal config")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, errStore := portal.NewStore(ctx, cfg.Database)
	if errStore != nil {
		log.WithError(errStore).Error("failed to initialize portal store")
		os.Exit(1)
	}
	defer store.Close()

	if errMigrate := store.Migrate(ctx); errMigrate != nil {
		log.WithError(errMigrate).Error("failed to migrate portal store")
		os.Exit(1)
	}

	sub2apiClient := portal.NewSub2APIClient(cfg.Sub2API)
	service := portal.NewService(cfg, store, sub2apiClient)

	dashboardStore, errDashboardStore := cpadashboard.NewStore(ctx, cpadashboard.DatabaseConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Name:     cfg.Database.Name,
		SSLMode:  cfg.Database.SSLMode,
	})
	if errDashboardStore != nil {
		log.WithError(errDashboardStore).Error("failed to initialize portal admin dashboard store")
		os.Exit(1)
	}
	defer dashboardStore.Close()
	service.SetDashboardService(cpadashboard.NewService(dashboardStore))

	if errBootstrap := service.BootstrapAdmin(ctx); errBootstrap != nil {
		log.WithError(errBootstrap).Error("failed to bootstrap portal admin")
		os.Exit(1)
	}

	server := portal.NewServer(cfg, service)
	httpServer := &http.Server{
		Addr:    cfg.BindAddress,
		Handler: server.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		log.WithField("bind", cfg.BindAddress).Info("portal api started")
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), portal.ShutdownTimeout)
		defer cancel()
		if errShutdown := httpServer.Shutdown(shutdownCtx); errShutdown != nil {
			log.WithError(errShutdown).Error("failed to shut down portal api")
			os.Exit(1)
		}
	case errServe := <-errCh:
		if errServe != nil && errServe != http.ErrServerClosed {
			log.WithError(errServe).Error("portal api stopped")
			os.Exit(1)
		}
	}
}
