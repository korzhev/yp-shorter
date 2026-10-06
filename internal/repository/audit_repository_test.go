package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditHTTPClientFunc func(*http.Request) (*http.Response, error)

func (f auditHTTPClientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type auditResponseBody struct {
	io.Reader
	closed bool
}

func (b *auditResponseBody) Close() error {
	b.closed = true
	return nil
}

type auditErrorReader struct {
	err error
}

func (r auditErrorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func newTestFileAudit(t *testing.T, path string) *FileAudit {
	t.Helper()
	audit, err := NewFileAudit(path)
	require.NoError(t, err)
	require.NotNil(t, audit.File)
	t.Cleanup(func() {
		assert.NoError(t, audit.File.Close())
	})
	return audit
}

func TestToAuditLog(t *testing.T) {
	before := time.Now().Unix()
	data, err := toAuditLog("shorten", "user-42", "https://example.com")
	after := time.Now().Unix()
	require.NoError(t, err)

	var entry model.AuditLog
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "shorten", entry.Action)
	assert.Equal(t, "user-42", entry.UserID)
	assert.Equal(t, "https://example.com", entry.URL)
	assert.GreaterOrEqual(t, entry.TS, before)
	assert.LessOrEqual(t, entry.TS, after)
	assert.JSONEq(t, fmt.Sprintf(`{"ts":%d,"action":"shorten","user_id":"user-42","url":"https://example.com"}`, entry.TS), string(data))
}

func TestToAuditLogEscapesSpecialCharacters(t *testing.T) {
	action := "action\n\"quoted\""
	userID := "пользователь\\42"
	url := "https://example.com/?q=\"value\"&next=\n"
	data, err := toAuditLog(action, userID, url)
	require.NoError(t, err)

	var entry model.AuditLog
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, action, entry.Action)
	assert.Equal(t, userID, entry.UserID)
	assert.Equal(t, url, entry.URL)
	assert.NotContains(t, string(data), "\n")
}

func TestToAuditLogEmptyFields(t *testing.T) {
	data, err := toAuditLog("", "", "")
	require.NoError(t, err)
	var entry model.AuditLog
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Empty(t, entry.Action)
	assert.Empty(t, entry.UserID)
	assert.Empty(t, entry.URL)
	assert.Positive(t, entry.TS)
}

func TestNewFileAuditCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	audit := newTestFileAudit(t, path)
	assert.Equal(t, path, audit.File.Name())
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Zero(t, info.Size())
}

func TestNewFileAuditInvalidPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "audit.log")
	audit, err := NewFileAudit(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
	require.NotNil(t, audit)
	assert.Nil(t, audit.File)
}

func TestFileAuditSaveWritesJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	audit := newTestFileAudit(t, path)
	before := time.Now().Unix()
	require.NoError(t, audit.Save("shorten", "user-42", "https://example.com"))
	after := time.Now().Unix()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(data), "\n"))
	assert.Equal(t, 1, strings.Count(string(data), "\n"))
	var entry model.AuditLog
	require.NoError(t, json.Unmarshal(data, &entry))
	assert.Equal(t, "shorten", entry.Action)
	assert.Equal(t, "user-42", entry.UserID)
	assert.Equal(t, "https://example.com", entry.URL)
	assert.GreaterOrEqual(t, entry.TS, before)
	assert.LessOrEqual(t, entry.TS, after)
}

func TestFileAuditSaveAppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	existing := "{\"action\":\"existing\"}\n"
	require.NoError(t, os.WriteFile(path, []byte(existing), 0600))
	audit := newTestFileAudit(t, path)
	require.NoError(t, audit.Save("shorten", "first", "https://example.com/first"))
	require.NoError(t, audit.Save("follow", "second", "https://example.com/second"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(string(data), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, existing, lines[0]+"\n")
	assert.Empty(t, lines[3])
	var first, second model.AuditLog
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &second))
	assert.Equal(t, "shorten", first.Action)
	assert.Equal(t, "first", first.UserID)
	assert.Equal(t, "https://example.com/first", first.URL)
	assert.Equal(t, "follow", second.Action)
	assert.Equal(t, "second", second.UserID)
	assert.Equal(t, "https://example.com/second", second.URL)
}

func TestFileAuditSaveClosedFile(t *testing.T) {
	audit, err := NewFileAudit(filepath.Join(t.TempDir(), "audit.log"))
	require.NoError(t, err)
	require.NoError(t, audit.File.Close())
	assert.ErrorIs(t, audit.Save("shorten", "user-42", "https://example.com"), os.ErrClosed)
}

func TestFileAuditSaveConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	audit := newTestFileAudit(t, path)
	const count = 50
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			errs <- audit.Save("shorten", fmt.Sprint(id), "https://example.com")
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(string(data), "\n")
	require.Len(t, lines, count+1)
	assert.Empty(t, lines[count])
	seen := make(map[string]bool)
	for _, line := range lines[:count] {
		var entry model.AuditLog
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		assert.Equal(t, "shorten", entry.Action)
		assert.Equal(t, "https://example.com", entry.URL)
		assert.False(t, seen[entry.UserID], "duplicate user ID %s", entry.UserID)
		seen[entry.UserID] = true
	}
	for i := 0; i < count; i++ {
		assert.True(t, seen[fmt.Sprint(i)])
	}
}

func TestNewHTTPAudit(t *testing.T) {
	audit := NewHTTPAudit("https://audit.example.com/events")
	require.NotNil(t, audit)
	assert.Equal(t, "https://audit.example.com/events", audit.URL)
	client, ok := audit.Client.(*http.Client)
	require.True(t, ok)
	assert.Equal(t, 30*time.Second, client.Timeout)
}

func TestHTTPAuditSaveSendsJSON(t *testing.T) {
	body := &auditResponseBody{Reader: strings.NewReader("accepted")}
	called := false
	audit := &HTTPAudit{
		URL: "https://audit.example.com/events",
		Client: auditHTTPClientFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			defer req.Body.Close()
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, "https://audit.example.com/events", req.URL.String())
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			var entry model.AuditLog
			require.NoError(t, json.NewDecoder(req.Body).Decode(&entry))
			assert.Equal(t, "shorten", entry.Action)
			assert.Equal(t, "user-42", entry.UserID)
			assert.Equal(t, "https://example.com", entry.URL)
			assert.Positive(t, entry.TS)
			return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
		}),
	}
	require.NoError(t, audit.Save("shorten", "user-42", "https://example.com"))
	assert.True(t, called)
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveAcceptsUpperSuccessBoundary(t *testing.T) {
	body := &auditResponseBody{Reader: strings.NewReader("")}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 299, Body: body}, nil
	})}
	assert.NoError(t, audit.Save("follow", "user-42", "https://example.com"))
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveInvalidURL(t *testing.T) {
	audit := &HTTPAudit{URL: "://invalid", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("client must not be called for an invalid URL")
		return nil, nil
	})}
	assert.Error(t, audit.Save("shorten", "user-42", "https://example.com"))
}

func TestHTTPAuditSaveClientError(t *testing.T) {
	expectedErr := errors.New("connection failed")
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return nil, expectedErr
	})}
	assert.ErrorIs(t, audit.Save("shorten", "user-42", "https://example.com"), expectedErr)
}

func TestHTTPAuditSaveResponseReadError(t *testing.T) {
	expectedErr := errors.New("read failed")
	body := &auditResponseBody{Reader: auditErrorReader{err: expectedErr}}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})}
	assert.ErrorIs(t, audit.Save("shorten", "user-42", "https://example.com"), expectedErr)
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveRejectsStatusBelowOK(t *testing.T) {
	body := &auditResponseBody{Reader: strings.NewReader("informational response")}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 199, Body: body}, nil
	})}
	assert.EqualError(t, audit.Save("shorten", "user-42", "https://example.com"), "HttpError(199) response: informational response")
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveRejectsRedirect(t *testing.T) {
	body := &auditResponseBody{Reader: strings.NewReader("multiple choices")}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusMultipleChoices, Body: body}, nil
	})}
	assert.EqualError(t, audit.Save("shorten", "user-42", "https://example.com"), "HttpError(300) response: multiple choices")
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveServerError(t *testing.T) {
	body := &auditResponseBody{Reader: strings.NewReader("service unavailable")}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body}, nil
	})}
	assert.EqualError(t, audit.Save("shorten", "user-42", "https://example.com"), "HttpError(503) response: service unavailable")
	assert.True(t, body.closed)
}

func TestHTTPAuditSaveLimitsResponseBody(t *testing.T) {
	const limit = 1 << 20
	reader := strings.NewReader(strings.Repeat("x", limit) + "unread suffix")
	body := &auditResponseBody{Reader: reader}
	audit := &HTTPAudit{URL: "https://audit.example.com", Client: auditHTTPClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadRequest, Body: body}, nil
	})}
	err := audit.Save("shorten", "user-42", "https://example.com")
	require.Error(t, err)
	assert.Equal(t, "HttpError(400) response: "+strings.Repeat("x", limit), err.Error())
	assert.Equal(t, len("unread suffix"), reader.Len())
	assert.True(t, body.closed)
}
