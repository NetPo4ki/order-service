package usecase

import "errors"

var (
	ErrProductNotFound = errors.New("usecase: product not found")
	ErrOrderNotFound   = errors.New("usecase: order not found")
)
