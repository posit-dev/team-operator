package internal

import (
	"log/slog"
	"os"
	"strings"

	"k8s.io/utils/env"
)

type ServerConfig struct {
	SiteName    string `json:"site_name"`
	Port        string `json:"port"`
	ShowAcademy bool   `json:"show_academy"`
	ShowConfig  bool   `json:"show_config"`
	LogLevel    string `json:"log_level"`
	LogFormat   string `json:"log_format"`
	DevMode     bool   `json:"dev_mode"`
	Namespace   string `json:"namespace"`
}

func NewServerConfig() *ServerConfig {
	s := &ServerConfig{
		SiteName:  env.GetString("SITE_NAME", "main"),
		Port:      env.GetString("PORT", "8080"),
		LogLevel:  env.GetString("LOG_LEVEL", "info"),
		LogFormat: env.GetString("LOG_FORMAT", "text"),
		Namespace: env.GetString("NAMESPACE", "posit-team"),
	}
	s.ShowAcademy, _ = env.GetBool("SHOW_ACADEMY", false)
	s.ShowConfig, _ = env.GetBool("SHOW_CONFIG", false)
	s.DevMode, _ = env.GetBool("DEV_MODE", false)
	return s
}

// SetupLogger configures the global slog logger based on config
func SetupLogger(config *ServerConfig) {
	var level slog.Level
	switch strings.ToLower(config.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(config.LogFormat) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
