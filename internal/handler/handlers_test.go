package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockShortLinkService is a mock implementation of IShortLinkService
type MockShortLinkService struct {
	mock.Mock
}

func (m *MockShortLinkService) GenerateID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockShortLinkService) GetById(id string) (model.ShortLink, error) {
	args := m.Called(id)
	return args.Get(0).(model.ShortLink), args.Error(1)
}

func (m *MockShortLinkService) Save(link string) (model.ShortLink, error) {
	args := m.Called(link)
	return args.Get(0).(model.ShortLink), args.Error(1)
}

func TestSaveLinkHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		ID := "abcde"
		shortLink := model.ShortLink{ID: ID, Link: link}

		mockService.On("Save", link).Return(shortLink, nil)

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Contains(t, rr.Body.String(), config.FlagBaseResultAddr+"/"+ID)
		mockService.AssertExpectations(t)
	})

	t.Run("Empty Body", func(t *testing.T) {
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(""))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Empty body")
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		mockService.On("Save", link).Return(model.ShortLink{}, errors.New("internal error"))

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal error")
	})
}

func TestGetByIDLinkHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}
		r := chi.NewRouter()
		r.Get("/{id}", h.GetByIDLinkHandlerFunc)

		id := "abcde"
		link := "https://example.com"
		shortLink := model.ShortLink{ID: id, Link: link}

		mockService.On("GetById", id).Return(shortLink, nil)

		req, _ := http.NewRequest("GET", "/"+id, nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
		assert.Equal(t, link, rr.Header().Get("Location"))
		mockService.AssertExpectations(t)
	})

	t.Run("Empty ID", func(t *testing.T) {
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}
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
		mockService := new(MockShortLinkService)
		h := ShorLinkHandler{ShortLinkService: mockService}
		r := chi.NewRouter()
		r.Get("/{id}", h.GetByIDLinkHandlerFunc)

		id := "nonexistent"
		mockService.On("GetById", id).Return(model.ShortLink{}, errors.New("not found"))

		req, _ := http.NewRequest("GET", "/"+id, nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "not found")
	})
}
