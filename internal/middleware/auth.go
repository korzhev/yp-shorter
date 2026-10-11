package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v4"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
)

// Claims contains the JWT claims used to identify an authenticated user.
type Claims struct {
	// RegisteredClaims contains standard JWT metadata, including expiration.
	jwt.RegisteredClaims
	// UserID identifies the user associated with the token.
	UserID int `json:"user_id"`
}

// AuthMiddleware wraps a handler with cookie-based user authentication.
type AuthMiddleware func(next http.Handler) http.Handler

// CtxKey is the key type for middleware values stored in a request context.
type CtxKey string

// UserIDContextKey stores the authenticated user ID as an int in a request context.
const UserIDContextKey CtxKey = "ctxUserID"

// BuildJWTString signs an HS256 token for id using secret, expiring after d.
func BuildJWTString(d time.Duration, id int, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(d)),
		},
		UserID: id,
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserID verifies an HS256 token using secret and returns its user ID.
// It logs validation errors and returns zero for an invalid token.
func GetUserID(tokenString string, secret string) int {
	claims := &Claims{}
	// не понял когда указаль передавать, а когда нет
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf(
					"unexpected signing method: %s",
					t.Method.Alg(),
				)
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		logger.Log.Infow("Error in token", "error", err, "token", tokenString)
		return 0
	}

	if !token.Valid {
		logger.Log.Infow("Invalid token", "token", tokenString)
		return 0
	}

	return claims.UserID
}

// NewAuthMiddleware creates middleware that validates the Auth cookie and adds
// the user ID to the request context. If the cookie is missing, it generates an
// ID with node and sets a signed cookie. Invalid tokens result in 401 responses.
func NewAuthMiddleware(c config.Config, node *snowflake.Node) AuthMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("Auth")
			// no cookie
			if err != nil {
				// random user id 1-100
				newID := node.Generate()
				d := time.Minute * time.Duration(c.TokenExpMinutes)
				t, e := BuildJWTString(d, int(newID.Int64()), c.TokenSecret)
				if e != nil {
					http.Error(w, e.Error(), http.StatusInternalServerError)
					return
				}

				http.SetCookie(w, &http.Cookie{
					Name:    "Auth",
					Value:   t,
					Expires: time.Now().Add(d),
				})
				ctx := context.WithValue(r.Context(), UserIDContextKey, int(newID.Int64()))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			id := GetUserID(cookie.Value, c.TokenSecret)
			if id == 0 {
				http.Error(w, "Invalid UserID in Cookie", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
