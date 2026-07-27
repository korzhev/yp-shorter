package main

import (
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/middleware"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"

	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"
)

func RootRouter(c config.Config, db *repository.ShortLinkDB) chi.Router {
	var ShortLinkHandler = handler.ShortLinkHandler{
		ShortLinkService: service.ShortLinkService{
			Charset:     c.ShortLinkCharset,
			IDLength:    c.ShortLinkLength,
			ShortLinkDB: db,
		},
	}
	var PingHandler = handler.PingHandler{
		Pg: db.DB,
	}

	r := chi.NewRouter()

	r.Use(middleware.NewLoggerMiddleware(logger.Log))
	r.Use(middleware.NewCompressorMiddleware())
	r.Use(chiMW.RedirectSlashes)
	r.Use(chiMW.Recoverer)

	r.Get("/{id}", ShortLinkHandler.GetByIDLinkHandlerFunc)
	r.Post("/", ShortLinkHandler.SaveLinkHandlerFunc)
	r.Post("/api/shorten", ShortLinkHandler.APISaveLinkHandlerFunc)
	r.Get("/ping", PingHandler.PingHandlerFunc)
	return r
}

func main() {
	config.ParseFlags()
	logger.InitLogger(config.Conf.LogLevel)
	defer logger.Log.Sync()

	logger.Log.Infow("Server starting with params",
		"address", config.Conf.RunAddr,
		"baseUrl", config.Conf.BaseResultAddr,
		"shortlinkLength", config.Conf.ShortLinkLength,
		"charsetLength", len(config.Conf.ShortLinkCharset),
		"FileStoragePath", config.Conf.FileStoragePath,
	)

	db := repository.NewShortLinkDB(config.Conf.FileStoragePath, config.Conf.DBDSN, config.Conf.StorageType)
	defer db.Close()

	r := RootRouter(config.Conf, db)
	err := http.ListenAndServe(config.Conf.RunAddr, r)
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
	}
}
