package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func Init(level string, jsonFormat bool) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	Log = slog.New(handler)
	slog.SetDefault(Log)
}

func Fatal(msg string, err error) {
	Log.Error(msg, "error", err)
	os.Exit(1)
}