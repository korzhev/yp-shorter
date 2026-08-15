package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootRouter(t *testing.T) {
	const baseResultAddr = "http://short.test"
	c := config.Config{
		BaseResultAddr:   baseResultAddr,
		ShortLinkCharset: config.DefaultCharset,
		ShortLinkLength:  6,
		TokenExpMinutes:  5,
		TokenSecret:      "router-test-secret",
	}
	oldConfig := config.Conf
	config.Conf = c
	t.Cleanup(func() {
		config.Conf = oldConfig
	})
	require.NoError(t, logger.InitLogger("error"))

	t.Run("creates and redirects a short link", func(t *testing.T) {
		db := repository.NewShortLinkDB("", "", config.InMemory)
		router := RootRouter(c, db)
		originalURL := "https://example.com/article"

		createRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(originalURL))
		createResponse := httptest.NewRecorder()
		router.ServeHTTP(createResponse, createRequest)

		require.Equal(t, http.StatusCreated, createResponse.Code)
		assert.Equal(t, "text/plain", createResponse.Header().Get("Content-Type"))
		shortURL := createResponse.Body.String()
		require.True(t, strings.HasPrefix(shortURL, baseResultAddr+"/"))
		id := strings.TrimPrefix(shortURL, baseResultAddr+"/")
		require.Len(t, id, c.ShortLinkLength)

		redirectRequest := httptest.NewRequest(http.MethodGet, "/"+id, nil)
		redirectResponse := httptest.NewRecorder()
		router.ServeHTTP(redirectResponse, redirectRequest)

		assert.Equal(t, http.StatusTemporaryRedirect, redirectResponse.Code)
		assert.Equal(t, originalURL, redirectResponse.Header().Get("Location"))
	})

	t.Run("handles gzipped JSON request and response", func(t *testing.T) {
		db := repository.NewShortLinkDB("", "", config.InMemory)
		router := RootRouter(c, db)
		requestBody := gzipData(t, []byte(`{"url":"https://example.com/gzip"}`))

		request := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(requestBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Encoding", "gzip")
		request.Header.Set("Accept-Encoding", "gzip")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		require.Equal(t, http.StatusCreated, response.Code)
		assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
		assert.Equal(t, "gzip", response.Header().Get("Content-Encoding"))
		body := ungzipData(t, response.Body.Bytes())
		var result model.ShortLinkResponse
		require.NoError(t, json.Unmarshal(body, &result))
		assert.True(t, strings.HasPrefix(result.Result, baseResultAddr+"/"))
	})

	t.Run("routes batch requests", func(t *testing.T) {
		db := repository.NewShortLinkDB("", "", config.InMemory)
		router := RootRouter(c, db)
		requestBody := `[
			{"correlation_id":"first","original_url":"https://example.com/first"},
			{"correlation_id":"second","original_url":"https://example.com/second"}
		]`

		request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(requestBody))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		require.Equal(t, http.StatusCreated, response.Code)
		var result []model.ShortLinkBatchItemResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Len(t, result, 2)
		assert.Equal(t, "first", result[0].CorrelationID)
		assert.Equal(t, "second", result[1].CorrelationID)
		assert.True(t, strings.HasPrefix(result[0].ShortURL, baseResultAddr+"/"))
		assert.True(t, strings.HasPrefix(result[1].ShortURL, baseResultAddr+"/"))
	})

	t.Run("returns links created by authenticated user", func(t *testing.T) {
		db := repository.NewShortLinkDB("", "", config.InMemory)
		router := RootRouter(c, db)
		originalURL := "https://example.com/user-link"

		createRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(originalURL))
		createResponse := httptest.NewRecorder()
		router.ServeHTTP(createResponse, createRequest)

		require.Equal(t, http.StatusCreated, createResponse.Code)
		shortURL := createResponse.Body.String()
		cookies := createResponse.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, "Auth", cookies[0].Name)

		listRequest := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		listRequest.AddCookie(cookies[0])
		listResponse := httptest.NewRecorder()
		router.ServeHTTP(listResponse, listRequest)

		require.Equal(t, http.StatusCreated, listResponse.Code)
		assert.Equal(t, "application/json", listResponse.Header().Get("Content-Type"))
		var result []model.UserShortLinkResponse
		require.NoError(t, json.Unmarshal(listResponse.Body.Bytes(), &result))
		require.Len(t, result, 1)
		assert.Equal(t, originalURL, result[0].OriginalURL)
		assert.Equal(t, shortURL, result[0].ShortURL)
	})

	t.Run("routes database ping", func(t *testing.T) {
		sqlDB, mock := newPingDB(t)
		mock.ExpectPing()
		db := &repository.ShortLinkDB{DB: sqlDB}
		router := RootRouter(c, db)

		request := httptest.NewRequest(http.MethodGet, "/ping", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("returns not found for an unknown route", func(t *testing.T) {
		db := repository.NewShortLinkDB("", "", config.InMemory)
		router := RootRouter(c, db)
		request := httptest.NewRequest(http.MethodGet, "/api/unknown/path", nil)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func ungzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reader.Close())
	})
	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	return result
}

func newPingDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return db, mock
}
