package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/config"
	"github.com/DizzyZ7/SignalBox/internal/delivery"
	"github.com/DizzyZ7/SignalBox/internal/httpapi"
	"github.com/DizzyZ7/SignalBox/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := storage.Connect(appCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	repo := storage.New(pool)
	if cfg.AutoMigrate {
		if err := repo.Migrate(appCtx); err != nil {
			logger.Error("migration failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	notifier := delivery.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramDefaultChatID, repo, logger, cfg.DeliveryMaxAttempts)
	if cfg.DeliveryWorkerEnabled {
		notifier.Start(appCtx, cfg.DeliveryWorkerInterval, cfg.DeliveryWorkerBatchSize, cfg.DeliveryWorkerLockDuration)
	}

	api := httpapi.NewServer(cfg, repo, notifier, logger)
	handler := httpapi.WithMetrics(httpapi.WithAdminUI(api.Handler()), repo)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       time.Minute,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.HTTPAddr))
		errs <- server.ListenAndServe()
	}()

	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case <-appCtx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
		_ = server.Close()
	}
}
