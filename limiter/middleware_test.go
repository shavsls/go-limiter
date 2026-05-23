package limiter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go-limiter/limiter"
)

func TestMiddleware_AllowedRequest(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	key := "rate_limit:127.0.0.1"
	rdb.Del(ctx, key)

	handler := rl.Middleware(5, 30*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("expected X-RateLimit-Limit=5, got %q", rec.Header().Get("X-RateLimit-Limit"))
	}
}

func TestMiddleware_RateLimitExceeded(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	const limit = 3
	key := "rate_limit:10.0.0.1"
	rdb.Del(ctx, key)

	handler := rl.Middleware(limit, 30*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set on 429")
	}
}

func TestMiddleware_XRateLimitHeaders(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	key := "rate_limit:192.168.1.1"
	rdb.Del(ctx, key)

	handler := rl.Middleware(10, 60*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.RemoteAddr = "192.168.1.1:9999"
	rec := httptest.NewRecorder()
	handler(rec, req)

	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")

	if limitHeader != "10" {
		t.Errorf("X-RateLimit-Limit: expected 10, got %q", limitHeader)
	}

	remaining, err := strconv.Atoi(remainingHeader)
	if err != nil {
		t.Fatalf("X-RateLimit-Remaining is not an integer: %q", remainingHeader)
	}
	if remaining != 9 {
		t.Errorf("X-RateLimit-Remaining: expected 9, got %d", remaining)
	}
}

func TestMiddleware_RealIPFromHeader(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	realIP := "203.0.113.1"
	key := "rate_limit:" + realIP
	rdb.Del(ctx, key)

	handler := rl.Middleware(5, 30*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("X-Real-IP", realIP)
	req.RemoteAddr = "10.0.0.1:80"
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	remaining := rdb.Get(ctx, key).Val()
	if remaining != "1" {
		t.Errorf("expected counter keyed by X-Real-IP=%s, got Redis key value %q", realIP, remaining)
	}
}
