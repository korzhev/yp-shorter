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
		IDLength:    config.ShortLinkLength,
		ShortLinkDB: repository.SLDB,
	},
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodPost {
		ShortLinkHandle.SaveLinkHandlerFunc(w, r)
		return
	}

	if r.Method == http.MethodGet && len(r.URL.Path) > 1 {
		ShortLinkHandle.GetByIDLinkHandlerFunc(w, r)
		return
	}

	http.Error(w, "Unknown error", http.StatusBadRequest)
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
	// mux := http.NewServeMux()

	// // Handle POST /
	// mux.HandleFunc("/", rootHandler)

	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", RootRouter()); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
