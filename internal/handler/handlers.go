package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/korzhev/yp-shorter/internal/service"
)

type ShorLinkHandle struct {
	ShortLinkService service.ShortLinkService
}

func (s ShorLinkHandle) SaveLinkHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	link := string(body)
	if link == "" {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	sl, err := s.ShortLinkService.Save(link)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	l := fmt.Sprintf("http://localhost:8080/%s", sl.ID)
	w.Write([]byte(l))
}

func (s ShorLinkHandle) GetByIDLinkHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "Empty ID", http.StatusBadRequest)
		return
	}
	sl, err := s.ShortLinkService.GetById(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", sl.Link)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
