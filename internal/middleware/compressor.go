package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type ComressorMiddleware func(next http.Handler) http.Handler

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

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

// first Write() flushes headers and after that they can't be changed
// so we check content type to select Response Writer
func (c *compressWriter) Write(p []byte) (int, error) {
	if c.shouldCompress() {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 && c.shouldCompress() {
		c.w.Header().Set("Content-Encoding", "gzip")
	}
	c.w.WriteHeader(statusCode)
}

func (c *compressWriter) Close() error {
	return c.zw.Close()
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

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.gr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.gr.Close()
}

func NewCompressorMiddleware() ComressorMiddleware {
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
