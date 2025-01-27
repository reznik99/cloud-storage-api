package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	RequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_api_requests_total",
			Help: "Total number of requests processed by the storage-api.",
		},
		[]string{"path", "status"},
	)
	ErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_api_requests_errors_total",
			Help: "Total number of error requests processed by the storage-api.",
		},
		[]string{"path", "status"},
	)
)

func PrometheusInit() {
	prometheus.MustRegister(RequestCount)
	prometheus.MustRegister(ErrorCount)
}

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
		if status >= 400 {
			logger.Errorf("from: %s | took: %dms | %d %s %s", clientIP, latency.Milliseconds(), status, method, path)
			ErrorCount.WithLabelValues(path, http.StatusText(status)).Inc()
		} else {
			logger.Infof("from: %s | took: %dms | %d %s %s", clientIP, latency.Milliseconds(), status, method, path)
		}
		RequestCount.WithLabelValues(path, http.StatusText(status)).Inc()
	}
}

// Wraps the prometheus handler with basic auth
func MetricsHandler() gin.HandlerFunc {
	promHandler := promhttp.Handler()
	metricsPassword := os.Getenv("METRICS_PASSWORD")

	return func(c *gin.Context) {
		_, pass, ok := c.Request.BasicAuth()

		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(metricsPassword)) != 1 {
			c.AbortWithError(http.StatusUnauthorized, errors.New("Unauthorized"))
			return
		}

		promHandler.ServeHTTP(c.Writer, c.Request)
	}
}
