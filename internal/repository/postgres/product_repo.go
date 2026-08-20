package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"order-service/internal/domain/product"
	"order-service/internal/usecase"
)

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{pool: pool}
}

func (r *ProductRepo) GetByID(ctx context.Context, id int64) (*product.Product, error) {
	row := executor(ctx, r.pool).QueryRow(ctx, `
		SELECT id, name, price, stock
		FROM products
		WHERE id = $1
	`, id)

	var (
		pid   int64
		name  string
		price int64
		stock int
	)
	if err := row.Scan(&pid, &name, &price, &stock); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, usecase.ErrProductNotFound
		}
		return nil, fmt.Errorf("scan product: %w", err)
	}

	return product.Rehydrate(pid, name, price, stock), nil
}

func (r *ProductRepo) Lock(ctx context.Context, productID int64) error {
	if _, err := executor(ctx, r.pool).Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, productID); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	return nil
}

func (r *ProductRepo) DecrementStock(ctx context.Context, productID int64, qty int) error {
	tag, err := executor(ctx, r.pool).Exec(ctx, `
		UPDATE products
		SET stock = stock - $1
		WHERE id = $2 AND stock >= $1
	`, qty, productID)
	if err != nil {
		return fmt.Errorf("decrement stock: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// 0 затронутых строк означает одно из двух: товара с таким id нет,
		// либо stock < qty. Мы не различаем эти случаи здесь — GetByID уже
		// отдал ErrProductNotFound раньше в сценарии, если товара не было,
		// так что сюда попадаем только по нехватке остатка.
		return product.ErrOutOfStock
	}
	return nil
}
