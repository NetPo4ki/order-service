package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"order-service/internal/delivery/http/response"
	"order-service/internal/domain/order"
	"order-service/internal/domain/product"
	"order-service/internal/usecase"
)

type OrderHandler struct {
	createOrder *usecase.CreateOrderUseCase
	getOrder    *usecase.GetOrderUseCase
}

func NewOrderHandler(createOrder *usecase.CreateOrderUseCase, getOrder *usecase.GetOrderUseCase) *OrderHandler {
	return &OrderHandler{createOrder: createOrder, getOrder: getOrder}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	o, err := h.createOrder.Execute(r.Context(), usecase.CreateOrderCmd{
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	})
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, toOrderResponse(o))
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	o, err := h.getOrder.Execute(r.Context(), id)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, toOrderResponse(o))
}

func toOrderResponse(o *order.Order) OrderResponse {
	return OrderResponse{
		ID:        o.ID().String(),
		UserID:    o.UserID(),
		ProductID: o.ProductID(),
		Quantity:  o.Quantity(),
		Status:    string(o.Status()),
		CreatedAt: o.CreatedAt(),
	}
}

func writeUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, product.ErrOutOfStock):
		response.WriteError(w, http.StatusConflict, "product is out of stock")
	case errors.Is(err, product.ErrInvalidQuantity), errors.Is(err, order.ErrInvalidQuantity):
		response.WriteError(w, http.StatusBadRequest, "quantity must be positive")
	case errors.Is(err, usecase.ErrProductNotFound):
		response.WriteError(w, http.StatusNotFound, "product not found")
	case errors.Is(err, usecase.ErrOrderNotFound):
		response.WriteError(w, http.StatusNotFound, "order not found")
	default:
		slog.Error("unexpected error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
