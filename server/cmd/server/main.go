package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/krteke/River/internal/api"
	"github.com/krteke/River/internal/config"
	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/media"
	"github.com/krteke/River/internal/thumbnail"
	"github.com/krteke/River/internal/transcode"
)

func main() {
	configPath := flag.String("c", "configs/config.toml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	fileService, err := filesystem.NewService(cfg.Roots)
	if err != nil {
		slog.Error("failed to init filesystem service", "error", err)
		os.Exit(1)
	}
	for _, root := range fileService.Roots() {
		slog.Info("root config", "id", root.ID, "name", root.Name, "path", root.RealPath)
	}

	mediaService := media.NewService(cfg.FFmpeg.FFprobePath, cfg.FFmpeg.FFmpegPath, cfg.Playback)
	thumbnailService := thumbnail.NewService(cfg.FFmpeg.FFmpegPath, cfg.Thumbnail)
	if err := thumbnailService.Cleanup(); err != nil {
		logger.Warn("thumbnail cache cleanup failed", "error", err)
	}

	transcodeManager := transcode.NewManager(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	transcodeManager.StartCleanupLoop(ctx)

	apiServer := api.NewServer(fileService, mediaService, thumbnailService, transcodeManager, cfg.Server.Password)
	httpServer := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("server start", "listen", cfg.Server.Listen)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		transcodeManager.StopAll()
		os.Exit(1)
	}
	transcodeManager.StopAll()
}
