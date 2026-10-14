// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package areaports declares the store ports of an area for the port rule.
package areaports

import (
	"context"

	"fixture/internal/platform/orgscope"
)

// Scoped takes the organization scope in every method.
type Scoped interface {
	// Get reads a row of the organization.
	Get(ctx context.Context, org orgscope.Org, id string) (string, error)
	// Put writes a row of the organization.
	Put(ctx context.Context, org orgscope.Org, id string) error
}

// Instance holds instance tables and says so.
type Instance interface {
	// Create inserts a user.
	//
	//orcon:scope instance application_user is an instance table
	Create(ctx context.Context, email string) error
	// Sweep clears the actor columns of every organization.
	//
	//orcon:scope engine the columns to clear are in every organization
	Sweep(ctx context.Context, batch int) error
}

// Unscoped forgets the scope in some methods.
type Unscoped interface {
	// Leak has neither a scope nor a reason.
	Leak(ctx context.Context, id string) (string, error)
	// NoReason names the directive but gives no reason.
	//
	//orcon:scope instance
	NoReason(ctx context.Context) error
	// WrongKind names a kind that is not allowed.
	//
	//orcon:scope org because I say so
	WrongKind(ctx context.Context) error
	// Fine is scoped, so one scoped method next to the others changes nothing.
	Fine(ctx context.Context, org orgscope.Org) error
}

// Embedded gets the methods of Scoped and Unscoped.
type Embedded interface {
	Scoped
	Unscoped
}
