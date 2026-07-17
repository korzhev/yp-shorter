package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
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
	sl, err := s.ShortLinkService.Save(link)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	l := fmt.Sprintf("%s/%s", config.Conf.BaseResultAddr, sl.ID)
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

func (s ShortLinkHandler) APISaveLinkHandlerFunc(w http.ResponseWriter, r *http.Request) {
	var req model.ShortLinkRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Infow("Cannot decode request JSON body", "error", err)
		http.Error(w, "Cannot decode request JSON body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	link := req.URL
	if link == "" {
		logger.Log.Infow("Empty URL")
		http.Error(w, "Empty URL", http.StatusBadRequest)
		return
	}
	sl, err := s.ShortLinkService.Save(link)

	if err != nil {
		logger.Log.Infow("Unexpected error while saving short link", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	l := fmt.Sprintf("%s/%s", config.Conf.BaseResultAddr, sl.ID)
	res := model.ShortLinkResponse{
		Result: l,
	}
	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := enc.Encode(res); err != nil {
		logger.Log.Infow("Enccoding response", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

}
