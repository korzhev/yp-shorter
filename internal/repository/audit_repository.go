package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/korzhev/yp-shorter/internal/model"
)

func toAuditLog(action, userID, url string) ([]byte, error) {
	al := model.AuditLog{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    url,
	}
	return json.Marshal(al)
}

// FileAudit writes audit events as newline-delimited JSON to a file.
// It must not be copied after first use.
type FileAudit struct {
	// Mutex serializes writes to File.
	sync.Mutex
	// File is the audit output file; its owner is responsible for closing it.
	File *os.File
}

// Save appends a timestamped audit event and a newline to the file.
// Concurrent calls are serialized by the embedded mutex.
func (af *FileAudit) Save(action, userID, url string) error {
	b, err := toAuditLog(action, userID, url)
	if err != nil {
		return err
	}

	af.Lock()
	defer af.Unlock()
	_, err = af.File.Write(append(b, '\n'))
	if err != nil {
		return err
	}
	return nil
}

// NewFileAudit opens or creates path for appending audit events.
// The caller must close the returned File after successful use.
func NewFileAudit(path string) (*FileAudit, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return &FileAudit{}, err
	}

	return &FileAudit{
		File: file,
	}, nil
}

// HTTPClient executes HTTP requests, as implemented by http.Client.
type HTTPClient interface { // http.Client
	// Do sends req and returns its response or a transport error.
	Do(req *http.Request) (*http.Response, error)
}

// HTTPAudit sends JSON audit events to an HTTP endpoint.
type HTTPAudit struct {
	// URL is the endpoint that receives audit events.
	URL string
	// Client executes audit HTTP requests.
	Client HTTPClient
}

// Save posts a timestamped JSON audit event to URL.
// It returns an error on request or response-reading failures and non-2xx statuses.
func (ha *HTTPAudit) Save(action, userID, url string) error {
	b, err := toAuditLog(action, userID, url)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ha.URL,
		bytes.NewReader(b),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ha.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HttpError(%v) response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// NewHTTPAudit creates an HTTP audit sink with a 30-second client timeout.
func NewHTTPAudit(url string) *HTTPAudit {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &HTTPAudit{
		Client: client,
		URL:    url,
	}
}
