package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	quotacollector "github.com/router-for-me/CLIProxyAPI/v7/internal/quota_collector"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	cfg, errConfig := quotacollector.LoadConfigFromEnv()
	if errConfig != nil {
		log.WithError(errConfig).Error("failed to load quota collector config")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, errStore := quotacollector.NewStore(ctx, cfg.Database)
	if errStore != nil {
		log.WithError(errStore).Error("failed to initialize quota collector store")
		os.Exit(1)
	}
	defer store.Close()

	collector := quotacollector.NewCollector(cfg, store, quotacollector.NewClient())
	if errRun := collector.Run(ctx); errRun != nil {
		log.WithError(errRun).Error("quota collector stopped")
		os.Exit(1)
	}
}
