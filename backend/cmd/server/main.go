package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"bws-checkin/backend/internal/app"
	"bws-checkin/backend/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	handler, cleanup, err := app.New(cfg)
	if err != nil {
		logger.Error("server_init_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer cleanup()

	logger.Info("server_starting",
		slog.String("addr", cfg.Addr),
		slog.String("db_path", cfg.DBPath),
		slog.String("upload_dir", cfg.UploadDir),
		slog.Bool("dev_auth", cfg.DevAuth),
		slog.Bool("cookie_secure", cfg.CookieSecure),
		slog.String("cookie_same_site", cfg.CookieSameSite),
	)

	if err := http.ListenAndServe(cfg.Addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server_stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
