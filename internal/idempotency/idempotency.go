package idempotency

import "context"

type Record struct {
	RequestHash string
	StatusCode  int
	Body        []byte
}

type Store interface {
	Reserve(ctx context.Context, key, requestHash string) (reserved bool, err error)

	Get(ctx context.Context, key string) (rec *Record, found bool, err error)

	Save(ctx context.Context, key string, statusCode int, body []byte) error
}
