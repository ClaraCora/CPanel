package store

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type requestTransactionKey struct{}

// RequestTransaction keeps all Store operations for one administrative request
// on one database transaction. Existing Begin calls become savepoints, so a
// successfully committed resource operation is still rolled back if its audit
// or the final idempotency result cannot be written.
type RequestTransaction struct {
	pool        *pgxpool.Pool
	tx          pgx.Tx
	mu          sync.Mutex
	err         error
	afterCommit []func()
}

func (s *Store) BeginRequest(ctx context.Context, options pgx.TxOptions) (context.Context, *RequestTransaction, error) {
	tx, err := s.pool.base.BeginTx(ctx, options)
	if err != nil {
		return ctx, nil, err
	}
	request := &RequestTransaction{pool: s.pool.base, tx: tx}
	return context.WithValue(ctx, requestTransactionKey{}, request), request, nil
}

func (r *RequestTransaction) Commit(ctx context.Context) error {
	if err := r.Error(); err != nil {
		return err
	}
	if err := r.tx.Commit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	callbacks := r.afterCommit
	r.afterCommit = nil
	r.mu.Unlock()
	for _, callback := range callbacks {
		callback()
	}
	return nil
}

// AfterCommit coordinates in-memory cache/session updates with the durable
// administrative transaction. Non-transactional callers retain immediate
// execution, while rolled-back requests never apply their callbacks.
func (s *Store) AfterCommit(ctx context.Context, callback func()) {
	if request := s.pool.request(ctx); request != nil {
		request.mu.Lock()
		request.afterCommit = append(request.afterCommit, callback)
		request.mu.Unlock()
		return
	}
	callback()
}

func (r *RequestTransaction) Error() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func IsRetryableTransactionError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01")
}

func markTransientFailure(ctx context.Context, err error) {
	if IsRetryableTransactionError(err) {
		markRequestFailure(ctx, err)
	}
}

func (r *RequestTransaction) Rollback(ctx context.Context) error { return r.tx.Rollback(ctx) }

func markRequestFailure(ctx context.Context, err error) {
	if err == nil {
		return
	}
	if request, ok := ctx.Value(requestTransactionKey{}).(*RequestTransaction); ok {
		request.mu.Lock()
		if request.err == nil {
			request.err = err
		}
		request.mu.Unlock()
	}
}

type requestPool struct{ base *pgxpool.Pool }

func (p *requestPool) request(ctx context.Context) *RequestTransaction {
	request, _ := ctx.Value(requestTransactionKey{}).(*RequestTransaction)
	if request != nil && request.pool == p.base {
		return request
	}
	return nil
}

func (p *requestPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if request := p.request(ctx); request != nil {
		tag, err := request.tx.Exec(ctx, sql, args...)
		markTransientFailure(ctx, err)
		return tag, err
	}
	return p.base.Exec(ctx, sql, args...)
}

func (p *requestPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if request := p.request(ctx); request != nil {
		rows, err := request.tx.Query(ctx, sql, args...)
		markTransientFailure(ctx, err)
		if err != nil {
			return nil, err
		}
		return &requestRows{Rows: rows, ctx: ctx}, nil
	}
	return p.base.Query(ctx, sql, args...)
}

func (p *requestPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if request := p.request(ctx); request != nil {
		return &requestRow{Row: request.tx.QueryRow(ctx, sql, args...), ctx: ctx}
	}
	return p.base.QueryRow(ctx, sql, args...)
}

func (p *requestPool) Begin(ctx context.Context) (pgx.Tx, error) {
	if request := p.request(ctx); request != nil {
		tx, err := request.tx.Begin(ctx)
		markTransientFailure(ctx, err)
		if err != nil {
			return nil, err
		}
		return &requestTx{Tx: tx}, nil
	}
	return p.base.Begin(ctx)
}

func (p *requestPool) Ping(ctx context.Context) error { return p.base.Ping(ctx) }

type requestTx struct{ pgx.Tx }

func (t *requestTx) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := t.Tx.Begin(ctx)
	markTransientFailure(ctx, err)
	if err != nil {
		return nil, err
	}
	return &requestTx{Tx: tx}, nil
}

func (t *requestTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := t.Tx.Exec(ctx, sql, args...)
	markTransientFailure(ctx, err)
	return tag, err
}

func (t *requestTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := t.Tx.Query(ctx, sql, args...)
	markTransientFailure(ctx, err)
	if err != nil {
		return nil, err
	}
	return &requestRows{Rows: rows, ctx: ctx}, nil
}

func (t *requestTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &requestRow{Row: t.Tx.QueryRow(ctx, sql, args...), ctx: ctx}
}

type requestRow struct {
	pgx.Row
	ctx context.Context
}

func (r *requestRow) Scan(values ...any) error {
	err := r.Row.Scan(values...)
	markTransientFailure(r.ctx, err)
	return err
}

type requestRows struct {
	pgx.Rows
	ctx context.Context
}

func (r *requestRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		markTransientFailure(r.ctx, r.Rows.Err())
	}
	return next
}

func (r *requestRows) Err() error {
	err := r.Rows.Err()
	markTransientFailure(r.ctx, err)
	return err
}
