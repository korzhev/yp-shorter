package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/middleware"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"
)

// AuditPub publishes audit events for short link operations.
type AuditPub interface {
	// Publish sends an action, user ID, and original URL to audit subscribers.
	Publish(action string, userID int, url string)
}

// ShortLinkHandler handles HTTP requests for short links and publishes audit events.
type ShortLinkHandler struct {
	// ShortLinkService performs short link operations.
	ShortLinkService service.IShortLinkService
	// Audit receives events for individual link creation and redirects.
	Audit AuditPub
}

// IsDuplicateError reports whether err, including a wrapped error, indicates
// a duplicate original URL in PostgreSQL or in-memory storage.
func IsDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// uniq index error
		if pgErr.Code == "23505" && pgErr.ConstraintName == "short_links_link_uidx" {
			return true
		}
	}
	var dError *repository.InMemoryDuplicateError
	if errors.As(err, &dError) {
		if dError.Link != "" {
			return true
		}
	}

	return false
}

// SaveLinkHandlerFunc shortens the URL in the request body for the context user.
// It returns a plain-text short URL with 201 for a new link or 409 for a duplicate,
// publishes a shorten event on success, and returns 400 on invalid input or errors.
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

	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
		http.Error(w, "UserID not defined or empty", http.StatusBadRequest)
		return
	}
	sl, err := s.ShortLinkService.Save(ctx, link, userID)

	status := http.StatusCreated
	if IsDuplicateError(err) {
		status = http.StatusConflict
		sl, err = s.ShortLinkService.GetByLink(ctx, link)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.Audit.Publish(model.AuditActionShorten, userID, link)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	l := fmt.Sprintf("%s/%s", config.Conf.BaseResultAddr, sl.ID)
	w.Write([]byte(l))
}

// GetByIDLinkHandlerFunc resolves the id route parameter and redirects with 307.
// It returns 410 when the service returns a deleted link and 400 on lookup errors
// or an empty ID. Successful redirects publish a follow event.
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
	if sl.Deleted {
		w.WriteHeader(http.StatusGone)
		return
	}
	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
	}
	s.Audit.Publish(model.AuditActionFollow, userID, sl.Link)

	w.Header().Set("Location", sl.Link)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// APISaveLinkHandlerFunc shortens a JSON URL request for the context user.
// It returns a JSON result with 201 for a new link or 409 for a duplicate,
// publishes a shorten event on success, and returns 400 on invalid input or errors.
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

	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
		http.Error(w, "UserID not defined or empty", http.StatusBadRequest)
		return
	}
	sl, err := s.ShortLinkService.Save(ctx, link, userID)
	status := http.StatusCreated
	if IsDuplicateError(err) {
		status = http.StatusConflict
		sl, err = s.ShortLinkService.GetByLink(ctx, link)
	}

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

	s.Audit.Publish(model.AuditActionShorten, userID, link)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(resp)

}

// APISaveLinkBatchHandlerFunc shortens up to 20 JSON batch items for the context
// user. It returns correlated short URLs with 201, or 400 on invalid input or errors.
func (s ShortLinkHandler) APISaveLinkBatchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	var req []model.ShortLinkBatchItemRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Infow("Cannot decode request JSON body", "error", err)
		http.Error(w, "Cannot decode request JSON body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	// i don't need DOS
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
	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
		http.Error(w, "UserID not defined or empty", http.StatusBadRequest)
		return
	}

	links, err := s.ShortLinkService.SaveBatch(ctx, userID, req)

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
		res = append(res, model.ShortLinkBatchItemResponse{CorrelationID: req[i].CorrelationID, ShortURL: config.Conf.BaseResultAddr + "/" + l.ID})
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

// APIGetLinksByUserIDHandlerFunc returns the context user's links as JSON with
// 200, or 204 if none exist. Missing user IDs and service errors result in 400.
func (s ShortLinkHandler) APIGetLinksByUserIDHandlerFunc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
		http.Error(w, "UserID not defined or empty", http.StatusBadRequest)
		return
	}
	links, err := s.ShortLinkService.GetByUserID(ctx, userID)
	if err != nil {
		logger.Log.Infow("Unexpected error while getting short link", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res := make([]model.UserShortLinkResponse, 0, len(links))
	for _, l := range links {
		res = append(res, model.UserShortLinkResponse{OriginalURL: l.Link, ShortURL: config.Conf.BaseResultAddr + "/" + l.ID})
	}

	resp, err := json.Marshal(res)
	if err != nil {
		logger.Log.Infow("Enccoding response", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(links) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// APIDeleteLinkBatchHandlerFunc accepts a JSON array of up to 200 short IDs for
// deletion by the context user. It returns 202 when accepted, not when deletion
// completes, or 400 on invalid input or an immediate service error.
func (s ShortLinkHandler) APIDeleteLinkBatchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	var req []string
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Infow("Cannot decode request JSON body", "error", err)
		http.Error(w, "Cannot decode request JSON body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	// i don't need DOS
	if len(req) > 200 {
		logger.Log.Infow("Request too long", "ShortLinkBatchRequest", req)
		http.Error(w, "Too many items in batch request", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	userID, ok := ctx.Value(middleware.UserIDContextKey).(int)
	if !ok {
		logger.Log.Infow("UserID not defined or empty", "userID", ctx.Value(middleware.UserIDContextKey))
		http.Error(w, "UserID not defined or empty", http.StatusBadRequest)
		return
	}

	err := s.ShortLinkService.DeleteBatch(ctx, userID, req)

	if err != nil {
		logger.Log.Infow("Unexpected error while deleting short links", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
