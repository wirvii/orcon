// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

const orgsUsage = `usage:
  orcon orgs create --slug <slug> --name <name> --admin <address>

Creates an organization and makes the account of --admin its Admin, in one
transaction with the audit entries of the creation. The account must already
exist (orcon accounts create). The slug is lowercase letters, digits and
single hyphens. When the server runs on SQLite it holds the database file, so
stop it first.`

// orgsCommand creates organizations for the operator (SPEC-016 SH4, SPEC-004 R-ID-02).
func orgsCommand() command {
	return command{
		name:    "orgs",
		summary: "create organizations and name their first administrator",
		run:     runOrgs,
	}
}

// runOrgs dispatches the orgs subcommands.
func runOrgs(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stdout, orgsUsage)
		return exitUsage
	}
	if args[0] != "create" {
		_, _ = fmt.Fprintf(stderr, "orcon: unknown orgs subcommand %q\n%s\n", args[0], orgsUsage)
		return exitUsage
	}
	return runOrgCreate(ctx, args[1:], stdout, stderr)
}

// runOrgCreate parses the flags, opens the store the server opens and creates the organization.
func runOrgCreate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("orcon orgs create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	slug := fs.String("slug", "", "the slug of the organization (lowercase letters, digits and single hyphens)")
	name := fs.String("name", "", "the name of the organization")
	admin := fs.String("admin", "", "the address of the account that becomes its Admin")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "orcon: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return exitUsage
	}
	required := []struct{ flag, value string }{{"--slug", *slug}, {"--name", *name}, {"--admin", *admin}}
	for _, r := range required {
		if r.value == "" {
			_, _ = fmt.Fprintf(stderr, "orcon: %s is required\n", r.flag)
			return exitUsage
		}
	}

	store, err := openAccountsStore(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitError
	}
	defer func() { _ = store.Close() }()

	orgs := identity.NewOrganizations(store, clock.System{}, id.NewV7Generator(time.Now))
	org, err := orgs.Create(ctx, identity.NewOrganization{Name: *name, Slug: *slug, AdminEmail: *admin})
	if err != nil {
		return accountFailure(stderr, err)
	}
	_, _ = fmt.Fprintf(stdout, "created organization %s (%s); %s is its Admin\n", org.Slug, org.ID, *admin)
	return exitOK
}
