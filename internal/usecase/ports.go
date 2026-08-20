package usecase

import (
	"context"

	"github.com/google/uuid"

	"order-service/internal/domain/order"
	"order-service/internal/domain/product"
)

type ProductRepository interface {
	GetByID(ctx context.Context, id int64) (*product.Product, error)

	Lock(ctx context.Context, productID int64) error

	DecrementStock(ctx context.Context, productID int64, qty int) error
}

type OrderRepository interface {
	Save(ctx context.Context, o *order.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error)
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
