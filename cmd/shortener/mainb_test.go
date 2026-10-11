package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"
	"github.com/stretchr/testify/require"
)

// BenchmarkHTTPRoutes exercises RootRouter with an isolated in-memory store.
// Each operation executes 100 scenarios; use -benchtime=1x for exactly 100.
func BenchmarkHTTPRoutes(b *testing.B) {
	const (
		repetitions = 100
		urlCount    = 100
		followCount = 10
		batchSize   = 20
	)
	c := config.Config{
		BaseResultAddr:   "http://short.test",
		ShortLinkCharset: config.DefaultCharset,
		ShortLinkLength:  6,
		TokenExpMinutes:  5,
		TokenSecret:      "router-benchmark-secret",
		StorageType:      config.InMemory,
	}
	oldConfig, oldLogger := config.Conf, logger.Log
	config.Conf = c
	b.Cleanup(func() {
		config.Conf = oldConfig
		logger.Log = oldLogger
	})
	require.NoError(b, logger.InitLogger("error"))
	node, err := snowflake.NewNode(1)
	require.NoError(b, err)
	var router http.Handler
	var cookie *http.Cookie
	serve := func(method, path, body, contentType string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	request := func(method, path, body, contentType string, status int) ([]byte, http.Header) {
		b.Helper()
		res := serve(method, path, body, contentType)
		require.Equal(b, status, res.Code, "%s %s: %s", method, path, res.Body.String())
		if cookie == nil {
			result := res.Result()
			cookies := result.Cookies()
			require.NoError(b, result.Body.Close())
			require.Len(b, cookies, 1)
			require.Equal(b, "Auth", cookies[0].Name)
			cookie = cookies[0]
		}
		return res.Body.Bytes(), res.Header()
	}
	runID := time.Now().UnixNano()
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		for repetition := 0; repetition < repetitions; repetition++ {
			b.StopTimer()
			// Avoid accumulating soft-deleted records across scenarios.
			db := repository.NewShortLinkDB("", "", config.InMemory)
			router = RootRouter(c, db, service.NewSemaphore(10), node, &service.AuditPublisher{})
			cookie = nil
			originals := make([]string, urlCount)
			for j := range originals {
				originals[j] = fmt.Sprintf("https://example.com/benchmark/%d/%d/%d/%d", runID, i, repetition, j)
			}
			singleBody, err := json.Marshal(model.ShortLinkRequest{URL: originals[1]})
			require.NoError(b, err)
			batch := make([]model.ShortLinkBatchItemRequest, urlCount-2)
			for j := range batch {
				batch[j] = model.ShortLinkBatchItemRequest{
					CorrelationID: fmt.Sprint(j),
					OriginalURL:   originals[j+2],
				}
			}
			batchBodies := make([]string, 0, (len(batch)+batchSize-1)/batchSize)
			for start := 0; start < len(batch); start += batchSize {
				end := min(start+batchSize, len(batch))
				batchBody, err := json.Marshal(batch[start:end])
				require.NoError(b, err)
				batchBodies = append(batchBodies, string(batchBody))
			}
			b.StartTimer()

			// 1. POST /: create one URL and obtain the user's cookie.
			body, _ := request(http.MethodPost, "/", originals[0], "text/plain", http.StatusCreated)
			shortURLs := make([]string, urlCount)
			shortURLs[0] = string(body)

			// 2. POST /api/shorten: create one more URL.
			body, _ = request(http.MethodPost, "/api/shorten", string(singleBody), "application/json", http.StatusCreated)
			var single model.ShortLinkResponse
			require.NoError(b, json.Unmarshal(body, &single))
			shortURLs[1] = single.Result

			// 3. POST /api/shorten/batch: create 98 URLs in batches of at most 20.
			for batchIndex, batchBody := range batchBodies {
				start := batchIndex * batchSize
				end := min(start+batchSize, len(batch))
				body, _ = request(http.MethodPost, "/api/shorten/batch", batchBody, "application/json", http.StatusCreated)
				var saved []model.ShortLinkBatchItemResponse
				require.NoError(b, json.Unmarshal(body, &saved))
				require.Len(b, saved, end-start)
				for j, item := range saved {
					require.Equal(b, batch[start+j].CorrelationID, item.CorrelationID)
					shortURLs[start+j+2] = item.ShortURL
				}
			}
			ids := make([]string, urlCount)
			expected := make(map[string]string, urlCount)
			for j, shortURL := range shortURLs {
				parsed, err := url.Parse(shortURL)
				require.NoError(b, err)
				require.NotEmpty(b, parsed.Host)
				ids[j] = strings.TrimPrefix(parsed.Path, "/")
				require.NotEmpty(b, ids[j])
				require.NotContains(b, ids[j], "/")
				expected[shortURL] = originals[j]
			}
			require.Len(b, expected, urlCount)

			// 4. GET /api/user/urls: verify all 100 URLs belong to this user.
			body, _ = request(http.MethodGet, "/api/user/urls", "", "", http.StatusOK)
			var links []model.UserShortLinkResponse
			require.NoError(b, json.Unmarshal(body, &links))
			require.Len(b, links, urlCount)
			for _, link := range links {
				require.Equal(b, expected[link.ShortURL], link.OriginalURL)
				delete(expected, link.ShortURL)
			}
			require.Empty(b, expected)

			// 5. GET /{id}: resolve 10 distinct URLs without following redirects.
			for j := 0; j < followCount; j++ {
				_, header := request(http.MethodGet, "/"+ids[j], "", "", http.StatusTemporaryRedirect)
				require.Equal(b, originals[j], header.Get("Location"))
			}

			// 6. DELETE /api/user/urls: delete all 100 URLs.
			deleteBody, err := json.Marshal(ids)
			require.NoError(b, err)
			request(http.MethodDelete, "/api/user/urls", string(deleteBody), "application/json", http.StatusAccepted)
			b.StopTimer()

			// A 202 response only acknowledges asynchronous work. Poll outside
			// the measured scenario until this user's list is empty.
			deadline := time.Now().Add(10 * time.Second)
			for {
				res := serve(http.MethodGet, "/api/user/urls", "", "")
				if res.Code == http.StatusNoContent {
					break
				}
				require.Equal(b, http.StatusOK, res.Code)
				require.True(b, time.Now().Before(deadline), "all 100 URLs must be deleted within 10 seconds")
				time.Sleep(10 * time.Millisecond)
			}
			require.NoError(b, db.Close())
			b.StartTimer()
		}
	}
	b.StopTimer()
	b.ReportMetric(repetitions, "scenarios/op")
}
