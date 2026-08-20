package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type pgUUID uuid.UUID

func (u pgUUID) UUIDValue() (pgtype.UUID, error) {
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

func (u *pgUUID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		*u = pgUUID{}
		return nil
	}
	*u = pgUUID(v.Bytes)
	return nil
}
