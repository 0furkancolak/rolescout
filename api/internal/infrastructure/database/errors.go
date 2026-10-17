package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/rolescout/api/internal/domain"
)

// translate maps PostgreSQL errors to domain errors while keeping the original
// error in the chain for logging.
func translate(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: %w: %w", op, domain.ErrConflict, err)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%s: %w: %w", op, domain.ErrNotFound, err)
		case "23514", "23502", "22P02": // check, not_null, invalid_text_representation
			return fmt.Errorf("%s: %w: %w", op, domain.ErrInvalid, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// inTx runs fn in a transaction and commits only if fn succeeds.
func inTx(ctx context.Context, db txBeginner, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
