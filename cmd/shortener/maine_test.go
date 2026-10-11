package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// These examples require an application running at http://localhost:8080.
// Configure its base URL to the same address. The ping example also requires
// PostgreSQL. Output comments are intentionally omitted: go test compiles these
// examples but does not execute requests against an external server.
const exampleServerURL = "http://localhost:8080"

func newExampleHTTPClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &http.Client{
		Jar:     jar, // Preserve the Auth cookie issued by the server.
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Inspect 307 without visiting the original URL.
		},
	}
}

// exampleCreateURL creates a link owned by this client. A unique original URL
// avoids reusing a link that belongs to a previous run's user.
func exampleCreateURL(client *http.Client) string {
	original := fmt.Sprintf("https://example.com/article?example=%d", time.Now().UnixNano())
	res, err := client.Post(exampleServerURL+"/", "text/plain", strings.NewReader(original))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusCreated {
		panic(fmt.Sprintf("create link: %s: %s", res.Status, body))
	}
	return string(body)
}

// POST / shortens a plain-text URL. Repeating an existing URL returns 409
// with the existing short URL instead of 201 with a new one.
func ExampleRootRouter_shortenText() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	res, err := client.Post(exampleServerURL+"/", "text/plain", strings.NewReader("https://example.com/article"))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Status)
	fmt.Println(string(body))
}

// GET /{id} returns 307 with the original URL in the Location header.
func ExampleRootRouter_redirect() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	shortURL := exampleCreateURL(client)
	res, err := client.Get(shortURL)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	fmt.Println(res.Status)
	fmt.Println(res.Header.Get("Location"))
}

// POST /api/shorten accepts {"url":"..."} and returns {"result":"..."}.
func ExampleRootRouter_shortenJSON() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	res, err := client.Post(exampleServerURL+"/api/shorten", "application/json",
		strings.NewReader(`{"url":"https://example.com/article"}`))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
		panic(res.Status)
	}
	var link struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&link); err != nil {
		panic(err)
	}
	fmt.Println(res.Status)
	fmt.Println(link.Result)
}

// POST /api/shorten/batch returns short URLs with matching correlation IDs.
func ExampleRootRouter_shortenBatch() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	runID := time.Now().UnixNano()
	body := fmt.Sprintf(`[
		{"correlation_id":"first","original_url":"https://example.com/first?example=%d"},
		{"correlation_id":"second","original_url":"https://example.com/second?example=%d"}
	]`, runID, runID)
	res, err := client.Post(exampleServerURL+"/api/shorten/batch", "application/json", strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		panic(res.Status)
	}
	var links []struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&links); err != nil {
		panic(err)
	}
	fmt.Println(res.Status)
	for _, link := range links {
		fmt.Println(link.CorrelationID, link.ShortURL)
	}
}

// GET /api/user/urls lists links owned by the user identified by the Auth cookie.
func ExampleRootRouter_userURLs() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	exampleCreateURL(client)
	// The same client's cookie jar automatically sends the Auth cookie.
	res, err := client.Get(exampleServerURL + "/api/user/urls")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	fmt.Println(res.Status)
	if res.StatusCode == http.StatusNoContent {
		return // No links; the response has no JSON body.
	}
	if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}
	var links []struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&links); err != nil {
		panic(err)
	}
	for _, link := range links {
		fmt.Println(link.ShortURL, link.OriginalURL)
	}
}

// DELETE /api/user/urls accepts a JSON array of short IDs, not full URLs.
// Use the owner's Auth cookie. A 202 response acknowledges asynchronous deletion
// and does not guarantee that the links have already disappeared from the list.
func ExampleRootRouter_deleteUserURLs() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	shortURL, err := url.Parse(exampleCreateURL(client))
	if err != nil {
		panic(err)
	}
	ids := []string{strings.TrimPrefix(shortURL.Path, "/")}
	body, err := json.Marshal(ids)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodDelete, exampleServerURL+"/api/user/urls", strings.NewReader(string(body)))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	fmt.Println(res.Status)
}

// GET /ping checks PostgreSQL connectivity and returns 200 on success.
// Start the application with a database DSN before using this endpoint.
func ExampleRootRouter_ping() {
	client := newExampleHTTPClient()
	defer client.CloseIdleConnections()
	res, err := client.Get(exampleServerURL + "/ping")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	fmt.Println(res.Status)
}
