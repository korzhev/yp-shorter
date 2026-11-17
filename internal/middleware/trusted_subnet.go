package middleware

import (
	"net"
	"net/http"
)

// NewTrustedSubnetMiddleware restricts access by X-Real-IP. Empty or invalid CIDRs
// and missing, malformed or repeated IP headers deny access.
func NewTrustedSubnetMiddleware(cidr string) func(http.Handler) http.Handler {
	_, subnet, err := net.ParseCIDR(cidr)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			values := r.Header.Values("X-Real-IP")
			if err != nil || len(values) != 1 || !subnet.Contains(net.ParseIP(values[0])) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
