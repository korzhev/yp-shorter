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

type FileAudit struct {
	sync.Mutex
	File *os.File
}

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

func NewFileAudit(path string) (*FileAudit, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return &FileAudit{}, err
	}

	return &FileAudit{
		File: file,
	}, nil
}

type HTTPClient interface { // http.Client
	Do(req *http.Request) (*http.Response, error)
}

type HTTPAudit struct {
	URL    string
	Client HTTPClient
}

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

func NewHTTPAudit(url string) *HTTPAudit {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &HTTPAudit{
		Client: client,
		URL:    url,
	}
}
