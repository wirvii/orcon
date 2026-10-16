// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Store is the credential store the service consumes (SPEC-004 SE7-store). The adapters implement it in
// internal/adapter/sqlite and internal/adapter/postgres. Every method takes the organization scope, and a row of
// another organization is never returned or changed. Writes that clash return [ErrConflict]; a missing row returns
// [ErrNotFound]. Versions are never updated in place: a replacement adds a row.
//
//nolint:interfacebloat // the store is one port with one method per row operation of the credential tables (ADR-0043)
type Store interface {
	// CreateCredential stores a credential and its first version in one transaction.
	CreateCredential(ctx context.Context, org orgscope.Org, rec Record, first Version, entry audit.Entry) error
	// GetCredential returns one credential of the organization, with the author and time of its current secret.
	GetCredential(ctx context.Context, org orgscope.Org, credential id.UUID) (Record, error)
	// ListCredentials returns the credentials a project owns or holds a share of, without the secrets.
	ListCredentials(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]Record, error)
	// GetCredentialVersion returns one stored version, with its ciphertext.
	GetCredentialVersion(ctx context.Context, org orgscope.Org, credential id.UUID, number int) (Version, error)
	// AddVersion stores a new version, makes it current and sets the status of the credential, all in one transaction,
	// and only while the current version is still expectedCurrent (otherwise [ErrConflict]). v.Number is
	// expectedCurrent+1.
	AddVersion(ctx context.Context, org orgscope.Org, credential id.UUID, expectedCurrent int, v Version, status Status, at time.Time, entry audit.Entry) error
	// UpdateCredential stores the name and the allowed hosts of a credential.
	UpdateCredential(ctx context.Context, org orgscope.Org, rec Record, entry audit.Entry) error
	// DeleteCredential removes a credential owned by the project, with its versions and shares.
	DeleteCredential(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, entry audit.Entry) error
	// GetShare returns the share of a credential with a project, or [ErrNotFound].
	GetShare(ctx context.Context, org orgscope.Org, credential id.UUID, project identity.ProjectID) (Share, error)
	// CreateShare stores a share; [ErrConflict] when the project already holds one.
	CreateShare(ctx context.Context, org orgscope.Org, s Share, entry audit.Entry) error
	// DeleteShare removes a share, or returns [ErrNotFound].
	DeleteShare(ctx context.Context, org orgscope.Org, credential id.UUID, project identity.ProjectID, entry audit.Entry) error
}

// DataKeys is the key service the credential service consumes. The envelope keyring implements it (ADR-0042): it
// wraps a fresh data key under the active organization key and unwraps a stored one under the version recorded with
// its ciphertext.
type DataKeys interface {
	// GenerateDataKey returns a fresh data key wrapped by the active key of the scope.
	GenerateDataKey(ctx context.Context, scope envelope.Scope) (envelope.DataKey, error)
	// UnwrapDataKey recovers a data key from its wrapped form under the key version named by id.
	UnwrapDataKey(ctx context.Context, id envelope.KeyID, wrapped []byte) (envelope.Key, error)
}

// Resolver is what a dispatch uses to obtain a credential for one node execution (ADR-0043 section 3). The service
// implements it; the dispatch path must call it for every request, never once per publication, so that a removed
// share stops the next dispatch.
type Resolver interface {
	// Resolve returns the current version of a credential that the project owns or holds a share of.
	Resolve(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) (Resolved, error)
}
