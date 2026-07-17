package main

import (
	"net/http"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/middleware"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"

	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"
)

func RootRouter(c config.Config) chi.Router {
	var ShortLinkHandle = handler.ShortLinkHandler{
		ShortLinkService: service.ShortLinkService{
			Charset:     c.ShortLinkCharset,
			IDLength:    c.ShortLinkLength,
			ShortLinkDB: repository.NewShortLinkDB(),
		},
	}
	r := chi.NewRouter()

	r.Use(middleware.NewLoggerMiddleware(logger.Log))
	r.Use(middleware.NewCompressorMiddleware())
	// r.Use(middleware.Logger)
	r.Use(chiMW.RedirectSlashes)
	r.Use(chiMW.Recoverer)

	r.Get("/{id}", ShortLinkHandle.GetByIDLinkHandlerFunc)
	r.Post("/", ShortLinkHandle.SaveLinkHandlerFunc)
	r.Post("/api/shorten", ShortLinkHandle.APISaveLinkHandlerFunc)
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
	)

	err := http.ListenAndServe(config.Conf.RunAddr, RootRouter(config.Conf))
	if err != nil {
		logger.Log.Errorf("Error starting server: %s\n", err)
	}
}
