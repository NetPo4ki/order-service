package usecase

import (
	"context"
	"fmt"

	"order-service/internal/domain/order"
)

type CreateOrderCmd struct {
	UserID    int64
	ProductID int64
	Quantity  int
}

type CreateOrderUseCase struct {
	products ProductRepository
	orders   OrderRepository
	txs      TxManager
}

func NewCreateOrderUseCase(products ProductRepository, orders OrderRepository, txs TxManager) *CreateOrderUseCase {
	return &CreateOrderUseCase{products: products, orders: orders, txs: txs}
}

func (uc *CreateOrderUseCase) Execute(ctx context.Context, cmd CreateOrderCmd) (*order.Order, error) {
	var result *order.Order

	err := uc.txs.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.products.Lock(ctx, cmd.ProductID); err != nil {
			return fmt.Errorf("lock product: %w", err)
		}

		p, err := uc.products.GetByID(ctx, cmd.ProductID)
		if err != nil {
			return fmt.Errorf("get product: %w", err)
		}
		if err := p.CanReserve(cmd.Quantity); err != nil {
			return err
		}

		if err := uc.products.DecrementStock(ctx, cmd.ProductID, cmd.Quantity); err != nil {
			return fmt.Errorf("decrement stock: %w", err)
		}

		o, err := order.New(cmd.UserID, cmd.ProductID, cmd.Quantity)
		if err != nil {
			return err
		}
		if err := uc.orders.Save(ctx, o); err != nil {
			return fmt.Errorf("save order: %w", err)
		}

		result = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
