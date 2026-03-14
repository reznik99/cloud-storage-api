package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newSilentLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// --- RateLimiter ---

func TestRateLimiter(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiter("2-M"))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	// First 2 requests should pass
	for i := range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, w.Code)
		}
	}

	// 3rd request should be rate-limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit request: got %d, want 429", w.Code)
	}
}

func TestRateLimiter_InvalidFormat(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("RateLimiter(invalid) did not panic")
		}
	}()
	RateLimiter("not-valid")
}

// --- ErrorHandler ---

func TestErrorHandler(t *testing.T) {
	r := gin.New()
	r.Use(ErrorHandler(newSilentLogger()))
	r.GET("/", func(c *gin.Context) {
		Abort(c, http.StatusBadRequest, errors.New("bad input"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("expected JSON error body, got empty")
	}
}

// --- MetricsHandler ---

func TestMetricsHandler(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		pass     string
		wantCode int
	}{
		{"valid creds", "admin", "secret", http.StatusOK},
		{"wrong password", "admin", "wrong", http.StatusUnauthorized},
		{"wrong username", "wrong", "secret", http.StatusUnauthorized},
		{"no creds", "", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("METRICS_CREDENTIALS", "admin:secret")

			r := gin.New()
			r.GET("/metrics", MetricsHandler())

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.user != "" || tt.pass != "" {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("got %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestMetricsHandler_InvalidCredentialFormat(t *testing.T) {
	t.Setenv("METRICS_CREDENTIALS", "no-colon-here")

	defer func() {
		if r := recover(); r == nil {
			t.Error("MetricsHandler() with bad METRICS_CREDENTIALS did not panic")
		}
	}()
	MetricsHandler()
}

// --- Protected ---

func TestProtected_WithSession(t *testing.T) {
	store := cookie.NewStore([]byte("test-secret-key!"))

	r := gin.New()
	r.Use(sessions.Sessions("test", store))
	r.GET("/login", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", int32(42))
		_ = s.Save()
		c.Status(http.StatusOK)
	})
	r.GET("/protected", Protected(func(c *gin.Context) {
		if c.Keys["user_id"] != int32(42) {
			t.Errorf("user_id = %v, want 42", c.Keys["user_id"])
		}
		c.Status(http.StatusOK)
	}))

	// Log in to get a session cookie
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, httptest.NewRequest(http.MethodGet, "/login", nil))

	// Use the cookie on the protected route
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	for _, c := range loginW.Result().Cookies() {
		req.AddCookie(c)
	}
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestProtected_WithoutSession(t *testing.T) {
	store := cookie.NewStore([]byte("test-secret-key!"))

	r := gin.New()
	r.Use(sessions.Sessions("test", store))
	r.GET("/protected", Protected(func(c *gin.Context) {
		c.Status(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}
