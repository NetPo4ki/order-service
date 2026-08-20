package httpapi

import (
	"net/http"

	"order-service/internal/delivery/http/middleware"
	"order-service/internal/idempotency"
	"order-service/internal/observability"
)

func NewRouter(orders *OrderHandler, idemStore idempotency.Store, limiter *middleware.RateLimiter) http.Handler {
	mux := http.NewServeMux()

	createOrder := middleware.Idempotency(idemStore)(http.HandlerFunc(orders.CreateOrder))
	mux.Handle("POST /orders", observability.InstrumentHTTP("/orders", createOrder))
	mux.Handle("GET /orders/{id}", observability.InstrumentHTTP("/orders/{id}", http.HandlerFunc(orders.GetOrder)))

	return limiter.Middleware(mux)
}
