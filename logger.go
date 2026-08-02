package main

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

// custom severity ordering, NOT slog's default (debug < info < warn < error) -- here info is
// the most verbose tier and debug sits between info and warn, matching the project-wide
// logging-management convention (see every Node service's src/logger.ts for the same scheme):
//
//	LOG_LEVEL=info  -> shows info + debug + warn + error (the default, most verbose)
//	LOG_LEVEL=debug -> shows debug + warn + error (skips info)
//	LOG_LEVEL=error -> shows error only
//
// implemented as an explicit allow-list per selected tier (filteringHandler.Enabled) rather than
// a numeric threshold, since a plain gte-threshold can't skip a middle tier (info) while still
// showing both its neighbors (debug, warn).
var currentLogLevel atomic.Value // stores a string: "info" | "debug" | "error"

type filteringHandler struct {
	slog.Handler
}

func (h *filteringHandler) Enabled(_ context.Context, level slog.Level) bool {
	switch currentLogLevelName() {
	case "error":
		return level >= slog.LevelError
	case "debug":
		return level == slog.LevelDebug || level >= slog.LevelWarn
	default: // "info"
		return true
	}
}

func normalizeLevelName(name string) string {
	switch name {
	case "debug", "error":
		return name
	default:
		return "info"
	}
}

// initLogger sets a JSON structured logger as the default, level controlled by LOG_LEVEL at
// boot and mutable afterward at runtime via setLogLevel (see POST /internal/log-level in
// main.go) -- no redeploy needed to change verbosity.
func initLogger() {
	currentLogLevel.Store(normalizeLevelName(os.Getenv("LOG_LEVEL")))
	// the wrapped handler's own Level is left at the most permissive setting -- filteringHandler
	// is what actually gatekeeps, checked per named tier above, not a numeric threshold here
	handler := &filteringHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})}
	slog.SetDefault(slog.New(handler))
}

func setLogLevel(name string) string {
	normalized := normalizeLevelName(name)
	currentLogLevel.Store(normalized)
	return normalized
}

func currentLogLevelName() string {
	selected, _ := currentLogLevel.Load().(string)
	if selected == "" {
		return "info"
	}
	return selected
}
