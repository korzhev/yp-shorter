package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/snowflake"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"
	"github.com/korzhev/yp-shorter/internal/config"
	configdb "github.com/korzhev/yp-shorter/internal/config/db"
	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/middleware"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

// newServerListener opens the application listener, using TLS only when enabled.
func newServerListener(c config.Config) (net.Listener, error) {
	if !c.EnableHTTPS {
		return net.Listen("tcp", c.RunAddr)
	}

	certificate, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load HTTPS certificate and key: %w", err)
	}
	return tls.Listen("tcp", c.RunAddr, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	})
}

func RootRouter(c config.Config, db *repository.ShortLinkDB, s *service.Semaphore, node *snowflake.Node, ap *service.AuditPublisher) chi.Router {
	var ShortLinkHandler = handler.ShortLinkHandler{
		ShortLinkService: service.ShortLinkService{
			Charset:     c.ShortLinkCharset,
			IDLength:    c.ShortLinkLength,
			ShortLinkDB: db,
			DBSemaphore: s,
		},
		Audit: ap,
	}
	var PingHandler = handler.PingHandler{
		Pg: db.DB,
	}

	r := chi.NewRouter()

	r.Use(middleware.NewLoggerMiddleware(logger.Log))
	r.Use(middleware.NewAuthMiddleware(c, node))
	r.Use(middleware.NewCompressorMiddleware())
	r.Use(chiMW.RedirectSlashes)
	r.Use(chiMW.Recoverer)

	r.Get("/{id}", ShortLinkHandler.GetByIDLinkHandlerFunc)
	r.Post("/", ShortLinkHandler.SaveLinkHandlerFunc)
	r.Post("/api/shorten", ShortLinkHandler.APISaveLinkHandlerFunc)
	r.Post("/api/shorten/batch", ShortLinkHandler.APISaveLinkBatchHandlerFunc)
	r.Get("/api/user/urls", ShortLinkHandler.APIGetLinksByUserIDHandlerFunc)
	r.Delete("/api/user/urls", ShortLinkHandler.APIDeleteLinkBatchHandlerFunc)
	r.Get("/ping", PingHandler.PingHandlerFunc)
	return r
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)

	err := config.ParseFlags()
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
		return
	}
	config.Conf.DetectStorageType()
	errl := logger.InitLogger(config.Conf.LogLevel)
	if errl != nil {
		fmt.Printf("err: %v", errl)
	}
	defer func() { _ = logger.Log.Sync() }()

	logger.Log.Infow("Server starting with params",
		"address", config.Conf.RunAddr,
		"enableHTTPS", config.Conf.EnableHTTPS,
		"baseUrl", config.Conf.BaseResultAddr,
		"shortlinkLength", config.Conf.ShortLinkLength,
		"charsetLength", len(config.Conf.ShortLinkCharset),
		"FileStoragePath", config.Conf.FileStoragePath,
		"StorageType", config.Conf.StorageType,
		"AuditFile", config.Conf.AuditFile,
		"AuditURL", config.Conf.AuditURL,
	)

	if config.Conf.StorageType == config.Database {
		if schemaErr := configdb.InitSchema(config.Conf.DBDSN); schemaErr != nil {
			logger.Log.Fatalw(
				"Failed to apply database migrations",
				"error", schemaErr,
			)
		}

		logger.Log.Info("Database schema is up to date")
	}

	db := repository.NewShortLinkDB(config.Conf.FileStoragePath, config.Conf.DBDSN, config.Conf.StorageType)
	defer func() {
		if err := db.Close(); err != nil {
			logger.Log.Errorf("Close storage: %v", err)
		}
	}()

	s := service.NewSemaphore(10)
	defer s.Wait()
	node, err := snowflake.NewNode(1)
	if err != nil {
		logger.Log.Fatalw(
			"Failed to generate node",
			"error", err,
		)
	}

	ap := &service.AuditPublisher{}

	if config.Conf.AuditFile != "" {
		fa, auditErr := repository.NewFileAudit(config.Conf.AuditFile)
		if auditErr != nil {
			logger.Log.Fatalw(
				"Failed to init file audit",
				"error", auditErr,
			)
		}
		ap.Register("file", fa)
		defer func() {
			if err := fa.Close(); err != nil {
				logger.Log.Errorf("Close file audit: %v", err)
			}
		}()
	}
	if config.Conf.AuditURL != "" {
		ha := repository.NewHTTPAudit(config.Conf.AuditURL)
		ap.Register("url", ha)
	}
	defer ap.Wait()

	r := RootRouter(config.Conf, db, s, node, ap)

	// For profiler
	if config.Conf.UseProfiler {
		profiler := &http.Server{Addr: "127.0.0.1:6060"}
		defer func() {
			if err := profiler.Shutdown(context.Background()); err != nil {
				logger.Log.Errorf("Shutdown pprof server: %v", err)
			}
		}()
		go func() {
			if profilerErr := profiler.ListenAndServe(); profilerErr != nil && !errors.Is(profilerErr, http.ErrServerClosed) {
				logger.Log.Errorf("pprof server: %v", profilerErr)
			}
		}()
	}
	listener, err := newServerListener(config.Conf)
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
		return
	}
	err = serveUntilShutdown(ctx, &http.Server{Handler: r}, listener)
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
	}
}

// serveUntilShutdown drains active requests before returning, including on a
// listener error. The signal context must not be used as the request context.
func serveUntilShutdown(ctx context.Context, server *http.Server, listener net.Listener) error {
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	var serveErr error
	select {
	case <-ctx.Done():
		shutdownErr := server.Shutdown(context.Background())
		serveErr = <-done
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(serveErr, shutdownErr)
	case serveErr = <-done:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(serveErr, server.Shutdown(context.Background()))
	}
}
