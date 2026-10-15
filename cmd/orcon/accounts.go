// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/adapter/codec/configyaml"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/password"
)

const accountsUsage = `usage:
  orcon accounts create --email <address> [--name <display name>]
  orcon accounts set-password --email <address>

The password is read from standard input, the first line, and is never a
command-line argument. It must have at least 12 characters. When the server
runs on SQLite it holds the database file, so stop it first.`

// accountsCommand creates accounts and changes their passwords (SPEC-016 SH5).
func accountsCommand() command {
	return command{
		name:    "accounts",
		summary: "create accounts and change their passwords",
		run: func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
			return runAccounts(ctx, args, os.Stdin, stdout, stderr)
		},
	}
}

// runAccounts dispatches the accounts subcommands; stdin carries the password.
func runAccounts(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stdout, accountsUsage)
		return exitUsage
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-password") || strings.HasPrefix(a, "--password") {
			_, _ = fmt.Fprintln(stderr, "orcon: the password is never a command-line argument; send it on standard input")
			return exitUsage
		}
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "create", "set-password":
		return runAccountChange(ctx, sub, rest, stdin, stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "orcon: unknown accounts subcommand %q\n%s\n", sub, accountsUsage)
		return exitUsage
	}
}

// runAccountChange parses one subcommand, reads the password and applies the change.
func runAccountChange(ctx context.Context, sub string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("orcon accounts "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String("email", "", "the address of the account")
	name := fs.String("name", "", "the display name (create only)")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "orcon: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return exitUsage
	}
	if *email == "" {
		_, _ = fmt.Fprintln(stderr, "orcon: --email is required")
		return exitUsage
	}
	if sub == "set-password" && *name != "" {
		_, _ = fmt.Fprintln(stderr, "orcon: --name applies to create only")
		return exitUsage
	}
	secret, err := readPassword(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitUsage
	}

	store, err := openAccountsStore(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitError
	}
	defer func() { _ = store.Close() }()

	accounts := identity.NewAccounts(store, password.Default(), clock.System{}, id.NewV7Generator(time.Now))
	switch sub {
	case "create":
		user, err := accounts.Create(ctx, *email, *name, secret)
		if err != nil {
			return accountFailure(stderr, err)
		}
		_, _ = fmt.Fprintf(stdout, "created account %s\n", user.ID)
	case "set-password":
		if err := accounts.SetPassword(ctx, *email, secret); err != nil {
			return accountFailure(stderr, err)
		}
		_, _ = fmt.Fprintln(stdout, "password changed; the account's live sessions ended")
	}
	return exitOK
}

// readPassword reads the first line of stdin, without its line ending.
func readPassword(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from standard input: %w", err)
	}
	secret := strings.TrimRight(line, "\r\n")
	if secret == "" {
		return "", errors.New("no password on standard input")
	}
	return secret, nil
}

// accountFailure prints a failed account change and returns its exit code. The messages
// of the identity service never carry the password.
func accountFailure(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
	if errors.Is(err, identity.ErrInvalidInput) {
		return exitUsage
	}
	return exitError
}

// openAccountsStore loads the configuration the way serve does and opens the store it names.
func openAccountsStore(ctx context.Context) (persistence, error) { //nolint:ireturn // the same store the server opens, used through the account ports
	schema := config.Default()
	flags := flag.NewFlagSet("orcon accounts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fv := schema.BindFlags(flags)
	res := schema.Load(config.Input{
		Flags: fv, Environ: os.Environ(), FS: config.OSFileSystem{}, Decode: configyaml.Decode, GOOS: runtime.GOOS,
	})
	if err := res.Err(); err != nil {
		return nil, err
	}
	return openStore(ctx, res.Config, version)
}
