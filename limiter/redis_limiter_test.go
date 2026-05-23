package limiter_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-limiter/limiter"

	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s, skipping: %v", addr, err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestAllow_WithinLimit(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	key := "test:within:" + t.Name()
	rdb.Del(ctx, key)

	for i := 1; i <= 3; i++ {
		allowed, current, err := rl.Allow(ctx, key, 5, 60)
		if err != nil {
			t.Fatalf("Allow() error on request %d: %v", i, err)
		}
		if !allowed {
			t.Errorf("request %d: expected allowed=true, got false", i)
		}
		if current != int64(i) {
			t.Errorf("request %d: expected current=%d, got %d", i, i, current)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	key := "test:exceeds:" + t.Name()
	rdb.Del(ctx, key)
	const limit = 3

	for i := 1; i <= limit; i++ {
		allowed, _, err := rl.Allow(ctx, key, limit, 60)
		if err != nil {
			t.Fatalf("Allow() error on request %d: %v", i, err)
		}
		if !allowed {
			t.Errorf("request %d: expected allowed=true within limit", i)
		}
	}

	allowed, current, err := rl.Allow(ctx, key, limit, 60)
	if err != nil {
		t.Fatalf("Allow() error on exceeding request: %v", err)
	}
	if allowed {
		t.Errorf("expected allowed=false after exceeding limit, got true (current=%d)", current)
	}
}

func TestAllow_TTLIsSet(t *testing.T) {
	rdb := newTestClient(t)
	rl := limiter.NewRateLimiter(rdb)
	ctx := context.Background()

	key := "test:ttl:" + t.Name()
	rdb.Del(ctx, key)

	_, _, err := rl.Allow(ctx, key, 10, 2)
	if err != nil {
		t.Fatalf("Allow() error: %v", err)
	}

	ttl := rdb.TTL(ctx, key).Val()
	if ttl <= 0 {
		t.Errorf("expected TTL > 0, got %v", ttl)
	}
}
