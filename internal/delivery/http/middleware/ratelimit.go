package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"order-service/internal/delivery/http/response"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

type RateLimiter struct {
	rdb    *redis.Client
	limit  int64
	window time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int64, window time.Duration) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "ratelimit:" + clientIP(r)

		current, err := rateLimitScript.Run(r.Context(), rl.rdb, []string{key}, rl.window.Milliseconds()).Int64()
		if err != nil {
			slog.Warn("rate limiter: redis error, failing open", "error", err)
			next.ServeHTTP(w, r)
			return
		}

		if current > rl.limit {
			if ttl, err := rl.rdb.PTTL(r.Context(), key).Result(); err == nil && ttl > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(ttl.Seconds())+1))
			}
			response.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
