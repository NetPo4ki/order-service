package grpcapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"order-service/internal/delivery/grpc/orderv1"
	"order-service/internal/domain/order"
	"order-service/internal/domain/product"
	"order-service/internal/usecase"
)

type OrderServer struct {
	orderv1.UnimplementedOrderServiceServer

	createOrder *usecase.CreateOrderUseCase
	getOrder    *usecase.GetOrderUseCase
}

func NewOrderServer(createOrder *usecase.CreateOrderUseCase, getOrder *usecase.GetOrderUseCase) *OrderServer {
	return &OrderServer{createOrder: createOrder, getOrder: getOrder}
}

func (s *OrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
	o, err := s.createOrder.Execute(ctx, usecase.CreateOrderCmd{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
		Quantity:  int(req.GetQuantity()),
	})
	if err != nil {
		return nil, mapUseCaseError(err)
	}
	return toProtoOrder(o), nil
}

func (s *OrderServer) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.Order, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order id")
	}

	o, err := s.getOrder.Execute(ctx, id)
	if err != nil {
		return nil, mapUseCaseError(err)
	}
	return toProtoOrder(o), nil
}

func toProtoOrder(o *order.Order) *orderv1.Order {
	return &orderv1.Order{
		Id:        o.ID().String(),
		UserId:    o.UserID(),
		ProductId: o.ProductID(),
		Quantity:  int32(o.Quantity()),
		Status:    string(o.Status()),
		CreatedAt: timestamppb.New(o.CreatedAt()),
	}
}

func mapUseCaseError(err error) error {
	switch {
	case errors.Is(err, product.ErrOutOfStock):
		return status.Error(codes.FailedPrecondition, "product is out of stock")
	case errors.Is(err, product.ErrInvalidQuantity), errors.Is(err, order.ErrInvalidQuantity):
		return status.Error(codes.InvalidArgument, "quantity must be positive")
	case errors.Is(err, usecase.ErrProductNotFound):
		return status.Error(codes.NotFound, "product not found")
	case errors.Is(err, usecase.ErrOrderNotFound):
		return status.Error(codes.NotFound, "order not found")
	default:
		slog.Error("unexpected error", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}
