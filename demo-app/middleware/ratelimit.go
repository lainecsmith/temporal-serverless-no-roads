package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// ipRateLimiter holds a per-IP limiter map.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newIPRateLimiter(r rate.Limit, b int) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        r,
		b:        b,
	}
}

func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	l, exists := i.limiters[ip]
	if !exists {
		l = rate.NewLimiter(i.r, i.b)
		i.limiters[ip] = l
	}
	return l
}

// Global limiter: 5 requests per minute per IP, burst of 5.
var limiter = newIPRateLimiter(rate.Every(60/5), 5)

// RateLimit is an HTTP middleware that enforces per-IP rate limiting on POST
// requests. GET requests (metrics polling) pass through without limiting.
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			ip := r.RemoteAddr
			if !limiter.getLimiter(ip).Allow() {
				http.Error(w, "rate limit exceeded — slow down!", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
