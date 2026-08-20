package order

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInvalidQuantity   = errors.New("order: quantity must be positive")
	ErrInvalidTransition = errors.New("order: invalid status transition")
)

type Order struct {
	id        uuid.UUID
	userID    int64
	productID int64
	quantity  int
	status    Status
	createdAt time.Time
}

func New(userID, productID int64, quantity int) (*Order, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	return &Order{
		id:        uuid.New(),
		userID:    userID,
		productID: productID,
		quantity:  quantity,
		status:    StatusPending,
		createdAt: time.Now().UTC(),
	}, nil
}

func Rehydrate(id uuid.UUID, userID, productID int64, quantity int, status Status, createdAt time.Time) *Order {
	return &Order{
		id:        id,
		userID:    userID,
		productID: productID,
		quantity:  quantity,
		status:    status,
		createdAt: createdAt,
	}
}

func (o *Order) ID() uuid.UUID        { return o.id }
func (o *Order) UserID() int64        { return o.userID }
func (o *Order) ProductID() int64     { return o.productID }
func (o *Order) Quantity() int        { return o.quantity }
func (o *Order) Status() Status       { return o.status }
func (o *Order) CreatedAt() time.Time { return o.createdAt }

func (o *Order) Confirm() error {
	if o.status != StatusPending {
		return ErrInvalidTransition
	}
	o.status = StatusConfirmed
	return nil
}

func (o *Order) Cancel() error {
	if o.status == StatusCancelled {
		return ErrInvalidTransition
	}
	o.status = StatusCancelled
	return nil
}
