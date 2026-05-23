package limiter

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

// realIP resolves the client IP from proxy headers before falling back to RemoteAddr.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be "client, proxy1, proxy2" — take the first entry.
		if host, _, err := net.SplitHostPort(xff); err == nil {
			return host
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *RateLimiter) Middleware(limit int, window time.Duration, next http.HandlerFunc) http.HandlerFunc {
	ttlSeconds := int(window.Seconds())

	return func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		key := "rate_limit:" + ip
		allowed, current, err := rl.Allow(r.Context(), key, limit, ttlSeconds)
		if err != nil {
			log.Printf("Rate limiter error for IP %s: %v", ip, err)
			next.ServeHTTP(w, r)
			return
		}

		remaining := int64(limit) - current
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if !allowed {
			log.Printf("Rate limit exceeded for IP: %s (%d/%d)", ip, current, limit)
			w.Header().Set("Retry-After", strconv.Itoa(ttlSeconds))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limit exceeded. Please try again later.\n"))
			return
		}

		log.Printf("Request allowed for IP: %s (%d/%d)", ip, current, limit)
		next.ServeHTTP(w, r)
	}
}
