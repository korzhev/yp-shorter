package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/service"
)

type ShortLinkHandler struct {
	ShortLinkService service.IShortLinkService
}

func (s ShortLinkHandler) SaveLinkHandlerFunc(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	link := string(body)
	if link == "" {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}
	fmt.Println(link)
	sl, err := s.ShortLinkService.Save(link)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	l := fmt.Sprintf("%s/%s", config.Conf.FlagBaseResultAddr, sl.ID)
	w.Write([]byte(l))
}

func (s ShortLinkHandler) GetByIDLinkHandlerFunc(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
