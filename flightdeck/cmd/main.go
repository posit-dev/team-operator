package main

import (
	"log/slog"

	"github.com/posit-dev/team-operator/flightdeck/http"
	"github.com/posit-dev/team-operator/flightdeck/internal"
)

func main() {
	config := internal.NewServerConfig()
	internal.SetupLogger(config)

	slog.Info("starting flightdeck",
		"version", internal.VersionString,
		"site", config.SiteName,
		"port", config.Port,
		"logLevel", config.LogLevel,
		"logFormat", config.LogFormat,
		"devMode", config.DevMode,
	)

	var siteClient internal.SiteInterface
	if config.DevMode {
		slog.Info("running in dev mode with mock data")
		siteClient = internal.NewMockSiteClient()
	} else {
		kubeClient, err := internal.NewKubeClient()
		if err != nil {
			slog.Error("failed to create kube client", "error", err)
			return
		}
		slog.Debug("kube client initialized")
		siteClient = kubeClient.SiteClient
	}

	s := http.NewServer(config, siteClient)

	slog.Info("server listening", "addr", ":"+config.Port)
	if err := s.Start(); err != nil {
		slog.Error("server stopped", "error", err)
	}
}
