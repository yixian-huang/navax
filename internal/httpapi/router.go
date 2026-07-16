package httpapi

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type VersionInfo struct {
	Version    string    `json:"version"`
	Commit     string    `json:"commit"`
	BuiltAt    time.Time `json:"builtAt"`
	GoVersion  string    `json:"goVersion"`
	Deployment string    `json:"deployment"`
}

type RouterOptions struct {
	Version        VersionInfo
	PublicBaseURL  string
	TrustedProxies []netip.Prefix
	Ready          func(context.Context) error
	MountAPI       func(chi.Router)
	Web            http.Handler
}

func NewRouter(options RouterOptions) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(RealIP(options.TrustedProxies))
	router.Use(AbuseProtection())
	router.Use(accessLog)
	router.Use(recoverer)
	router.Use(SecurityHeaders(strings.HasPrefix(options.PublicBaseURL, "https://")))
	router.Use(middleware.Compress(5, "application/json", "text/html", "text/css", "application/javascript"))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteRawJSON(w, r, http.StatusOK, map[string]any{"status": "healthy"})
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if options.Ready != nil {
			if err := options.Ready(r.Context()); err != nil {
				WriteRawJSON(w, r, http.StatusServiceUnavailable, map[string]any{
					"status": "degraded",
					"checks": map[string]string{"database": "unavailable"},
				})
				return
			}
		}
		WriteRawJSON(w, r, http.StatusOK, map[string]any{"status": "healthy"})
	})

	router.Route("/api/v1", func(api chi.Router) {
		api.Use(VerifyOrigin(options.PublicBaseURL))
		api.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			WriteJSON(w, r, http.StatusOK, options.Version)
		})
		if options.MountAPI != nil {
			options.MountAPI(api)
		}
		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "API 资源不存在", nil)
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "API 请求方法不受支持", nil)
		})
	})

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if options.Web != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			options.Web.ServeHTTP(w, r)
			return
		}
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "资源不存在", nil)
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不受支持", nil)
	})
	return router
}
