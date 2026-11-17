package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServeUntilShutdownDrainsRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	shutdownStarted := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		if r.Context().Err() != nil {
			http.Error(w, "request cancelled", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "completed")
	})}
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	done := make(chan error, 1)
	go func() { done <- serveUntilShutdown(ctx, server, listener) }()
	t.Cleanup(func() { _ = server.Close() })
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	result := make(chan string, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			result <- err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		result <- string(body)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-shutdownStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before request completed: %v", err)
	default:
	}
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
		if conn != nil {
			_ = conn.Close()
		}
		return err != nil
	}, 5*time.Second, time.Millisecond)
	close(release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	require.Equal(t, "completed", <-result)
}

func TestServeUntilShutdownListenerError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	require.Error(t, serveUntilShutdown(context.Background(), &http.Server{}, listener))
}
