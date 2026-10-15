// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package ports asserts the ports its store implements; the port rule reads the assertions.
package ports

import (
	"context"
	"io"

	"fixture/internal/areaports"
	"fixture/internal/platform/orgscope"
)

// Store implements every port of the fixture area, whether or not the rule likes it.
type Store struct{}

var (
	_ areaports.Scoped   = (*Store)(nil)
	_ areaports.Instance = (*Store)(nil)
	_ areaports.Unscoped = (*Store)(nil) // want `port method areaports.Unscoped.Leak takes no orgscope.Org` `port method areaports.Unscoped.NoReason takes no orgscope.Org` `port method areaports.Unscoped.WrongKind takes no orgscope.Org`
	_ areaports.Embedded = (*Store)(nil) // want `port method areaports.Embedded.Leak takes no orgscope.Org` `port method areaports.Embedded.NoReason takes no orgscope.Org` `port method areaports.Embedded.WrongKind takes no orgscope.Org`
	// The standard library's interfaces are not ports.
	_ io.Closer = (*Store)(nil)
)

func (*Store) Get(context.Context, orgscope.Org, string) (string, error) { return "", nil }
func (*Store) Put(context.Context, orgscope.Org, string) error           { return nil }
func (*Store) Create(context.Context, string) error                      { return nil }
func (*Store) Sweep(context.Context, int) error                          { return nil }
func (*Store) Leak(context.Context, string) (string, error)              { return "", nil }
func (*Store) NoReason(context.Context) error                            { return nil }
func (*Store) WrongKind(context.Context) error                           { return nil }
func (*Store) Fine(context.Context, orgscope.Org) error                  { return nil }
func (*Store) Close() error                                              { return nil }
