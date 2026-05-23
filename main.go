package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"go-limiter/limiter"

	"github.com/redis/go-redis/v9"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
var Version = "dev"

func main() {
	log.Printf("go-limiter version %s starting", Version)

	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
	port := envOrDefault("PORT", "8080")
	rateLimit := envInt("RATE_LIMIT", 5)
	rateWindow := envDuration("RATE_LIMIT_WINDOW", 30*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Fatal: failed to connect to Redis at %s: %v", redisAddr, err)
	}
	log.Printf("Connected to Redis at %s", redisAddr)
	log.Printf("Rate limit: %d requests per %s", rateLimit, rateWindow)

	rl := limiter.NewRateLimiter(rdb)

	helloHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Request processed successfully.\n"))
	}

	http.HandleFunc("/data", rl.Middleware(rateLimit, rateWindow, helloHandler))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	server := &http.Server{
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}
