package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const testUserID = 42

func withUserID(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), "UserID", testUserID))
}

func TestIsDuplicateError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "PostgreSQL duplicate link",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "short_links_link_uidx",
			},
			want: true,
		},
		{
			name: "Wrapped PostgreSQL duplicate link",
			err: fmt.Errorf("save link: %w", &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "short_links_link_uidx",
			}),
			want: true,
		},
		{
			name: "PostgreSQL different constraint",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "short_links_pkey",
			},
			want: false,
		},
		{
			name: "PostgreSQL different code",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "short_links_link_uidx",
			},
			want: false,
		},
		{
			name: "In-memory duplicate link",
			err:  repository.NewInMemoryDuplicateError("https://example.com"),
			want: true,
		},
		{
			name: "Wrapped in-memory duplicate link",
			err:  fmt.Errorf("save link: %w", repository.NewInMemoryDuplicateError("https://example.com")),
			want: true,
		},
		{
			name: "In-memory duplicate with empty link",
			err:  repository.NewInMemoryDuplicateError(""),
			want: false,
		},
		{
			name: "Unrelated error",
			err:  errors.New("save failed"),
			want: false,
		},
		{
			name: "Nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsDuplicateError(tt.err))
		})
	}
}

func TestSaveLinkHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}

		link := "https://example.com"
		ID := "abcde"
		shortLink := model.ShortLink{ID: ID, Link: link, UserID: testUserID}

		mockService.EXPECT().Save(gomock.Any(), link, testUserID).Return(shortLink, nil)

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		req = withUserID(req)
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
		mockService.EXPECT().Save(gomock.Any(), link, testUserID).Return(model.ShortLink{}, errors.New("internal error"))

		req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(link))
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal error")
	})

	t.Run("Missing UserID", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://example.com"))
		rr := httptest.NewRecorder()

		h.SaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "UserID not defined or empty")
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
		shortLink := model.ShortLink{ID: ID, Link: link, UserID: testUserID}

		mockService.EXPECT().Save(gomock.Any(), link, testUserID).Return(shortLink, nil)

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":"`+link+`"}`))
		req = withUserID(req)
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
		mockService.EXPECT().Save(gomock.Any(), link, testUserID).Return(model.ShortLink{}, errors.New("internal error"))

		req, _ := http.NewRequest("POST", "/api/shorten", bytes.NewBufferString(`{"url":"`+link+`"}`))
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "internal error")
	})

	t.Run("Missing UserID", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/shorten",
			bytes.NewBufferString(`{"url":"https://example.com"}`),
		)
		rr := httptest.NewRecorder()

		h.APISaveLinkHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "UserID not defined or empty")
	})
}

func TestAPISaveLinkBatchHandlerFunc(t *testing.T) {
	const baseResultAddr = "http://localhost:8080"
	oldBaseResultAddr := config.Conf.BaseResultAddr
	config.Conf.BaseResultAddr = baseResultAddr
	t.Cleanup(func() {
		config.Conf.BaseResultAddr = oldBaseResultAddr
	})

	batchRequest := []model.ShortLinkBatchItemRequest{
		{CorrelationID: "first-id", OriginalURL: "https://example.com/first"},
		{CorrelationID: "second-id", OriginalURL: "https://example.com/second"},
	}
	savedLinks := []model.ShortLink{
		{ID: "first-short-id", Link: "https://example.com/first", UserID: testUserID},
		{ID: "second-short-id", Link: "https://example.com/second", UserID: testUserID},
	}

	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().SaveBatch(gomock.Any(), testUserID, batchRequest).Return(savedLinks, nil)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/shorten/batch",
			bytes.NewBufferString(`[
				{"correlation_id":"first-id","original_url":"https://example.com/first"},
				{"correlation_id":"second-id","original_url":"https://example.com/second"}
			]`),
		)
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.JSONEq(t, `[
			{"correlation_id":"first-id","short_url":"http://localhost:8080/first-short-id"},
			{"correlation_id":"second-id","short_url":"http://localhost:8080/second-short-id"}
		]`, rr.Body.String())
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString(`[{`))
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Cannot decode request JSON body")
	})

	t.Run("Too many items", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		batch := make([]model.ShortLinkBatchItemRequest, 21)
		for i := range batch {
			batch[i] = model.ShortLinkBatchItemRequest{
				CorrelationID: "correlation-id",
				OriginalURL:   "https://example.com",
			}
		}
		body, err := json.Marshal(batch)
		assert.NoError(t, err)
		mockService.EXPECT().SaveBatch(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Too many items in batch request")
	})

	t.Run("Empty batch", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		emptyBatch := []model.ShortLinkBatchItemRequest{}
		mockService.EXPECT().SaveBatch(gomock.Any(), testUserID, emptyBatch).Return([]model.ShortLink{}, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString(`[]`))
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.JSONEq(t, `[]`, rr.Body.String())
	})

	t.Run("Empty correlation ID or original URL", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/shorten/batch",
			bytes.NewBufferString(`[{"correlation_id":"","original_url":"https://example.com"}]`),
		)
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Empty CorrelationID or OriginalURL")
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		expectedErr := errors.New("batch save failed")
		mockService.EXPECT().SaveBatch(gomock.Any(), testUserID, batchRequest).Return(nil, expectedErr)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/shorten/batch",
			bytes.NewBufferString(`[
				{"correlation_id":"first-id","original_url":"https://example.com/first"},
				{"correlation_id":"second-id","original_url":"https://example.com/second"}
			]`),
		)
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), expectedErr.Error())
	})

	t.Run("Different response length", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().SaveBatch(gomock.Any(), testUserID, batchRequest).Return(savedLinks[:1], nil)

		body, err := json.Marshal(batchRequest)
		assert.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		req = withUserID(req)
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Different length of links 1 and req 2")
	})

	t.Run("Missing UserID", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().SaveBatch(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		body, err := json.Marshal(batchRequest)
		assert.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		h.APISaveLinkBatchHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "UserID not defined or empty")
	})
}

func TestAPIGetLinksByUserIDHandlerFunc(t *testing.T) {
	const baseResultAddr = "http://localhost:8080"
	oldBaseResultAddr := config.Conf.BaseResultAddr
	config.Conf.BaseResultAddr = baseResultAddr
	t.Cleanup(func() {
		config.Conf.BaseResultAddr = oldBaseResultAddr
	})

	t.Run("Success", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		links := []model.ShortLink{
			{ID: "first-id", Link: "https://example.com/first", UserID: testUserID},
			{ID: "second-id", Link: "https://example.com/second", UserID: testUserID},
		}
		mockService.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(links, nil)

		req := withUserID(httptest.NewRequest(http.MethodGet, "/api/user/urls", nil))
		rr := httptest.NewRecorder()

		h.APIGetLinksByUserIDHandlerFunc(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.JSONEq(t, `[
			{"short_url":"http://localhost:8080/first-id","original_url":"https://example.com/first"},
			{"short_url":"http://localhost:8080/second-id","original_url":"https://example.com/second"}
		]`, rr.Body.String())
	})

	t.Run("Service Error", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		expectedErr := errors.New("get links failed")
		mockService.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, expectedErr)

		req := withUserID(httptest.NewRequest(http.MethodGet, "/api/user/urls", nil))
		rr := httptest.NewRecorder()

		h.APIGetLinksByUserIDHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), expectedErr.Error())
	})

	t.Run("Missing UserID", func(t *testing.T) {
		mockService := mocks.NewMockIShortLinkService(gomock.NewController(t))
		h := ShortLinkHandler{ShortLinkService: mockService}
		mockService.EXPECT().GetByUserID(gomock.Any(), gomock.Any()).Times(0)

		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		rr := httptest.NewRecorder()

		h.APIGetLinksByUserIDHandlerFunc(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "UserID not defined or empty")
	})
}

func init() {
	// I don't want to do anything with logger singletone
	logger.InitLogger("error")
}
