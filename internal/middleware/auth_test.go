package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v4"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBuildJWTString(t *testing.T) {
	const (
		userID = 42
		secret = "test-secret"
	)
	duration := 10 * time.Minute
	before := time.Now().Add(duration)

	tokenString, err := BuildJWTString(duration, userID, secret)

	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)
	assert.Equal(t, jwt.SigningMethodHS256, token.Method)
	assert.Equal(t, userID, claims.UserID)
	require.NotNil(t, claims.ExpiresAt)
	assert.WithinDuration(t, before, claims.ExpiresAt.Time, time.Second)
}

func TestGetUserID(t *testing.T) {
	useTestLogger(t)

	const (
		userID = 42
		secret = "test-secret"
	)

	validToken := mustBuildJWTString(t, time.Minute, userID, secret)
	expiredToken := mustBuildJWTString(t, -time.Minute, userID, secret)
	zeroUserIDToken := mustBuildJWTString(t, time.Minute, 0, secret)
	unexpectedMethodToken := mustBuildToken(t, jwt.SigningMethodHS384, userID, secret)

	tests := []struct {
		name       string
		token      string
		secret     string
		expectedID int
	}{
		{
			name:       "returns user ID from valid token",
			token:      validToken,
			secret:     secret,
			expectedID: userID,
		},
		{
			name:       "returns zero for wrong secret",
			token:      validToken,
			secret:     "wrong-secret",
			expectedID: 0,
		},
		{
			name:       "returns zero for expired token",
			token:      expiredToken,
			secret:     secret,
			expectedID: 0,
		},
		{
			name:       "returns zero for malformed token",
			token:      "not-a-jwt",
			secret:     secret,
			expectedID: 0,
		},
		{
			name:       "returns zero for unexpected signing method",
			token:      unexpectedMethodToken,
			secret:     secret,
			expectedID: 0,
		},
		{
			name:       "returns zero when token contains zero user ID",
			token:      zeroUserIDToken,
			secret:     secret,
			expectedID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedID, GetUserID(tt.token, tt.secret))
		})
	}
}

func TestNewAuthMiddleware(t *testing.T) {
	useTestLogger(t)

	const secret = "middleware-secret"
	c := config.Config{
		TokenExpMinutes: 10,
		TokenSecret:     secret,
	}
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	t.Run("creates user and auth cookie when request has no cookie", func(t *testing.T) {
		var contextUserID int
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			var ok bool
			contextUserID, ok = r.Context().Value(UserIDContextKey).(int)
			require.True(t, ok)
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		before := time.Now().Add(time.Duration(c.TokenExpMinutes) * time.Minute)

		NewAuthMiddleware(c, node)(next).ServeHTTP(rr, req)

		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "Auth", cookies[0].Name)
		assert.Equal(t, contextUserID, GetUserID(cookies[0].Value, secret))
		assert.WithinDuration(t, before, cookies[0].Expires, time.Second)
	})

	t.Run("uses user ID from valid auth cookie", func(t *testing.T) {
		userID := int(node.Generate().Int64())
		token := mustBuildJWTString(t, time.Minute, userID, secret)
		var contextUserID int
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ok bool
			contextUserID, ok = r.Context().Value(UserIDContextKey).(int)
			require.True(t, ok)
			w.WriteHeader(http.StatusAccepted)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "Auth", Value: token})
		rr := httptest.NewRecorder()

		NewAuthMiddleware(c, node)(next).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)
		assert.Equal(t, userID, contextUserID)
		assert.Empty(t, rr.Header().Values("Set-Cookie"))
	})

	t.Run("rejects invalid auth cookie without calling next handler", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "Auth", Value: "not-a-jwt"})
		rr := httptest.NewRecorder()

		NewAuthMiddleware(c, node)(next).ServeHTTP(rr, req)

		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Equal(t, "Invalid UserID in Cookie\n", rr.Body.String())
	})

	t.Run("rejects cookie with zero user ID", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})
		token := mustBuildJWTString(t, time.Minute, 0, secret)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "Auth", Value: token})
		rr := httptest.NewRecorder()

		NewAuthMiddleware(c, node)(next).ServeHTTP(rr, req)

		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func mustBuildJWTString(t *testing.T, duration time.Duration, userID int, secret string) string {
	t.Helper()

	token, err := BuildJWTString(duration, userID, secret)
	require.NoError(t, err)
	return token
}

func mustBuildToken(t *testing.T, method jwt.SigningMethod, userID int, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(method, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
		UserID: userID,
	})
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenString
}

func useTestLogger(t *testing.T) {
	t.Helper()

	original := logger.Log
	logger.Log = zap.NewNop().Sugar()
	t.Cleanup(func() {
		logger.Log = original
	})
}
