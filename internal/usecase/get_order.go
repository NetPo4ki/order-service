package usecase

import (
	"context"

	"github.com/google/uuid"

	"order-service/internal/domain/order"
)

type GetOrderUseCase struct {
	orders OrderRepository
}

func NewGetOrderUseCase(orders OrderRepository) *GetOrderUseCase {
	return &GetOrderUseCase{orders: orders}
}

func (uc *GetOrderUseCase) Execute(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	return uc.orders.GetByID(ctx, id)
}
