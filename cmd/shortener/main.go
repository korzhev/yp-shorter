package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var urlMap = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

func GenerateRandomString(length int) string {
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

func getId() string {
	id := GenerateRandomString(6)
	urlMap.RLock()
	_, ok := urlMap.m[id]
	urlMap.RUnlock()
	if ok {
		id = getId()
	}
	return id
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if string(body) == "" {
			http.Error(w, "Empty body", http.StatusBadRequest)
			return
		}

		fmt.Printf("Body: %v\n", body)
		w.WriteHeader(http.StatusCreated)
		id := getId()
		urlMap.Lock()
		urlMap.m[id] = string(body)
		urlMap.Unlock()

		link := fmt.Sprintf("http://localhost:8080/%s", id)
		w.Write([]byte(link))
		return
	}

	if r.Method == http.MethodGet && len(r.URL.Path) > 1 {
		fmt.Printf("MethodGet: %v\n", r.Method)
		short := strings.TrimPrefix(r.URL.Path, "/")
		fmt.Printf("Id2: %v\n", short)
		if short != "" {
			urlMap.RLock()
			l, ok := urlMap.m[short]
			urlMap.RUnlock()
			fmt.Printf("l, ok: %v %v\n", l, ok)
			if ok {
				w.Header().Set("Location", l)
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
			}

		}
	}

	w.WriteHeader(http.StatusBadRequest)
}

func main() {
	mux := http.NewServeMux()

	// Handle POST /
	mux.HandleFunc("/", rootHandler)

	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
