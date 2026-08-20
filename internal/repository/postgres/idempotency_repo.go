package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"order-service/internal/idempotency"
)

type IdempotencyRepo struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepo(pool *pgxpool.Pool) *IdempotencyRepo {
	return &IdempotencyRepo{pool: pool}
}

func (r *IdempotencyRepo) Reserve(ctx context.Context, key, requestHash string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO idempotency_keys (key, request_hash)
		VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING
	`, key, requestHash)
	if err != nil {
		return false, fmt.Errorf("reserve idempotency key: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *IdempotencyRepo) Get(ctx context.Context, key string) (*idempotency.Record, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body
		FROM idempotency_keys
		WHERE key = $1
	`, key)

	var rec idempotency.Record
	var status *int
	var body []byte
	if err := row.Scan(&rec.RequestHash, &status, &body); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("scan idempotency record: %w", err)
	}
	if status == nil {
		// Ключ зарезервирован, но ответ ещё не готов — для вызывающей стороны
		// (middleware) это тот же случай "уже обрабатывается", что и
		// reserved=false в Reserve.
		return nil, false, nil
	}
	rec.StatusCode = *status
	rec.Body = body
	return &rec, true, nil
}

func (r *IdempotencyRepo) Save(ctx context.Context, key string, statusCode int, body []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_status = $2, response_body = $3
		WHERE key = $1
	`, key, statusCode, body)
	if err != nil {
		return fmt.Errorf("save idempotency response: %w", err)
	}
	return nil
}
