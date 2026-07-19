package main

import (
	"log/slog"
	"os"
)

// initLogger sets a JSON structured logger as the default, level configurable
// via LOG_LEVEL (debug/info/warn/error) so debug-level noise stays off in prod
// unless explicitly turned on.
func initLogger() {
	level := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
