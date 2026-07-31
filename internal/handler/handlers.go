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
	sl, err := s.ShortLinkService.Save(r.Context(), link)

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
	sl, err := s.ShortLinkService.GetByShort(r.Context(), id)
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
	sl, err := s.ShortLinkService.Save(r.Context(), link)

	if err != nil {
		logger.Log.Infow("Unexpected error while saving short link", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	l := fmt.Sprintf("%s/%s", config.Conf.BaseResultAddr, sl.ID)
	res := model.ShortLinkResponse{
		Result: l,
	}
	resp, err := json.Marshal(res)
	if err != nil {
		logger.Log.Infow("Enccoding response", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)

}

func (s ShortLinkHandler) APISaveLinkBatchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	var req []model.ShortLinkBatchItemRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Infow("Cannot decode request JSON body", "error", err)
		http.Error(w, "Cannot decode request JSON body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	if len(req) > 20 {
		logger.Log.Infow("Request too long", "ShortLinkBatchRequest", req)
		http.Error(w, "Too many items in batch request", http.StatusBadRequest)
		return
	}
	for _, sl := range req {
		if sl.CorrelationID == "" || sl.OriginalURL == "" {
			logger.Log.Infow("Empty URL", "ShortLink", sl)
			http.Error(w, "Empty CorrelationID or OriginalURL", http.StatusBadRequest)
			return
		}
	}

	links, err := s.ShortLinkService.SaveBatch(r.Context(), req)

	if err != nil {
		logger.Log.Infow("Unexpected error while saving short links", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(links) != len(req) {
		s := fmt.Sprintf("Different length of links %v and req %v", len(links), len(req))
		logger.Log.Infow(s, "error", err)
		http.Error(w, s, http.StatusBadRequest)
		return
	}
	res := make([]model.ShortLinkBatchItemResponse, 0, len(links))
	for i, l := range links {
		res = append(res, model.ShortLinkBatchItemResponse{CorrelationID: req[i].CorrelationID, ShortURL: config.Conf.BaseResultAddr +"/"+ l.ID})
	}

	resp, err := json.Marshal(res)
	if err != nil {
		logger.Log.Infow("Enccoding response", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}
