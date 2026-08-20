package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"order-service/internal/config"
	grpcapi "order-service/internal/delivery/grpc"
	"order-service/internal/delivery/grpc/orderv1"
	httpapi "order-service/internal/delivery/http"
	"order-service/internal/delivery/http/middleware"
	infrapg "order-service/internal/infra/postgres"
	inforedis "order-service/internal/infra/redis"
	"order-service/internal/observability"
	repopg "order-service/internal/repository/postgres"
	"order-service/internal/usecase"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ready := observability.NewReadiness()
	adminSrv := observability.NewAdminServer(cfg.AdminAddr, ready)

	pool, err := infrapg.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := inforedis.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("connect to redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Warn("close redis", "error", err)
		}
	}()

	productRepo := repopg.NewProductRepo(pool)
	orderRepo := repopg.NewOrderRepo(pool)
	idemRepo := repopg.NewIdempotencyRepo(pool)
	txManager := repopg.NewTxManager(pool)

	createOrder := usecase.NewCreateOrderUseCase(productRepo, orderRepo, txManager)
	getOrder := usecase.NewGetOrderUseCase(orderRepo)

	orderHandler := httpapi.NewOrderHandler(createOrder, getOrder)
	rateLimiter := middleware.NewRateLimiter(rdb, int64(cfg.RateLimitRequests), cfg.RateLimitWindow)
	router := httpapi.NewRouter(orderHandler, idemRepo, rateLimiter)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Error("listen grpc", "error", err)
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(observability.UnaryServerInterceptor()))
	orderv1.RegisterOrderServiceServer(grpcSrv, grpcapi.NewOrderServer(createOrder, getOrder))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	reflection.Register(grpcSrv)

	ready.SetReady(true)

	serverErr := make(chan error, 3)
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()
	go func() {
		log.Info("grpc listening", "addr", cfg.GRPCAddr)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()
	go func() {
		log.Info("admin listening", "addr", cfg.AdminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
		return
	}

	ready.SetReady(false)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Warn("http: forced shutdown after timeout", "error", err)
		}
	}()

	go func() {
		defer wg.Done()
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-shutdownCtx.Done():
			log.Warn("grpc: forced stop after timeout")
			grpcSrv.Stop()
		}
	}()

	go func() {
		defer wg.Done()
		if err := adminSrv.Shutdown(shutdownCtx); err != nil {
			log.Warn("admin: forced shutdown after timeout", "error", err)
		}
	}()

	wg.Wait()
	log.Info("shutdown complete")
}
