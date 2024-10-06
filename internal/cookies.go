package internal

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// InitCors parses env variables for allowed cors origins and creates a cors config, then returns middleware func for gin
func (h *Handler) InitCors() gin.HandlerFunc {
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowCredentials: true,
		AllowHeaders:     []string{"content-type"},
		MaxAge:           12 * time.Hour,
	})
}

// InitCookieStore parses env variables for cookie key and duration, creates session storage and returns middleware func for gin
func (h *Handler) InitCookieStore() gin.HandlerFunc {
	cookieAuthKey := os.Getenv("COOKIE_AUTH_KEY")
	cookieDuration := os.Getenv("COOKIE_DURATION")
	dur, err := time.ParseDuration(cookieDuration)
	if err != nil {
		h.Logger.Fatalf("Invalid COOKIE_DURATION  '%s': %s", cookieDuration, err)
	}
	h.cookieDuration = int(dur.Seconds())

	store := cookie.NewStore([]byte(cookieAuthKey))
	return sessions.Sessions("gdrive", store)
}

// creates a standard cookie and writes it on this gin context
func (h *Handler) createCookie(c *gin.Context, id int32) {
	session := sessions.Default(c)
	session.Options(sessions.Options{
		Path:     "/",              // ?
		Domain:   c.Request.Host,   // Domain for which cookie should be sent
		MaxAge:   h.cookieDuration, // Lifespan of cookie
		Secure:   true,             // HTTPS only (except localhost)
		HttpOnly: true,             // Always true
		SameSite: 2,                // TODO: figure out ideal value
	})

	// Set session data
	session.Set("id", id)

	// Set-Cookie header for response
	session.Save()
}

// deletes the standard cookie for this gin context
func (h *Handler) destroyCookie(c *gin.Context) {
	session := sessions.Default(c)
	session.Options(sessions.Options{
		Path:     "/",
		Domain:   c.Request.Host,
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: 2,
	})
	session.Clear()
	session.Save()
}
