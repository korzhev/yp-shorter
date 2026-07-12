package main

import (
	"fmt"
	"net/http"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/{id}", ShortLinkHandle.GetByIDLinkHandlerFunc)
	r.Post("/", ShortLinkHandle.SaveLinkHandlerFunc)
	return r
}

func main() {
	config.ParseFlags()
	fmt.Printf("Config: -a %s -b %s -l %v -c %v \n", config.Conf.RunAddr, config.Conf.BaseResultAddr, config.Conf.ShortLinkLength, len(config.Conf.ShortLinkCharset))
	fmt.Printf("Server starting on %s\n", config.Conf.RunAddr)
	if err := http.ListenAndServe(config.Conf.RunAddr, RootRouter(config.Conf)); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
