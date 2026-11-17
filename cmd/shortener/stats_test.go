package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/snowflake"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/repository"
	"github.com/korzhev/yp-shorter/internal/service"
	"github.com/stretchr/testify/require"
)

func TestInternalStats(t *testing.T) {
	require.NoError(t, logger.InitLogger("error"))
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)
	db := repository.NewShortLinkDB("", "", config.InMemory)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Save(context.Background(), "one", "https://one.example", 1)
	require.NoError(t, err)
	_, err = db.Save(context.Background(), "two", "https://two.example", 1)
	require.NoError(t, err)
	_, err = db.Save(context.Background(), "three", "https://three.example", 2)
	require.NoError(t, err)
	for _, tt := range []struct {
		name, subnet string
		ips          []string
		status       int
	}{
		{"IPv4 allowed", "192.168.1.0/24", []string{"192.168.1.42"}, 200},
		{"network boundary", "192.168.1.0/24", []string{"192.168.1.0"}, 200},
		{"upper boundary", "192.168.1.0/24", []string{"192.168.1.255"}, 200},
		{"outside", "192.168.1.0/24", []string{"192.168.2.1"}, 403},
		{"empty subnet", "", []string{"192.168.1.42"}, 403},
		{"invalid subnet", "invalid", []string{"192.168.1.42"}, 403},
		{"missing header", "192.168.1.0/24", nil, 403},
		{"empty header", "192.168.1.0/24", []string{""}, 403},
		{"invalid IP", "192.168.1.0/24", []string{"invalid"}, 403},
		{"IP with port", "192.168.1.0/24", []string{"192.168.1.42:8080"}, 403},
		{"IP list", "192.168.1.0/24", []string{"192.168.1.42, 10.0.0.1"}, 403},
		{"repeated header", "192.168.1.0/24", []string{"192.168.1.42", "10.0.0.1"}, 403},
		{"IPv6 allowed", "2001:db8::/32", []string{"2001:db8::1"}, 200},
		{"IPv6 outside", "2001:db8::/32", []string{"2001:db9::1"}, 403},
		{"wrong family", "2001:db8::/32", []string{"192.168.1.42"}, 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := RootRouter(config.Config{TrustedSubnet: tt.subnet}, db, service.NewSemaphore(1), node, &service.AuditPublisher{})
			for _, invalidCookie := range []bool{false, true} {
				r := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
				r.RemoteAddr = "192.168.1.42:1234"
				r.Header.Set("X-Forwarded-For", "192.168.1.42")
				for _, ip := range tt.ips {
					r.Header.Add("X-Real-IP", ip)
				}
				if invalidCookie {
					r.AddCookie(&http.Cookie{Name: "Auth", Value: "invalid"})
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				require.Equal(t, tt.status, w.Code)
				if tt.status == http.StatusOK {
					require.Equal(t, "application/json", w.Header().Get("Content-Type"))
					require.JSONEq(t, `{"urls":3,"users":2}`, w.Body.String())
				}
			}
		})
	}
}
