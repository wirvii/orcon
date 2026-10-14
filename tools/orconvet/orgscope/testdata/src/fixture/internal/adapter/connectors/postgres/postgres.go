// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package postgres stands for a connector that queries the user's own database.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Run is allowed to call the driver (the statement is the user's, not Orcon's).
func Run(ctx context.Context, conn *pgx.Conn) error {
	return conn.Exec(ctx, "SELECT 1")
}
