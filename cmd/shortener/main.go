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
			Charset:     c.FlagShortLinkCharset,
			IDLength:    c.FlagShortLinkLength,
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
	fmt.Printf("Config: -a %s -b %s -l %v -c %v \n", config.Conf.FlagRunAddr, config.Conf.FlagBaseResultAddr, config.Conf.FlagShortLinkLength, len(config.Conf.FlagShortLinkCharset))
	fmt.Printf("Server starting on %s\n", config.Conf.FlagRunAddr)
	if err := http.ListenAndServe(config.Conf.FlagRunAddr, RootRouter(config.Conf)); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
