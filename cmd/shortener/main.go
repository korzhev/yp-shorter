package main

import (
	"net/http"

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

func RootRouter(c config.Config, db *repository.ShortLinkDB, s *service.Semaphore, node *snowflake.Node) chi.Router {
	var ShortLinkHandler = handler.ShortLinkHandler{
		ShortLinkService: service.ShortLinkService{
			Charset:     c.ShortLinkCharset,
			IDLength:    c.ShortLinkLength,
			ShortLinkDB: db,
			DBSemaphore: s,
		},
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
	err := config.ParseFlags()
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
		return
	}
	config.Conf.DetectStorageType()
	logger.InitLogger(config.Conf.LogLevel)
	defer logger.Log.Sync()

	logger.Log.Infow("Server starting with params",
		"address", config.Conf.RunAddr,
		"baseUrl", config.Conf.BaseResultAddr,
		"shortlinkLength", config.Conf.ShortLinkLength,
		"charsetLength", len(config.Conf.ShortLinkCharset),
		"FileStoragePath", config.Conf.FileStoragePath,
		"StorageType", config.Conf.StorageType,
	)

	if config.Conf.StorageType == config.Database {
		if err := configdb.InitSchema(config.Conf.DBDSN); err != nil {
			logger.Log.Fatalw(
				"Failed to apply database migrations",
				"error", err,
			)
		}

		logger.Log.Info("Database schema is up to date")
	}

	db := repository.NewShortLinkDB(config.Conf.FileStoragePath, config.Conf.DBDSN, config.Conf.StorageType)
	defer db.Close()

	s := service.NewSemaphore(10)
	node, err := snowflake.NewNode(1)
	if err != nil {
		logger.Log.Fatalw(
			"Failed to generate node",
			"error", err,
		)
	}
	r := RootRouter(config.Conf, db, s, node)
	err = http.ListenAndServe(config.Conf.RunAddr, r)
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
	}
}
