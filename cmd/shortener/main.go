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

var ShortLinkHandle = handler.ShorLinkHandler{
	ShortLinkService: service.ShortLinkService{
		Charset:     config.ShortLinkCharset,
		IDLength:    config.FlagShortLinkLength,
		ShortLinkDB: repository.SLDB,
	},
}

func RootRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/{id}", ShortLinkHandle.GetByIDLinkHandlerFunc)
	r.Post("/", ShortLinkHandle.SaveLinkHandlerFunc)
	return r
}

func main() {
	config.ParseFlags()

	fmt.Printf("Server starting on %s\n", config.FlagRunAddr)
	if err := http.ListenAndServe(config.FlagRunAddr, RootRouter()); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
