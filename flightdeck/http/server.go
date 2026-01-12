package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/posit-dev/team-operator/flightdeck/html"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "maragu.dev/gomponents"
	ghttp "maragu.dev/gomponents/http"
)

type Server struct {
	config     *internal.ServerConfig
	siteClient internal.SiteInterface
	mux        *http.ServeMux
	server     *http.Server
}

func NewServer(serverConfig *internal.ServerConfig, siteClient internal.SiteInterface) *Server {
	return &Server{
		config:     serverConfig,
		siteClient: siteClient,
		mux:        http.NewServeMux(),
		server:     &http.Server{Addr: fmt.Sprintf(":%s", serverConfig.Port)},
	}
}

func (s *Server) setupRoutes() {
	Static(s.mux)
	Home(s.mux, s.siteClient, s.config)
	Config(s.mux, s.siteClient, s.config)
	Help(s.mux, s.config)
}

func (s *Server) Start() error {
	s.setupRoutes()
	s.server.Handler = requestLoggingMiddleware(s.mux)
	return s.server.ListenAndServe()
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// requestLoggingMiddleware logs all incoming HTTP requests
func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Skip logging for static assets at debug level
		if len(r.URL.Path) > 7 && r.URL.Path[:8] == "/static/" {
			slog.Debug("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", duration,
			)
		} else {
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", duration,
				"remoteAddr", r.RemoteAddr,
			)
		}
	})
}

func Home(mux *http.ServeMux, getter internal.SiteInterface, config *internal.ServerConfig) {
	mux.Handle("GET /", ghttp.Adapt(func(w http.ResponseWriter, r *http.Request) (Node, error) {
		site, err := getter.Get(config.SiteName, config.Namespace, metav1.GetOptions{}, r.Context())
		if err != nil {
			slog.Error("failed to get site for home page",
				"site", config.SiteName,
				"error", err,
			)
			return nil, err
		}
		slog.Debug("rendering home page", "site", config.SiteName)
		return html.HomePage(*site, config), nil
	}))
}

func Help(mux *http.ServeMux, config *internal.ServerConfig) {
	mux.Handle("GET /help", ghttp.Adapt(func(w http.ResponseWriter, r *http.Request) (Node, error) {
		slog.Debug("rendering help page")
		return html.HelpPage(config), nil
	}))
}

func Config(mux *http.ServeMux, getter internal.SiteInterface, config *internal.ServerConfig) {
	mux.Handle("GET /config", ghttp.Adapt(func(w http.ResponseWriter, r *http.Request) (Node, error) {
		site, err := getter.Get(config.SiteName, config.Namespace, metav1.GetOptions{}, r.Context())
		if err != nil {
			slog.Error("failed to get site for config page",
				"site", config.SiteName,
				"error", err,
			)
			return nil, err
		}
		slog.Debug("rendering config page", "site", config.SiteName)
		return html.SiteConfigPage(site, config), nil
	}))
}

func Static(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public"))))
}
