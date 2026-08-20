//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"order-service/internal/domain/product"
	repopg "order-service/internal/repository/postgres"
	"order-service/internal/usecase"
)

func TestCreateOrderUseCase_LastItemRace(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("orders_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts("../../../migrations/000001_init_schema.up.sql"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	var productID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id
	`, "Last Chair", 1999, 1).Scan(&productID)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}

	productRepo := repopg.NewProductRepo(pool)
	orderRepo := repopg.NewOrderRepo(pool)
	txManager := repopg.NewTxManager(pool)
	uc := usecase.NewCreateOrderUseCase(productRepo, orderRepo, txManager)

	const concurrency = 30

	var (
		wg           sync.WaitGroup
		successCount int64
		outOfStock   int64
		otherErrors  int64
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()

			_, err := uc.Execute(ctx, usecase.CreateOrderCmd{
				UserID:    userID,
				ProductID: productID,
				Quantity:  1,
			})
			switch {
			case err == nil:
				atomic.AddInt64(&successCount, 1)
			case errors.Is(err, product.ErrOutOfStock):
				atomic.AddInt64(&outOfStock, 1)
			default:
				atomic.AddInt64(&otherErrors, 1)
				t.Logf("unexpected error: %v", err)
			}
		}(int64(i + 1))
	}
	wg.Wait()

	if otherErrors != 0 {
		t.Fatalf("got %d unexpected errors, want 0", otherErrors)
	}
	if successCount != 1 {
		t.Fatalf("successCount = %d, want exactly 1 — double-booking protection failed", successCount)
	}
	if outOfStock != concurrency-1 {
		t.Fatalf("outOfStock = %d, want %d", outOfStock, concurrency-1)
	}

	var finalStock int
	if err := pool.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&finalStock); err != nil {
		t.Fatalf("select final stock: %v", err)
	}
	if finalStock != 0 {
		t.Fatalf("final stock = %d, want 0", finalStock)
	}

	var orderCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE product_id = $1`, productID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 1 {
		t.Fatalf("orderCount = %d, want 1", orderCount)
	}
}
