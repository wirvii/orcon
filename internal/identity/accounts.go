// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Password bounds (SPEC-004 R-AUTH-01, Q18): at least twelve characters, no composition
// rules. The upper bound keeps one hash from costing an unbounded amount of work.
const (
	MinPasswordLength = 12
	maxPasswordLength = 1024
)

// AccountStore is what the account service needs from storage.
type AccountStore interface {
	UserStore
	PrincipalStore
}

// Accounts creates user accounts and changes their passwords for the operator's command line
// (SPEC-016 SH5). Each change is stored with its audit entry in the same transaction, and the
// installation acts as the actor (SYSTEM principal).
type Accounts struct {
	store  AccountStore
	hasher PasswordHasher
	clock  clock.Clock
	ids    id.Generator
}

// NewAccounts returns the account service over its ports.
func NewAccounts(store AccountStore, hasher PasswordHasher, clk clock.Clock, ids id.Generator) *Accounts {
	return &Accounts{store: store, hasher: hasher, clock: clk, ids: ids}
}

// ValidatePassword enforces the password bounds. The error names the rule and never the password.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLength:
		return fmt.Errorf("%w: the password must have at least %d characters", ErrInvalidInput, MinPasswordLength)
	case len(password) > maxPasswordLength:
		return fmt.Errorf("%w: the password must have at most %d bytes", ErrInvalidInput, maxPasswordLength)
	}
	return nil
}

// Create adds a user account with the given address, display name and password. The address
// is normalized; a second account with the same address fails with [ErrConflict].
func (a *Accounts) Create(ctx context.Context, email, displayName, password string) (UserRecord, error) {
	email = NormalizeEmail(email)
	if email == "" || !strings.Contains(email, "@") || utf8.RuneCountInString(email) > 320 {
		return UserRecord{}, fmt.Errorf("%w: the address must be an email address", ErrInvalidInput)
	}
	if err := ValidatePassword(password); err != nil {
		return UserRecord{}, err
	}
	if displayName == "" {
		displayName = email
	}
	hash, err := a.hasher.Hash(password)
	if err != nil {
		return UserRecord{}, fmt.Errorf("create account: hash password: %w", err)
	}
	actor, err := a.systemActor(ctx)
	if err != nil {
		return UserRecord{}, err
	}
	now := clock.Micros(a.clock.Now())
	user := UserRecord{
		ID: UserID(a.ids.New()), PrincipalID: PrincipalID(a.ids.New()), EmailNormalized: email,
		DisplayName: displayName, Status: UserActive, CreatedAt: now, UpdatedAt: now,
	}
	details, err := jsonvalue.Object(jsonvalue.Member{Key: "userId", Value: jsonvalue.String(user.ID.String())})
	if err != nil {
		return UserRecord{}, fmt.Errorf("create account: %w", err)
	}
	entry, err := a.entry(actor, "account.created", details, now)
	if err != nil {
		return UserRecord{}, err
	}
	if err := a.store.CreateUserWithEntry(ctx, user, hash, entry); err != nil {
		return UserRecord{}, fmt.Errorf("create account: %w", err)
	}
	return user, nil
}

// SetPassword replaces the password of the account with the given address and revokes every
// live session of that account, in one transaction with the audit entry. An unknown address
// fails with [ErrNotFound].
func (a *Accounts) SetPassword(ctx context.Context, email, password string) error {
	email = NormalizeEmail(email)
	user, err := a.store.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := a.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("set password: hash password: %w", err)
	}
	actor, err := a.systemActor(ctx)
	if err != nil {
		return err
	}
	now := clock.Micros(a.clock.Now())
	details, err := jsonvalue.Object(jsonvalue.Member{Key: "userId", Value: jsonvalue.String(user.ID.String())})
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	entry, err := a.entry(actor, "password.changed", details, now)
	if err != nil {
		return err
	}
	if err := a.store.ChangePassword(ctx, user.ID, hash, now, entry); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return nil
}

// systemActor returns the actor of the installation, creating its SYSTEM principal the first
// time it is needed (the schema allows one).
func (a *Accounts) systemActor(ctx context.Context) (audit.Actor, error) {
	return installationActor(ctx, a.store, a.clock, a.ids)
}

// installationActor returns the SYSTEM principal of the installation as the actor of an
// operator action, creating it the first time (the schema allows one).
func installationActor(ctx context.Context, store PrincipalStore, clk clock.Clock, ids id.Generator) (audit.Actor, error) {
	p, err := store.GetSystemPrincipal(ctx)
	if errors.Is(err, ErrNotFound) {
		p = PrincipalRecord{ID: PrincipalID(ids.New()), Kind: PrincipalSystem, CreatedAt: clock.Micros(clk.Now()), UpdatedAt: clock.Micros(clk.Now())}
		if err := store.CreatePrincipal(ctx, p); err != nil && !errors.Is(err, ErrConflict) {
			return audit.Actor{}, fmt.Errorf("system principal: %w", err)
		}
		p, err = store.GetSystemPrincipal(ctx)
	}
	if err != nil {
		return audit.Actor{}, fmt.Errorf("system principal: %w", err)
	}
	return audit.Actor{PrincipalID: id.UUID(p.ID), Kind: audit.ActorSystem}, nil
}

// entry builds the audit entry of an account action, stamped with the given instant.
func (a *Accounts) entry(actor audit.Actor, action string, details jsonvalue.Value, at time.Time) (audit.Entry, error) {
	e, err := audit.NewEntry(audit.EntryParams{
		ID: a.ids.New(), Chain: audit.InstanceChain(), OccurredAt: at, Actor: actor,
		Action: action, Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("audit %s: %w", action, err)
	}
	return e, nil
}
