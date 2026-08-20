package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"order-service/internal/domain/order"
	"order-service/internal/usecase"
)

type OrderRepo struct {
	pool *pgxpool.Pool
}

func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{pool: pool}
}

func (r *OrderRepo) Save(ctx context.Context, o *order.Order) error {
	_, err := executor(ctx, r.pool).Exec(ctx, `
		INSERT INTO orders (id, user_id, product_id, quantity, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, pgUUID(o.ID()), o.UserID(), o.ProductID(), o.Quantity(), string(o.Status()), o.CreatedAt())
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	row := executor(ctx, r.pool).QueryRow(ctx, `
		SELECT id, user_id, product_id, quantity, status, created_at
		FROM orders
		WHERE id = $1
	`, pgUUID(id))

	var (
		oid       pgUUID
		userID    int64
		productID int64
		quantity  int
		status    string
		createdAt time.Time
	)
	if err := row.Scan(&oid, &userID, &productID, &quantity, &status, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, usecase.ErrOrderNotFound
		}
		return nil, fmt.Errorf("scan order: %w", err)
	}

	return order.Rehydrate(uuid.UUID(oid), userID, productID, quantity, order.Status(status), createdAt), nil
}
