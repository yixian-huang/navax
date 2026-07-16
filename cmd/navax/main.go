package main

import (
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/yixian-huang/navax/internal/app"
	"github.com/yixian-huang/navax/internal/config"
)

var (
	version    = "dev"
	commit     = "unknown"
	builtAt    = ""
	deployment = "development"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	buildTime, err := time.Parse(time.RFC3339, builtAt)
	if err != nil {
		buildTime = time.Unix(0, 0).UTC()
	}
	ctx, cancel := app.SignalContext()
	defer cancel()
	if err := app.Run(ctx, cfg, app.BuildInfo{
		Version: version, Commit: commit, BuiltAt: buildTime,
		GoVersion: runtime.Version(), Deployment: deployment,
	}); err != nil {
		slog.Error("nav.ax stopped", "error", err)
		os.Exit(1)
	}
}
