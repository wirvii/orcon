// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// The cases of the store methods. Each one acts as organization A, with A's scope
// and B's identities (which an attacker can guess or has seen), and expects what
// ADR-0046 promises: a row of B is not found, a list holds none of B's rows, a write
// that names B is refused, B's data is unchanged afterwards, and nothing returned or
// stored carries a sentinel of B.

// storeCases lists the cases by method name; the name must be the name of a method of [Stores].
var storeCases = map[string]func(ctx context.Context, tb testing.TB, env *Env){
	// Organization.
	"CreateOrganization": caseCreateOrganization,
	"GetOrganization":    caseGetOrganization,
	// Principals.
	"CreatePrincipal":    caseCreatePrincipal,
	"GetSystemPrincipal": caseGetSystemPrincipal,
	"DeletePrincipal":    caseDeletePrincipal,
	// Users (instance table).
	"CreateUser":     caseCreateUser,
	"GetUser":        caseGetUser,
	"GetUserByEmail": caseGetUserByEmail,
	// Organizations of a user (engine plane: membership rows of every organization).
	"ResolveUserOrganizations":  caseResolveUserOrganizations,
	"GetUserCredentialsByEmail": caseGetUserCredentialsByEmail,
	// Key store (ADR-0042).
	"MasterCanary":    caseMasterCanary,
	"PutMasterCanary": caseMasterCanary,
	"PutKey":          casePutKey,
	"Key":             caseKey,
	"ActiveKey":       caseActiveKey,
	"ActiveKeys":      caseActiveKeys,
	"SetPasswordHash": caseSetPasswordHash,
	// Sessions and the sign-in throttle (instance tables).
	"CreateSession":          caseCreateSession,
	"GetSession":             caseGetSession,
	"GetSessionByTokenHash":  caseGetSessionByTokenHash,
	"UpdateSessionActivity":  caseUpdateSessionActivity,
	"MarkSessionRevoked":     caseMarkSessionRevoked,
	"GetLoginThrottle":       caseGetLoginThrottle,
	"PutLoginThrottle":       casePutLoginThrottle,
	"ClearLoginThrottle":     caseClearLoginThrottle,
	"IncrementLoginThrottle": caseIncrementLoginThrottle,
	"LockLoginThrottle":      caseLockLoginThrottle,
	// Memberships.
	"CreateMembership":       caseCreateMembership,
	"GetMembership":          caseGetMembership,
	"GetMembershipOfUser":    caseGetMembershipOfUser,
	"ListMemberships":        caseListMemberships,
	"UpdateMembershipStatus": caseUpdateMembershipStatus,
	// Projects.
	"CreateProject": caseCreateProject,
	"GetProject":    caseGetProject,
	"ListProjects":  caseListProjects,
	// Flows.
	"CreateFlow":      caseCreateFlow,
	"GetFlow":         caseGetFlow,
	"GetFlowByName":   caseGetFlowByName,
	"ListFlows":       caseListFlows,
	"UpdateFlowDraft": caseUpdateFlowDraft,
	// Versions.
	"PublishVersion":     casePublishVersion,
	"GetVersion":         caseGetVersion,
	"GetVersionByNumber": caseGetVersionByNumber,
	"ListVersions":       caseListVersions,
	// Snapshots.
	"CreateSnapshot": caseCreateSnapshot,
	"GetSnapshot":    caseGetSnapshot,
	// Workers (instance table).
	"RegisterWorker": caseRegisterWorker,
}

func addStoreCases(r *Registry) error {
	for name, run := range storeCases {
		if err := r.Add(Case{Subject: StoreMethod(name), Run: run}); err != nil {
			return err
		}
	}
	return nil
}

// dump flattens a value into the text a leak would show up in: strings, byte slices as text,
// and every field of structs, slices and pointers.
func dump(v any) string {
	var b strings.Builder
	var walk func(reflect.Value)
	walk = func(rv reflect.Value) {
		switch rv.Kind() { //nolint:exhaustive // only the kinds that can hold text matter
		case reflect.String:
			b.WriteString(rv.String())
			b.WriteByte(' ')
		case reflect.Slice:
			if rv.Type().Elem().Kind() == reflect.Uint8 {
				b.Write(rv.Bytes())
				b.WriteByte(' ')
				return
			}
			for i := range rv.Len() {
				walk(rv.Index(i))
			}
		case reflect.Struct:
			if _, isTime := rv.Interface().(time.Time); isTime {
				return
			}
			for i := range rv.NumField() {
				if rv.Type().Field(i).IsExported() {
					walk(rv.Field(i))
				}
			}
		case reflect.Pointer, reflect.Interface:
			if !rv.IsNil() {
				walk(rv.Elem())
			}
		default: // numbers, booleans and the rest hold no text
		}
	}
	walk(reflect.ValueOf(v))
	return b.String()
}

// noLeak fails tb when a result or an error carries a secret of org.
func noLeak(tb testing.TB, where string, result any, err error, other *Org) {
	tb.Helper()
	text := dump(result)
	if err != nil {
		text += " " + err.Error()
	}
	AssertNoLeak(tb, where, text, other.Secrets()...)
}

// wantErr fails tb unless err satisfies errors.Is for want.
func wantErr(tb testing.TB, what string, err, want error) {
	tb.Helper()
	if !errors.Is(err, want) {
		tb.Errorf("%s: error %v, want %v", what, err, want)
	}
}

// intact fails tb when anything of org B changed since it was seeded.
func intact(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	b, s := env.B, env.Stores
	if got, err := s.GetOrganization(ctx, b.Scope); err != nil || got.Name != b.Record.Name || got.Status != b.Record.Status {
		tb.Errorf("organization B changed: %+v, %v", got, err)
	}
	if got, err := s.GetProject(ctx, b.Scope, b.Project.ID); err != nil || got.Name != b.Project.Name || got.Handle != b.Project.Handle {
		tb.Errorf("project of B changed: %+v, %v", got, err)
	}
	if got, err := s.ListProjects(ctx, b.Scope, identity.ProjectID{}, 100); err != nil || len(got) != 1 {
		tb.Errorf("projects of B: %d, %v; want exactly the seeded one", len(got), err)
	}
	if got, err := s.GetFlow(ctx, b.Scope, b.Flow.ID); err != nil || got.DraftRevision != 0 || !bytes.Equal(got.Draft, b.Flow.Draft) || got.ActiveVersionID != b.Version.ID {
		tb.Errorf("flow of B changed: %+v, %v", got, err)
	}
	if got, err := s.ListFlows(ctx, b.Scope, b.Project.ID, flow.FlowID{}, 100); err != nil || len(got) != 1 {
		tb.Errorf("flows of B: %d, %v; want exactly the seeded one", len(got), err)
	}
	if got, err := s.ListVersions(ctx, b.Scope, b.Flow.ID, 0, 100); err != nil || len(got) != 1 {
		tb.Errorf("versions of B: %d, %v; want exactly the seeded one", len(got), err)
	}
	if got, err := s.GetSnapshot(ctx, b.Scope, b.Snapshot.ID); err != nil || !bytes.Equal(got.Document, b.Snapshot.Document) {
		tb.Errorf("snapshot of B changed: %+v, %v", got, err)
	}
	if got, err := s.ListMemberships(ctx, b.Scope, identity.MembershipID{}, 100); err != nil || len(got) != 1 || got[0].Status != identity.MembershipActive {
		tb.Errorf("memberships of B: %+v, %v; want exactly the seeded one, active", got, err)
	}
}

func fresh[T ~[16]byte](env *Env) T { return T(env.IDs.New()) }

// ---------------------------------------------------------------- organization

func caseCreateOrganization(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// Creating organizations is an instance operation. Taking B's slug is refused and says nothing about B.
	dup := identity.OrgRecord{ID: fresh[identity.OrgID](env), Name: "Copycat", Slug: env.B.Record.Slug, Status: identity.OrgActive, CreatedAt: seededAt, UpdatedAt: seededAt}
	err := env.Stores.CreateOrganization(ctx, dup)
	wantErr(tb, "an organization with the slug of B", err, identity.ErrConflict)
	noLeak(tb, "the refusal", nil, err, &env.B)
	intact(ctx, tb, env)
}

func caseGetOrganization(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetOrganization(ctx, env.A.Scope)
	if err != nil || got.ID != env.A.Record.ID {
		tb.Fatalf("organization of A = %+v, %v", got, err)
	}
	noLeak(tb, "the organization of A", got, err, &env.B)
	// The scope is the only way to name an organization, so B's cannot be asked for by A's scope.
	if got.ID == env.B.Record.ID {
		tb.Error("A's scope returned organization B")
	}
}

// ---------------------------------------------------------------- principals

func caseCreatePrincipal(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// B's user already has a principal: reusing its identity is refused and the refusal says nothing about B.
	err := env.Stores.CreatePrincipal(ctx, identity.PrincipalRecord{ID: env.B.User.PrincipalID, Kind: identity.PrincipalUser, CreatedAt: seededAt, UpdatedAt: seededAt})
	wantErr(tb, "a principal with the identity of B's user", err, identity.ErrConflict)
	noLeak(tb, "the refusal", nil, err, &env.B)
	intact(ctx, tb, env)
}

func caseGetSystemPrincipal(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	sys := identity.PrincipalRecord{ID: fresh[identity.PrincipalID](env), Kind: identity.PrincipalSystem, CreatedAt: seededAt, UpdatedAt: seededAt}
	if err := env.Stores.CreatePrincipal(ctx, sys); err != nil {
		tb.Fatal(err)
	}
	got, err := env.Stores.GetSystemPrincipal(ctx)
	if err != nil || got.ID != sys.ID {
		tb.Fatalf("system principal = %+v, %v", got, err)
	}
	noLeak(tb, "the system principal", got, err, &env.B)
}

func caseDeletePrincipal(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// A principal belongs to no organization, and what it did in B stays in B: deleting it clears
	// the actor columns and deletes no row of history. Here the principal invited B's member.
	inviter := identity.UserRecord{
		ID: fresh[identity.UserID](env), PrincipalID: fresh[identity.PrincipalID](env), EmailNormalized: "inviter@example.test",
		DisplayName: "Inviter", Status: identity.UserActive, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	if err := env.Stores.CreateUser(ctx, inviter); err != nil {
		tb.Fatal(err)
	}
	invitee := identity.UserRecord{
		ID: fresh[identity.UserID](env), PrincipalID: fresh[identity.PrincipalID](env), EmailNormalized: "invitee@example.test",
		DisplayName: "Invitee", Status: identity.UserActive, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	if err := env.Stores.CreateUser(ctx, invitee); err != nil {
		tb.Fatal(err)
	}
	m := identity.MembershipRecord{
		ID: fresh[identity.MembershipID](env), OrgID: identity.OrgID(env.B.UUID), UserID: invitee.ID, Status: identity.MembershipInvited,
		InvitedByPrincipalID: inviter.PrincipalID, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	if err := env.Stores.CreateMembership(ctx, env.B.Scope, m); err != nil {
		tb.Fatal(err)
	}
	err := env.Stores.DeletePrincipal(ctx, inviter.PrincipalID, 1)
	if err != nil {
		tb.Fatalf("delete the principal: %v", err)
	}
	noLeak(tb, "the deletion", nil, err, &env.B)
	got, err := env.Stores.GetMembership(ctx, env.B.Scope, m.ID)
	if err != nil || !got.InvitedByPrincipalID.IsZero() || got.Status != identity.MembershipInvited {
		tb.Errorf("B's membership after its inviter was deleted: %+v, %v; want it kept, with no inviter", got, err)
	}
	// A principal that A merely names but that does not exist is not found, and does not reveal B's.
	err = env.Stores.DeletePrincipal(ctx, fresh[identity.PrincipalID](env), 10)
	wantErr(tb, "an unknown principal", err, identity.ErrNotFound)
}

// ---------------------------------------------------------------- users

func caseCreateUser(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// B's address is taken: the refusal does not carry B's user.
	again := identity.UserRecord{
		ID: fresh[identity.UserID](env), PrincipalID: fresh[identity.PrincipalID](env), EmailNormalized: env.B.User.EmailNormalized,
		DisplayName: "Impostor", Status: identity.UserActive, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	err := env.Stores.CreateUser(ctx, again)
	wantErr(tb, "a user with B's address", err, identity.ErrConflict)
	noLeak(tb, "the refusal", nil, err, &Org{slug: "none"})
	if _, err := env.Stores.GetUser(ctx, again.ID); !errors.Is(err, identity.ErrNotFound) {
		tb.Errorf("the refused user exists: %v", err)
	}
	intact(ctx, tb, env)
}

func caseGetUser(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// Users are global to the instance: a user is a person, not organization data. What the store
	// returns holds no organization data of anyone.
	got, err := env.Stores.GetUser(ctx, env.B.User.ID)
	if err != nil || got.ID != env.B.User.ID {
		tb.Fatalf("user B = %+v, %v", got, err)
	}
	for _, secret := range []string{env.B.Sentinel("org-name"), env.B.Sentinel("project-name"), env.B.Sentinel("flow-name"), env.B.Record.Slug} {
		AssertNoLeak(tb, "the user record", dump(got), secret)
	}
	// And a user B does not have is not found.
	_, err = env.Stores.GetUser(ctx, fresh[identity.UserID](env))
	wantErr(tb, "an unknown user", err, identity.ErrNotFound)
}

func caseGetUserByEmail(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetUserByEmail(ctx, env.B.User.EmailNormalized)
	if err != nil || got.ID != env.B.User.ID {
		tb.Fatalf("user B by address = %+v, %v", got, err)
	}
	for _, secret := range []string{env.B.Sentinel("org-name"), env.B.Sentinel("project-name"), env.B.Sentinel("flow-name"), env.B.Record.Slug} {
		AssertNoLeak(tb, "the user record", dump(got), secret)
	}
	// Knowing that B's user exists is not membership of B: A's scope does not find it.
	_, err = env.Stores.GetMembershipOfUser(ctx, env.A.Scope, got.ID)
	wantErr(tb, "B's user in A", err, identity.ErrNotFound)
}

// ---------------------------------------------------------------- memberships

func caseCreateMembership(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// A record that names B cannot be written under A's scope.
	m := identity.MembershipRecord{
		ID: fresh[identity.MembershipID](env), OrgID: identity.OrgID(env.B.UUID), UserID: env.A.User.ID, Status: identity.MembershipActive,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	wantErr(tb, "a membership of B under A's scope", env.Stores.CreateMembership(ctx, env.A.Scope, m), identity.ErrScopeMismatch)
	// Adding A's user to A twice is a conflict, and the conflict says nothing about B.
	again := env.A.Membership
	again.ID = fresh[identity.MembershipID](env)
	err := env.Stores.CreateMembership(ctx, env.A.Scope, again)
	wantErr(tb, "a second membership of A's user", err, identity.ErrConflict)
	noLeak(tb, "the refusal", nil, err, &env.B)
	// The membership of A's user in B was not created by any of it.
	_, err = env.Stores.GetMembershipOfUser(ctx, env.B.Scope, env.A.User.ID)
	wantErr(tb, "A's user in B", err, identity.ErrNotFound)
	intact(ctx, tb, env)
}

func caseGetMembership(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetMembership(ctx, env.A.Scope, env.B.Membership.ID)
	wantErr(tb, "B's membership by identity", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
	if own, err := env.Stores.GetMembership(ctx, env.A.Scope, env.A.Membership.ID); err != nil || own.OrgID != identity.OrgID(env.A.UUID) {
		tb.Errorf("A's own membership = %+v, %v", own, err)
	}
}

func caseGetMembershipOfUser(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetMembershipOfUser(ctx, env.A.Scope, env.B.User.ID)
	wantErr(tb, "B's user in A", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListMemberships(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.ListMemberships(ctx, env.A.Scope, identity.MembershipID{}, 100)
	if err != nil || len(got) != 1 || got[0].ID != env.A.Membership.ID {
		tb.Fatalf("memberships of A = %+v, %v; want only its own", got, err)
	}
	noLeak(tb, "the list", got, err, &env.B)
	// Paging with B's identity as the cursor does not walk into B.
	got, err = env.Stores.ListMemberships(ctx, env.A.Scope, env.B.Membership.ID, 100)
	if err != nil {
		tb.Fatal(err)
	}
	for i := range got {
		if got[i].OrgID != identity.OrgID(env.A.UUID) {
			tb.Errorf("page after B's cursor holds a membership of %v", got[i].OrgID)
		}
	}
}

func caseUpdateMembershipStatus(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	err := env.Stores.UpdateMembershipStatus(ctx, env.A.Scope, env.B.Membership.ID, identity.MembershipSuspended, seededAt.Add(time.Hour))
	wantErr(tb, "suspending B's membership from A", err, identity.ErrNotFound)
	noLeak(tb, "the refusal", nil, err, &env.B)
	intact(ctx, tb, env)
}

// ---------------------------------------------------------------- projects

func caseCreateProject(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := identity.ProjectRecord{
		ID: fresh[identity.ProjectID](env), OrgID: identity.OrgID(env.B.UUID), Name: "Intruder", Slug: "intruder", Handle: "intruder",
		IsProtected: true, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	wantErr(tb, "a project of B under A's scope", env.Stores.CreateProject(ctx, env.A.Scope, rec), identity.ErrScopeMismatch)
	// B's handle is taken for everyone. The store says so, and only that.
	rec.OrgID, rec.Handle = identity.OrgID(env.A.UUID), env.B.Project.Handle
	err := env.Stores.CreateProject(ctx, env.A.Scope, rec)
	wantErr(tb, "A taking B's handle", err, identity.ErrHandleTaken)
	AssertNoLeak(tb, "the refusal", errText(err), env.B.Sentinel("project-name"), env.B.Sentinel("flow-name"), env.B.Sentinel("org-name"))
	intact(ctx, tb, env)
}

func caseGetProject(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetProject(ctx, env.A.Scope, env.B.Project.ID)
	wantErr(tb, "B's project by identity", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListProjects(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.ListProjects(ctx, env.A.Scope, identity.ProjectID{}, 100)
	if err != nil || len(got) != 1 || got[0].ID != env.A.Project.ID {
		tb.Fatalf("projects of A = %+v, %v; want only its own", got, err)
	}
	noLeak(tb, "the list", got, err, &env.B)
}

// ---------------------------------------------------------------- flows

func caseCreateFlow(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := flow.Record{
		ID: fresh[flow.FlowID](env), OrgID: identity.OrgID(env.A.UUID), ProjectID: env.B.Project.ID, Name: "planted", Draft: []byte(`{}`),
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	// A flow of A in B's project: the composite foreign key refuses it.
	err := env.Stores.CreateFlow(ctx, env.A.Scope, rec)
	wantErr(tb, "a flow of A in B's project", err, identity.ErrReference)
	noLeak(tb, "the refusal", nil, err, &env.B)
	// A flow that names B under A's scope is refused before the database is asked.
	rec.OrgID = identity.OrgID(env.B.UUID)
	wantErr(tb, "a flow of B under A's scope", env.Stores.CreateFlow(ctx, env.A.Scope, rec), identity.ErrScopeMismatch)
	intact(ctx, tb, env)
}

func caseGetFlow(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetFlow(ctx, env.A.Scope, env.B.Flow.ID)
	wantErr(tb, "B's flow by identity", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseGetFlowByName(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetFlowByName(ctx, env.A.Scope, env.B.Project.ID, env.B.Flow.Name)
	wantErr(tb, "B's flow by project and name", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
	// B's flow name asked in A's own project is not found either.
	got, err = env.Stores.GetFlowByName(ctx, env.A.Scope, env.A.Project.ID, env.B.Flow.Name)
	wantErr(tb, "B's flow name in A's project", err, identity.ErrNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListFlows(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.ListFlows(ctx, env.A.Scope, env.B.Project.ID, flow.FlowID{}, 100)
	if err != nil || len(got) != 0 {
		tb.Errorf("flows of B's project listed for A: %d, %v", len(got), err)
	}
	noLeak(tb, "the list", got, err, &env.B)
	own, err := env.Stores.ListFlows(ctx, env.A.Scope, env.A.Project.ID, env.B.Flow.ID, 100)
	if err != nil {
		tb.Fatal(err)
	}
	noLeak(tb, "the page after B's cursor", own, err, &env.B)
}

func caseUpdateFlowDraft(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	_, err := env.Stores.UpdateFlowDraft(ctx, env.A.Scope, env.B.Flow.ID, []byte(`{"defaced":true}`), 0, nil, seededAt.Add(time.Hour))
	wantErr(tb, "updating B's draft from A", err, identity.ErrNotFound)
	noLeak(tb, "the refusal", nil, err, &env.B)
	intact(ctx, tb, env)
}

// ---------------------------------------------------------------- versions

func casePublishVersion(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	v := flow.VersionRecord{
		ID: fresh[flow.FlowVersionID](env), OrgID: identity.OrgID(env.A.UUID), FlowID: env.B.Flow.ID, Number: 2,
		Document: []byte(`{}`), ContentHash: "planted", PublishedAt: seededAt, CreatedAt: seededAt,
	}
	err := env.Stores.PublishVersion(ctx, env.A.Scope, v)
	if err == nil {
		tb.Error("A published a version of B's flow")
	}
	noLeak(tb, "the refusal", nil, err, &env.B)
	v.OrgID = identity.OrgID(env.B.UUID)
	wantErr(tb, "a version of B under A's scope", env.Stores.PublishVersion(ctx, env.A.Scope, v), identity.ErrScopeMismatch)
	intact(ctx, tb, env)
}

func caseGetVersion(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetVersion(ctx, env.A.Scope, env.B.Version.ID)
	wantErr(tb, "B's version by identity", err, flow.ErrVersionNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseGetVersionByNumber(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetVersionByNumber(ctx, env.A.Scope, env.B.Flow.ID, env.B.Version.Number)
	wantErr(tb, "B's version by number", err, flow.ErrVersionNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListVersions(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.ListVersions(ctx, env.A.Scope, env.B.Flow.ID, 0, 100)
	if err != nil || len(got) != 0 {
		tb.Errorf("versions of B's flow listed for A: %d, %v", len(got), err)
	}
	noLeak(tb, "the list", got, err, &env.B)
}

// ---------------------------------------------------------------- snapshots

func caseCreateSnapshot(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// B's content hash under A's scope and A's flow is A's own snapshot, never B's.
	own := flow.SnapshotRecord{
		ID: fresh[flow.FlowSnapshotID](env), OrgID: identity.OrgID(env.A.UUID), FlowID: env.A.Flow.ID, ContentHash: env.B.Snapshot.ContentHash,
		Document: []byte(`{"own":true}`), CreatedAt: seededAt,
	}
	got, err := env.Stores.CreateSnapshot(ctx, env.A.Scope, own)
	if err != nil || got.ID != own.ID || got.OrgID != identity.OrgID(env.A.UUID) {
		tb.Fatalf("A's snapshot with B's content hash = %+v, %v; want A's own", got, err)
	}
	// The content hash is the one A sent; what must not come back is B's document.
	AssertNoLeak(tb, "the snapshot document", string(got.Document), env.B.Sentinel("snapshot"))
	if got.FlowID != env.A.Flow.ID {
		tb.Errorf("the snapshot belongs to flow %v, want A's %v", got.FlowID, env.A.Flow.ID)
	}
	// A snapshot of B's flow is refused.
	planted := own
	planted.ID, planted.FlowID, planted.ContentHash = fresh[flow.FlowSnapshotID](env), env.B.Flow.ID, "planted"
	_, err = env.Stores.CreateSnapshot(ctx, env.A.Scope, planted)
	wantErr(tb, "a snapshot of B's flow", err, identity.ErrReference)
	noLeak(tb, "the refusal", nil, err, &env.B)
	planted.OrgID = identity.OrgID(env.B.UUID)
	_, err = env.Stores.CreateSnapshot(ctx, env.A.Scope, planted)
	wantErr(tb, "a snapshot of B under A's scope", err, identity.ErrScopeMismatch)
	intact(ctx, tb, env)
}

func caseGetSnapshot(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetSnapshot(ctx, env.A.Scope, env.B.Snapshot.ID)
	wantErr(tb, "B's snapshot by identity", err, flow.ErrSnapshotNotFound)
	noLeak(tb, "the lookup", got, err, &env.B)
}

// ---------------------------------------------------------------- workers

func caseRegisterWorker(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	// The worker registry belongs to the instance. Registering a worker touches no organization data.
	w := execution.WorkerRecord{ID: fresh[execution.WorkerID](env), Role: execution.WorkerWorker, Host: "isolation-host", Version: "1.0.0", StartedAt: seededAt, HeartbeatAt: seededAt}
	err := env.Stores.RegisterWorker(ctx, w)
	if err != nil {
		tb.Fatalf("register a worker: %v", err)
	}
	intact(ctx, tb, env)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}

// ---------------------------------------------------------------- organizations of a user

// caseResolveUserOrganizations (SPEC-004 R-ID-02, section 7-3): a user lists only the
// organizations of its active memberships, and a suspended membership leaves the list.
func caseResolveUserOrganizations(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.ResolveUserOrganizations(ctx, env.A.User.ID, identity.MaxResolvedOrganizations)
	if err != nil || len(got) != 1 || got[0].ID != env.A.Record.ID {
		tb.Fatalf("organizations of the user of A = %+v, %v; want only A", got, err)
	}
	noLeak(tb, "the organizations of a user", got, err, &env.B)

	if err := env.Stores.UpdateMembershipStatus(ctx, env.B.Scope, env.B.Membership.ID, identity.MembershipSuspended, time.Now().UTC()); err != nil {
		tb.Fatalf("suspend the membership of B: %v", err)
	}
	got, err = env.Stores.ResolveUserOrganizations(ctx, env.B.User.ID, identity.MaxResolvedOrganizations)
	if err != nil || len(got) != 0 {
		tb.Fatalf("organizations of a user whose only membership is suspended = %+v, %v; want none", got, err)
	}
}
