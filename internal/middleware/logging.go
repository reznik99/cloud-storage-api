package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ErrorHandler is middleware that returns errors in structured JSON fromat
func ErrorHandler(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if err := c.Errors.Last(); err != nil {
			logger.Error(err)
			c.JSON(-1, gin.H{"message": err.Error()})
		}
	}
}

// LogHandler is middleware that logs response times
func LogHandler(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next() // Process request

		status := c.Writer.Status()
		clientIP := c.ClientIP()
		latency := time.Since(start)
		if status >= 500 {
			logger.Errorf("from: %s | took: %dms | %d %s %s", clientIP, latency.Milliseconds(), status, method, path)
		} else {
			logger.Infof("from: %s | took: %dms | %d %s %s", clientIP, latency.Milliseconds(), status, method, path)
		}
	}
}
