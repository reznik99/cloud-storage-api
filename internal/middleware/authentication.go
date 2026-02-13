package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func Protected(next gin.HandlerFunc) gin.HandlerFunc {

	return func(c *gin.Context) {
		// Check if user has a cookie (authenticated)
		session := sessions.Default(c)
		userID, ok := session.Get("id").(int32)
		if !ok {
			c.AbortWithError(http.StatusUnauthorized, errors.New("unauthenticated"))
			return
		}
		// Populate request with session values
		c.Keys["user_id"] = userID

		next(c)
	}
}
