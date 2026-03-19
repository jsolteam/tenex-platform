package database

import (
	"context"
	"database/sql"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type TxFunc func(ctx context.Context, tx *sql.Tx) error

func WithTx(ctx context.Context, db *sql.DB, fn TxFunc) (retErr error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.DBConnect("database.WithTx.begin", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.DB("database.WithTx.commit", err)
	}
	return nil
}

func WithTxOpts(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn TxFunc) (retErr error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return apperrors.DBConnect("database.WithTxOpts.begin", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.DB("database.WithTxOpts.commit", err)
	}
	return nil
}
