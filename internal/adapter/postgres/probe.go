// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Probe checks that the server accepts the connection string, credentials
// included, for orcon config check --probe. It opens one authenticated
// connection and runs SELECT 1 in a read-only transaction that it rolls back, so
// it writes nothing. The error never repeats the connection string.
func Probe(ctx context.Context, dsn string) error {
	if dsn == "" {
		return errors.New("probe postgres: the connection string is empty")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("probe postgres: the connection string cannot be parsed") //nolint:err113 // the parse error can quote the string
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("probe postgres: connect: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }() // read-only probe: nothing to lose
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("probe postgres: begin read-only transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // read-only: the rollback ends it
	var one int
	if err := tx.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("probe postgres: SELECT 1: %w", err)
	}
	return nil
}
