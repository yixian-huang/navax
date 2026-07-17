package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	adminpkg "github.com/yixian-huang/navax/internal/admin"
	"github.com/yixian-huang/navax/internal/analytics"
	"github.com/yixian-huang/navax/internal/assets"
	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/config"
	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/dataexchange"
	"github.com/yixian-huang/navax/internal/directoryadmin"
	"github.com/yixian-huang/navax/internal/httpapi"
	"github.com/yixian-huang/navax/internal/idempotency"
	"github.com/yixian-huang/navax/internal/integrations"
	"github.com/yixian-huang/navax/internal/linkcheck"
	"github.com/yixian-huang/navax/internal/maintenance"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/security"
	"github.com/yixian-huang/navax/internal/subdomains"
	"github.com/yixian-huang/navax/internal/webui"
)

type BuildInfo struct {
	Version    string
	Commit     string
	BuiltAt    time.Time
	GoVersion  string
	Deployment string
}

func Run(ctx context.Context, cfg config.Config, build BuildInfo) error {
	runContext, requestStop := context.WithCancel(ctx)
	defer requestStop()
	startedAt := time.Now()
	restoreSwap, err := maintenance.ApplyPendingRestore(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("apply pending database restore: %w", err)
	}
	if restoreSwap.Applied() && strings.TrimSpace(os.Getenv("NAVAX_MASTER_KEY")) == "" {
		cfg.MasterKey, err = security.LoadOrCreateKey(filepath.Join(cfg.DataDir, "master.key"), 32)
		if err != nil {
			_ = restoreSwap.Rollback()
			return fmt.Errorf("reload restored instance master key: %w", err)
		}
	}
	db, err := database.OpenAndMigrate(runContext, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		_ = restoreSwap.Rollback()
		return fmt.Errorf("initialize database: %w", err)
	}
	if err := restoreSwap.Commit(); err != nil {
		_ = db.Close()
		return fmt.Errorf("finalize database restore: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	authStore := auth.NewSQLStore(db)
	authService := auth.NewService(authStore, cfg.SetupToken, cfg.SessionTTL)
	initialized, err := authService.Initialized(ctx)
	if err != nil {
		return fmt.Errorf("read initialization status: %w", err)
	}
	if !initialized {
		slog.Warn("instance setup is required", "setup_token", cfg.SetupToken, "endpoint", "/api/v1/bootstrap")
	}
	adminService := adminpkg.NewService(adminpkg.NewSQLStore(db))
	providerService, err := integrations.NewService(db, cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("initialize provider configuration: %w", err)
	}
	authHandler := httpapi.NewAuthHandler(authService, httpapi.AuthHandlerOptions{
		SecureCookies: cfg.SecureCookies,
		InstanceName:  cfg.InstanceName,
		PublicBaseURL: cfg.PublicBaseURL,
		Version:       build.Version,
		Mailer:        providerService,
	})
	accountHandler := httpapi.NewAccountHandler(authService)
	adminHandler := httpapi.NewAdminHandler(authService, adminService, httpapi.AdminHandlerOptions{
		Version: build.Version, StartedAt: startedAt, InstanceName: cfg.InstanceName, Mailer: providerService,
	})
	providerHandler := httpapi.NewProviderHandler(providerService)
	backupService, err := maintenance.NewBackupService(db, cfg.DataDir+"/backups")
	if err != nil {
		return fmt.Errorf("initialize backups: %w", err)
	}
	if err := backupService.ConfigureRestore(cfg.DatabasePath); err != nil {
		return fmt.Errorf("configure backup restore: %w", err)
	}
	backupHandler := httpapi.NewBackupHandler(backupService, httpapi.BackupHandlerOptions{
		Auth: authService, RequestRestart: requestStop,
	})
	updateService := maintenance.NewUpdateService(db, build.Version, build.Deployment, cfg.UpdateManifestURL, cfg.UpdatePublicKey)
	updateService.AttachBackups(backupService)
	if err := updateService.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize update state: %w", err)
	}
	idempotencyService := idempotency.NewService(db)
	updateHandler := httpapi.NewUpdateHandler(updateService, httpapi.UpdateHandlerOptions{
		Idempotency: idempotencyService, RequestRestart: requestStop,
	})
	navigationService := navigation.NewService(navigation.NewSQLStore(db))
	navigationHandler := httpapi.NewNavigationHandler(navigationService, cfg.PublicBaseURL, httpapi.NavigationHandlerOptions{
		Idempotency: idempotencyService,
	})
	dataExchangeHandler := httpapi.NewDataExchangeHandler(dataexchange.NewService(db))
	linkCheckHandler := httpapi.NewLinkCheckHandler(linkcheck.NewService(db))
	directoryAdminHandler := httpapi.NewDirectoryAdminHandler(authService, directoryadmin.NewService(directoryadmin.NewSQLStore(db)))
	catalogHandler := httpapi.NewCatalogHandler(catalog.NewService(db))
	analyticsKey, err := security.LoadOrCreateKey(filepath.Join(cfg.DataDir, "analytics.key"), 32)
	if err != nil {
		return fmt.Errorf("initialize analytics privacy key: %w", err)
	}
	analyticsService, err := analytics.NewService(db, analyticsKey)
	if err != nil {
		return fmt.Errorf("initialize analytics: %w", err)
	}
	analyticsHandler := httpapi.NewAnalyticsHandler(analyticsService)
	assetService, err := assets.NewService(db, filepath.Join(cfg.DataDir, "assets"))
	if err != nil {
		return fmt.Errorf("initialize asset storage: %w", err)
	}
	assetService.SetStorageResolver(func(ctx context.Context) (*assets.S3Config, error) {
		endpoint, region, bucket, prefix, accessKey, secretKey, publicBaseURL, pathStyle, ok, resolveErr := providerService.ActiveS3Config(ctx)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if !ok {
			return nil, nil
		}
		return &assets.S3Config{
			Endpoint: endpoint, Region: region, Bucket: bucket, Prefix: prefix,
			AccessKey: accessKey, SecretKey: secretKey, PathStyle: pathStyle, PublicBaseURL: publicBaseURL,
		}, nil
	})
	assetHandler := httpapi.NewAssetHandler(assetService)
	subdomainService := subdomains.NewService(subdomains.NewSQLStore(db))
	subdomainHandler := httpapi.NewSubdomainHandler(authService, subdomainService)
	webHandler, err := webui.New(webui.Options{ResolveSEO: func(r *http.Request) (webui.SEO, error) {
		var page navigation.PublishedPage
		var resolveErr error
		if strings.HasPrefix(r.URL.Path, "/u/") {
			page, resolveErr = navigationService.PublicBySlug(r.Context(), strings.TrimPrefix(r.URL.Path, "/u/"))
		} else if r.URL.Path == "/" {
			host := r.Host
			if value, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
				host = value
			}
			page, resolveErr = navigationService.PublicHomeForHost(r.Context(), host)
		} else {
			return webui.SEO{}, nil
		}
		if resolveErr != nil {
			return webui.SEO{}, resolveErr
		}
		canonical := cfg.PublicBaseURL + r.URL.Path
		if page.Subdomain != nil && r.URL.Path == "/" {
			scheme := "https://"
			if strings.HasPrefix(cfg.PublicBaseURL, "http://") {
				scheme = "http://"
			}
			canonical = scheme + *page.Subdomain + "/"
		}
		robots := "noindex,follow"
		if page.Visibility == navigation.VisibilityPublic || page.Kind == navigation.PageKindSystem {
			robots = "index,follow"
		}
		title := page.Title
		if strings.TrimSpace(page.SEOTitle) != "" {
			title = page.SEOTitle
		}
		description := page.Description
		if strings.TrimSpace(page.SEODescription) != "" {
			description = page.SEODescription
		}
		image := page.OGImage
		if image != "" && strings.HasPrefix(image, "/") {
			image = strings.TrimRight(cfg.PublicBaseURL, "/") + image
		}
		return webui.SEO{Title: title, Description: description, Canonical: canonical, Robots: robots, Image: image}, nil
	}})
	if err != nil {
		return fmt.Errorf("initialize embedded frontend: %w", err)
	}

	handler := httpapi.NewRouter(httpapi.RouterOptions{
		Version: httpapi.VersionInfo{
			Version: build.Version, Commit: build.Commit, BuiltAt: build.BuiltAt,
			GoVersion: build.GoVersion, Deployment: build.Deployment,
		},
		PublicBaseURL:  cfg.PublicBaseURL,
		TrustedProxies: cfg.TrustedProxies,
		Ready:          db.PingContext,
		Web:            webHandler,
		MountAPI: func(router chi.Router) {
			authHandler.Mount(router)
			accountHandler.Mount(router)
			navigationHandler.MountPublic(router)
			catalogHandler.Mount(router)
			analyticsHandler.MountPublic(router)
			assetHandler.MountPublic(router)
			router.Group(func(protected chi.Router) {
				protected.Use(httpapi.RequireSession(authService))
				navigationHandler.MountProtected(protected)
				dataExchangeHandler.MountProtected(protected)
				linkCheckHandler.MountProtected(protected)
				analyticsHandler.MountProtected(protected)
				assetHandler.MountProtected(protected)
				subdomainHandler.MountUserRoutes(protected)
				protected.Route("/admin", func(admin chi.Router) {
					admin.Use(httpapi.RequireAdmin)
					adminHandler.MountRoutes(admin)
					directoryAdminHandler.MountRoutes(admin)
					providerHandler.Mount(admin)
					backupHandler.Mount(admin)
					updateHandler.Mount(admin)
					subdomainHandler.MountAdminRoutes(admin)
				})
			})
		},
	})

	server := &http.Server{
		Addr: cfg.Addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go analyticsService.RunRetention(runContext)
	go maintenance.NewUpdateSupervisor(updateService, requestStop).Run(runContext)
	go func() {
		slog.Info("nav.ax server started", "addr", cfg.Addr, "public_base_url", cfg.PublicBaseURL, "version", build.Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
		close(serverErrors)
	}()

	select {
	case <-runContext.Done():
	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		_ = server.Close()
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	return nil
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
