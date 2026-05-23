package limiter

import (
	"context"
	"github.com/redis/go-redis/v9"
)

// luaScript atomically increments the counter, sets TTL on first hit, and returns the current count.
const luaScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl   = tonumber(ARGV[2])

local current = redis.call("INCR", key)
if current == 1 then
    redis.call("EXPIRE", key, ttl)
end
return current
`

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow increments the counter for key and returns whether the request is within limit.
// ttlSeconds sets the expiry window for the counter key.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, ttlSeconds int) (allowed bool, current int64, err error) {
	res, err := r.client.Eval(ctx, luaScript, []string{key}, limit, ttlSeconds).Result()
	if err != nil {
		return false, 0, err
	}
	current = res.(int64)
	return current <= int64(limit), current, nil
}
