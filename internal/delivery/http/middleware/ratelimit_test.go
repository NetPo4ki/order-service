package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"order-service/internal/delivery/http/middleware"
)

func TestRateLimiter(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	const limit = 3
	const window = 200 * time.Millisecond

	rl := middleware.NewRateLimiter(rdb, limit, window)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(okHandler)

	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	t.Run("allows requests within limit", func(t *testing.T) {
		for i := 0; i < limit; i++ {
			rec := doRequest()
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
			}
		}
	})

	t.Run("rejects requests over limit with Retry-After", func(t *testing.T) {
		rec := doRequest()
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Fatal("expected Retry-After header on 429 response")
		}
	})

	t.Run("different client IP has its own independent limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.9:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 for a different client IP", rec.Code)
		}
	})

	t.Run("resets after the window expires", func(t *testing.T) {
		mr.FastForward(window + 10*time.Millisecond)

		rec := doRequest()
		if rec.Code != http.StatusOK {
			t.Fatalf("status after window reset = %d, want 200", rec.Code)
		}
	})
}

func TestRateLimiter_FailsOpenOnRedisOutage(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	rl := middleware.NewRateLimiter(rdb, 1, time.Second)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(okHandler)

	mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fail-open) when Redis is unreachable", rec.Code)
	}
}
