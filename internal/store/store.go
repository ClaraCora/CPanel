package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound                     = errors.New("resource not found")
	ErrConflict                     = errors.New("resource conflict")
	ErrMachineHasNodes              = errors.New("machine still has nodes")
	ErrAccessGroupInUse             = errors.New("access group is still in use")
	ErrPlanInUse                    = errors.New("plan is still in use")
	ErrRoutePolicyInUse             = errors.New("route policy is still in use")
	ErrNodePortInUse                = errors.New("node port is already in use")
	ErrSubscriptionTokenUnavailable = errors.New("subscription token unavailable")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503", "23514":
			return errors.Join(ErrConflict, err)
		}
	}
	return err
}
