package main

import (
	"fmt"
	"net/http"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"
)

var ShortLinkHandle = handler.ShorLinkHandle{
	ShortLinkService: service.ShortLinkService{
		Charset:     config.ShortLinkCharset,
		IDLength:    config.ShortLinkLength,
		ShortLinkDB: repository.SLDB,
	},
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodPost {
		ShortLinkHandle.SaveLinkHandler(w, r)
		return
	}

	if r.Method == http.MethodGet && len(r.URL.Path) > 1 {
		ShortLinkHandle.GetByIDLinkHandler(w, r)
		return
	}

	http.Error(w, "Unknown error", http.StatusBadRequest)
}

func main() {
	mux := http.NewServeMux()

	// Handle POST /
	mux.HandleFunc("/", rootHandler)

	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
