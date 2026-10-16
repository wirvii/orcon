// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
)

const orgsUsage = `usage:
  orcon orgs create --slug <slug> --name <name> --admin <address>

Creates an organization and makes the account of --admin its Admin, in one
transaction with the organization's key and the audit entries of the creation.
The command needs the same master key settings as the server, and the server
must have started once. An organization that exists without a key (created by
an older build) gets one at the next server start, logged as a warning. The
account must already
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

// openCommandKeyring loads the master key the configuration names and checks it against the stored
// canary, as the server does at start (SPEC-004 R-KEY-05), so a command never writes a key under the
// wrong master key. A store without keys has not been started once: the server creates the canary
// and the instance key at its first start, after the backup acknowledgement.
func openCommandKeyring(ctx context.Context, cfg *config.Config, store persistence) (*envelope.Keyring, error) {
	mk, err := openMasterKey(ctx, cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		return nil, err
	}
	rep, err := envelope.Verify(ctx, mk, store)
	if err != nil {
		return nil, fmt.Errorf("verify the master key: %w", err)
	}
	if rep.Fresh {
		return nil, errors.New("the store has no keys yet: start the server once so it creates them, then create the organization")
	}
	return envelope.NewKeyring(mk, store), nil
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

	cfg, err := loadCommandConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitError
	}
	store, err := openStore(ctx, cfg, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitError
	}
	defer func() { _ = store.Close() }()
	kr, err := openCommandKeyring(ctx, cfg, store)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitError
	}

	orgs := identity.NewOrganizations(store, orgKeyIssuer{kr: kr}, clock.System{}, id.NewV7Generator(time.Now))
	org, err := orgs.Create(ctx, identity.NewOrganization{Name: *name, Slug: *slug, AdminEmail: *admin})
	if err != nil {
		return accountFailure(stderr, err)
	}
	_, _ = fmt.Fprintf(stdout, "created organization %s (%s); %s is its Admin\n", org.Slug, org.ID, *admin)
	return exitOK
}
