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

type Claims struct {
	jwt.RegisteredClaims
	UserID int `json:"user_id"`
}

type AuthMiddleware func(next http.Handler) http.Handler

type CtxKey string

const UserIDContextKey CtxKey = "ctxUserID"

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

func NewAuthMiddleware(c config.Config, node *snowflake.Node) AuthMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("Auth")
			// no cookie
			if err != nil {
				// random user id 1-100
				newId := node.Generate()
				d := time.Minute * time.Duration(c.TokenExpMinutes)
				t, e := BuildJWTString(d, int(newId.Int64()), c.TokenSecret)
				if e != nil {
					http.Error(w, e.Error(), http.StatusInternalServerError)
					return
				}

				http.SetCookie(w, &http.Cookie{
					Name:    "Auth",
					Value:   t,
					Expires: time.Now().Add(d),
				})
				ctx := context.WithValue(r.Context(), UserIDContextKey, int(newId.Int64()))
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
