package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/krteke/River/internal/config"
	filesystem "github.com/krteke/River/internal/fs"
)

func main() {
	configPath := flag.String("c", "config/config.toml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	server, err := filesystem.NewService(cfg.Roots)
	if err != nil {
		logger.Error("failed to init filesystem service", "error", err)
		os.Exit(1)
	}

}
