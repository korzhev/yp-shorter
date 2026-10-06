package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// CompressorMiddleware wraps a handler with gzip request and response support.
type CompressorMiddleware func(next http.Handler) http.Handler

// compressWriter implements http.ResponseWriter
type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header returns the underlying response headers.
func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

// Write compresses JSON and HTML response data and writes other types unchanged.
func (c *compressWriter) Write(p []byte) (int, error) {
	if c.shouldCompress() {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

// WriteHeader sends the status code, setting gzip encoding for JSON and HTML
// responses when the status code is less than 300.
func (c *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 && c.shouldCompress() {
		c.w.Header().Set("Content-Encoding", "gzip")
	}
	c.w.WriteHeader(statusCode)
}

// Close flushes and closes the gzip writer for JSON and HTML responses.
func (c *compressWriter) Close() error {
	if c.shouldCompress() {
		return c.zw.Close()
	}
	return nil
}

func (c *compressWriter) shouldCompress() bool {
	ct := strings.ToLower(c.w.Header().Get("Content-Type"))
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "text/html")
}

// compressReader implements io.ReadCloser
type compressReader struct {
	r  io.ReadCloser
	gr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &compressReader{
		r:  r,
		gr: gr,
	}, nil
}

// Read reads decompressed request data into p.
func (c compressReader) Read(p []byte) (n int, err error) {
	return c.gr.Read(p)
}

// Close closes the request body, then the gzip reader if the first close succeeds.
func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.gr.Close()
}

// NewCompressorMiddleware creates middleware that decompresses gzip request
// bodies and compresses JSON and HTML responses when the client accepts gzip.
// An invalid gzip request header or stream header results in a 400 response.
func NewCompressorMiddleware() CompressorMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := w

			acceptEncoding := r.Header.Get("Accept-Encoding")
			supportsGzip := strings.Contains(strings.ToLower(acceptEncoding), "gzip")
			if supportsGzip {
				cw := newCompressWriter(ww)
				ww = cw
				defer cw.Close()
			}

			contentEncoding := r.Header.Get("Content-Encoding")
			sendsGzip := strings.Contains(strings.ToLower(contentEncoding), "gzip")
			if sendsGzip {
				cr, err := newCompressReader(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				r.Body = cr
				defer cr.Close()
			}
			next.ServeHTTP(ww, r)
		})
	}
}
