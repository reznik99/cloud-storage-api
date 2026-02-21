package middleware

import "github.com/gin-gonic/gin"

// Abort is a wrapper around gin's AbortWithError that discards the return value.
func Abort(c *gin.Context, code int, err error) {
	_ = c.AbortWithError(code, err)
}
