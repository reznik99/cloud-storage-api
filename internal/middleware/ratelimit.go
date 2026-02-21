package middleware

import (
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiter returns a Gin middleware that rate-limits requests by client IP.
// Format examples: "5-M" (5/min), "10-H" (10/hour), "1-S" (1/sec).
func RateLimiter(formatted string) gin.HandlerFunc {
	rate, err := limiter.NewRateFromFormatted(formatted)
	if err != nil {
		panic("ratelimit: invalid rate format: " + formatted)
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return mgin.NewMiddleware(instance)
}
