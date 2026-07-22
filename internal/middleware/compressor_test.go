package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCompressorMiddleware(t *testing.T) {
	t.Run("compresses json response when client accepts gzip", func(t *testing.T) {
		body := []byte(`{"result":"ok"}`)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
		assert.Equal(t, body, ungzipBody(t, rr.Body.Bytes()))
	})

	t.Run("compresses html response when accept encoding contains gzip", func(t *testing.T) {
		body := []byte("<html><body>ok</body></html>")
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "br, GZip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
		assert.Equal(t, body, ungzipBody(t, rr.Body.Bytes()))
	})

	t.Run("does not compress response when client does not accept gzip", func(t *testing.T) {
		body := []byte(`{"result":"ok"}`)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write(body)
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, body, rr.Body.Bytes())
	})

	t.Run("does not compress unsupported content type", func(t *testing.T) {
		body := []byte("plain text")
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, body, rr.Body.Bytes()[:len(body)])
	})

	t.Run("does not set gzip encoding header for unsuccessful response", func(t *testing.T) {
		body := []byte(`{"error":"bad request"}`)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write(body)
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, body, ungzipBody(t, rr.Body.Bytes()))
	})

	t.Run("decompresses gzip request body", func(t *testing.T) {
		body := []byte(`{"url":"https://example.com"}`)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actualBody, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, body, actualBody)

			w.WriteHeader(http.StatusCreated)
		})

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipBody(t, body)))
		req.Header.Set("Content-Encoding", "gzip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("returns bad request error for invalid gzip request body", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not gzip"))
		req.Header.Set("Content-Encoding", "gzip")
		rr := httptest.NewRecorder()

		NewCompressorMiddleware()(next).ServeHTTP(rr, req)

		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestCompressWriterShouldCompress(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "application json",
			contentType: "application/json",
			expected:    true,
		},
		{
			name:        "application json with charset",
			contentType: "Application/JSON; charset=utf-8",
			expected:    true,
		},
		{
			name:        "text html",
			contentType: "text/html",
			expected:    true,
		},
		{
			name:        "text plain",
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "empty content type",
			contentType: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			rr.Header().Set("Content-Type", tt.contentType)

			cw := newCompressWriter(rr)
			defer func() {
				require.NoError(t, cw.Close())
			}()

			assert.Equal(t, tt.expected, cw.shouldCompress())
		})
	}
}

func gzipBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(body)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func ungzipBody(t *testing.T, body []byte) []byte {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, zr.Close())
	}()

	decoded, err := io.ReadAll(zr)
	require.NoError(t, err)

	return decoded
}
