// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateDatabase creates an empty database named name on the server that adminDSN reaches. Test drivers
// of other packages use it to give each case a fresh database; the caller generates the name, which must
// be a plain identifier (CREATE DATABASE takes no parameter).
func CreateDatabase(ctx context.Context, adminDSN, name string) error {
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("connect to the administrative database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0"); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	return nil
}

// DropDatabase removes the database named name, with its connections, on the server that adminDSN reaches.
func DropDatabase(ctx context.Context, adminDSN, name string) error {
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("connect to the administrative database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		return fmt.Errorf("drop database %s: %w", name, err)
	}
	return nil
}
