package main

import (
	"context"
	"log/slog"
	"testing"
)

func TestFilteringHandlerInfoShowsEverything(t *testing.T) {
	currentLogLevel.Store("info")
	h := &filteringHandler{}
	for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		if !h.Enabled(context.Background(), lvl) {
			t.Errorf("level %v should be visible at LOG_LEVEL=info", lvl)
		}
	}
}

func TestFilteringHandlerDebugSkipsInfo(t *testing.T) {
	currentLogLevel.Store("debug")
	h := &filteringHandler{}

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be hidden at LOG_LEVEL=debug")
	}
	for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelWarn, slog.LevelError} {
		if !h.Enabled(context.Background(), lvl) {
			t.Errorf("level %v should be visible at LOG_LEVEL=debug", lvl)
		}
	}
}

func TestFilteringHandlerErrorShowsOnlyError(t *testing.T) {
	currentLogLevel.Store("error")
	h := &filteringHandler{}

	for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn} {
		if h.Enabled(context.Background(), lvl) {
			t.Errorf("level %v should be hidden at LOG_LEVEL=error", lvl)
		}
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("error should be visible at LOG_LEVEL=error")
	}
}

func TestNormalizeLevelNameRejectsUnknown(t *testing.T) {
	if got := normalizeLevelName("verbose"); got != "info" {
		t.Errorf("expected unknown level names to fall back to info, got %q", got)
	}
}

func TestSetLogLevelUpdatesCurrent(t *testing.T) {
	setLogLevel("debug")
	if got := currentLogLevelName(); got != "debug" {
		t.Errorf("expected debug, got %q", got)
	}
	setLogLevel("info")
}
