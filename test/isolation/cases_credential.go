// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The credential store (SPEC-004 SE7, ADR-0043; R-ISO-10, R-AZ-07, R-CRED-01). A credential of A is invisible to B
// through every method: no read, no list, no version, no change, no share. B's own credentials are not affected.

// credentialCases maps each method of the credential store to its case; addCredentialCases registers them.
var credentialCases = map[string]func(context.Context, testing.TB, *Env){
	"CreateCredential":     caseCreateCredential,
	"GetCredential":        caseGetCredential,
	"ListCredentials":      caseListCredentials,
	"GetCredentialVersion": caseGetCredentialVersion,
	"AddVersion":           caseAddVersion,
	"UpdateCredential":     caseUpdateCredential,
	"DeleteCredential":     caseDeleteCredential,
	"GetShare":             caseGetShare,
	"CreateShare":          caseCreateShare,
	"DeleteShare":          caseDeleteShare,
}

// addCredentialCases registers the cases of the credential store.
func addCredentialCases(r *Registry) error {
	for name, run := range credentialCases {
		if err := r.Add(Case{Subject: StoreMethod(name), Run: run}); err != nil {
			return err
		}
	}
	return nil
}

// credentialOf returns a credential of organization o with one version, stored through the store under o's scope.
// The ciphertext is opaque to the store, so a fixed byte string stands in for the sealed secret.
func credentialOf(ctx context.Context, tb testing.TB, env *Env, o *Org) (credential.Record, credential.Version) {
	tb.Helper()
	cred := fresh[id.UUID](env)
	rec := credential.Record{
		ID: cred, OrgID: identity.OrgIDOf(o.Scope), ProjectID: o.Project.ID,
		Name: "credential " + cred.String()[:8], ConnectorReference: "isolation", Status: credential.StatusOK,
		AllowedHosts: []string{"api.example.com"}, CurrentVersion: 1, CreatedBy: o.User.PrincipalID,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	v := versionOf(env, o, rec, 1)
	if err := env.Stores.CreateCredential(ctx, o.Scope, rec, v); err != nil {
		tb.Fatalf("create a credential of %s: %v", o.ID, err)
	}
	return rec, v
}

// versionOf builds a version of rec for organization o. Its public field carries the organization's sentinel.
func versionOf(env *Env, o *Org, rec credential.Record, number int) credential.Version {
	return credential.Version{
		ID: fresh[id.UUID](env), OrgID: identity.OrgIDOf(o.Scope), CredentialID: rec.ID, Number: number,
		PublicFields:     []byte(`{"label":"` + o.Sentinel("credential-label") + `"}`),
		SecretCiphertext: bytes.Repeat([]byte{0xAB}, 48), DataKeyWrapped: []byte{1, 2, 3, 4}, OrgKeyVersion: 1,
		CreatedBy: o.User.PrincipalID, CreatedAt: seededAt,
	}
}

// caseCreateCredential: B cannot store a credential under A's organization, and A's credential is stored once.
func caseCreateCredential(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, v := credentialOf(ctx, tb, env, &env.A)
	if err := env.Stores.CreateCredential(ctx, env.B.Scope, rec, versionOf(env, &env.A, rec, 1)); err == nil {
		tb.Fatal("B stored a credential under the organization of A")
	}
	if err := env.Stores.CreateCredential(ctx, env.A.Scope, rec, v); err == nil {
		tb.Fatal("a second credential with the same identity was stored")
	}
}

// caseGetCredential: the owner reads its credential; B gets not found and nothing of A.
func caseGetCredential(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	got, err := env.Stores.GetCredential(ctx, env.A.Scope, rec.ID)
	if err != nil || got.ID != rec.ID {
		tb.Fatalf("A reads its credential = %v, %v", got.ID, err)
	}
	other, err := env.Stores.GetCredential(ctx, env.B.Scope, rec.ID)
	wantErr(tb, "B reads the credential of A", err, credential.ErrNotFound)
	noLeak(tb, "GetCredential of B", other, err, &env.A)
}

// caseListCredentials: B lists its own credentials only; A's project is empty for B.
func caseListCredentials(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	credentialOf(ctx, tb, env, &env.A)
	seen, err := env.Stores.ListCredentials(ctx, env.B.Scope, env.A.Project.ID)
	if err != nil || len(seen) != 0 {
		tb.Fatalf("B lists the credentials of the project of A = %d, %v; want none", len(seen), err)
	}
	mine, err := env.Stores.ListCredentials(ctx, env.B.Scope, env.B.Project.ID)
	noLeak(tb, "ListCredentials of B", mine, err, &env.A)
}

// caseGetCredentialVersion: the owner reads a version; B gets not found and no public field of A.
func caseGetCredentialVersion(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	if _, err := env.Stores.GetCredentialVersion(ctx, env.A.Scope, rec.ID, 1); err != nil {
		tb.Fatalf("A reads its version: %v", err)
	}
	got, err := env.Stores.GetCredentialVersion(ctx, env.B.Scope, rec.ID, 1)
	wantErr(tb, "B reads a version of A", err, credential.ErrNotFound)
	noLeak(tb, "GetCredentialVersion of B", got, err, &env.A)
}

// caseAddVersion: B cannot add a version to A's credential, and A's current version does not move.
func caseAddVersion(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	bad := versionOf(env, &env.B, rec, 2)
	if err := env.Stores.AddVersion(ctx, env.B.Scope, rec.ID, 1, bad, credential.StatusOK, seededAt); err == nil {
		tb.Fatal("B added a version to the credential of A")
	}
	got, err := env.Stores.GetCredential(ctx, env.A.Scope, rec.ID)
	if err != nil || got.CurrentVersion != 1 {
		tb.Fatalf("the current version of A after B's attempt = %d, %v; want 1", got.CurrentVersion, err)
	}
}

// caseUpdateCredential: B cannot rename or re-host A's credential.
func caseUpdateCredential(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	forged := rec
	forged.OrgID = identity.OrgIDOf(env.B.Scope)
	forged.Name = env.B.Sentinel("renamed")
	if err := env.Stores.UpdateCredential(ctx, env.B.Scope, forged); err == nil {
		tb.Fatal("B updated the credential of A")
	}
	got, err := env.Stores.GetCredential(ctx, env.A.Scope, rec.ID)
	if err != nil || got.Name != rec.Name {
		tb.Fatalf("the name of A after B's attempt = %q, %v", got.Name, err)
	}
}

// caseDeleteCredential: B cannot delete A's credential.
func caseDeleteCredential(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	wantErr(tb, "B deletes the credential of A", env.Stores.DeleteCredential(ctx, env.B.Scope, env.A.Project.ID, rec.ID), credential.ErrNotFound)
	if _, err := env.Stores.GetCredential(ctx, env.A.Scope, rec.ID); err != nil {
		tb.Fatalf("A lost its credential after B's attempt: %v", err)
	}
}

// caseGetShare: B cannot see a share of A's credential.
func caseGetShare(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	_, err := env.Stores.GetShare(ctx, env.B.Scope, rec.ID, env.A.Project.ID)
	wantErr(tb, "B reads a share of A", err, credential.ErrNotFound)
}

// caseCreateShare: a share never crosses organizations, in either direction.
func caseCreateShare(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	toB := credential.Share{ID: fresh[id.UUID](env), OrgID: identity.OrgIDOf(env.A.Scope), CredentialID: rec.ID,
		ProjectID: env.B.Project.ID, CreatedAt: seededAt}
	if err := env.Stores.CreateShare(ctx, env.A.Scope, toB); err == nil {
		tb.Fatal("A shared its credential with a project of B")
	}
	fromB := toB
	fromB.OrgID = identity.OrgIDOf(env.B.Scope)
	fromB.ProjectID = env.B.Project.ID
	if err := env.Stores.CreateShare(ctx, env.B.Scope, fromB); err == nil {
		tb.Fatal("B shared a credential of A")
	}
	if _, err := env.Stores.GetShare(ctx, env.A.Scope, rec.ID, env.B.Project.ID); !errors.Is(err, credential.ErrNotFound) {
		tb.Fatalf("a share from A to B exists: %v", err)
	}
}

// caseDeleteShare: B cannot remove a share of A's credential.
func caseDeleteShare(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec, _ := credentialOf(ctx, tb, env, &env.A)
	wantErr(tb, "B removes a share of A", env.Stores.DeleteShare(ctx, env.B.Scope, rec.ID, env.A.Project.ID), credential.ErrNotFound)
}
