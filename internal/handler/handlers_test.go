package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSaveLinkHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		ID := "abcde"
		shortLink := model.ShortLink{ID: ID, Link: link}

		mockService.EXPECT().Save(gomock.Any(), link).Return(shortLink, nil)

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Contains(t, rr.Body.String(), config.Conf.BaseResultAddr+"/"+ID)
	})

	t.Run("Empty Body", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Empty body")
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		mockService.EXPECT().Save(gomock.Any(), link).Return(model.ShortLink{}, errors.New("internal error"))

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal error")
	})
}

func TestGetByIDLinkHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		r := chi.NewRouter()
		r.Get("/{id}", h.GetByIDLinkHandlerFunc)

		id := "abcde"
		link := "https://example.com"
		shortLink := model.ShortLink{ID: id, Link: link}

		mockService.EXPECT().GetByShort(gomock.Any(), id).Return(shortLink, nil)

		req, _ := http.NewRequest("GET", "/"+id, nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
		assert.Equal(t, link, rr.Header().Get("Location"))
	})

	t.Run("Empty ID", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		r := chi.NewRouter()
		// To test the "Empty ID" logic, we need a route that matches but results in an empty 'id' parameter
		r.Get("/", h.GetByIDLinkHandlerFunc)

		req, _ := http.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Empty ID")
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		r := chi.NewRouter()
		r.Get("/{id}", h.GetByIDLinkHandlerFunc)

		id := "nonexistent"
		mockService.EXPECT().GetByShort(gomock.Any(), id).Return(model.ShortLink{}, errors.New("not found"))

		req, _ := http.NewRequest("GET", "/"+id, nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "not found")
	})
}

func TestAPISaveLinkHandlerFunc(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		ID := "abcde"
		shortLink := model.ShortLink{ID: ID, Link: link}

		mockService.EXPECT().Save(gomock.Any(), link).Return(shortLink, nil)

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":"`+link+`"}`))
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"result":"`+config.Conf.BaseResultAddr+"/"+ID+`"}`, rr.Body.String())
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":`))
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Cannot decode request JSON body")
	})

	t.Run("Empty URL", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":""}`))
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Empty URL")
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		mockService.EXPECT().Save(gomock.Any(), link).Return(model.ShortLink{}, errors.New("internal error"))

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":"`+link+`"}`))
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal error")
	})
}

func init() {
	// I don't want to do anything with logger singletone
	logger.InitLogger("error")
}
