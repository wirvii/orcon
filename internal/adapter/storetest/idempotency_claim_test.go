// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
)

// raceRounds is how many times the claim race of BL-127 runs: the loser of one race is decided
// by timing, so one round proves little.
const raceRounds = 40

// TestIdempotencyClaimRaceAnswersTaken (BL-127): two requests that carry the same Idempotency-Key
// race to claim it. Exactly one commits its project; the other is refused with
// idempotencyclaim.ErrTaken, which the API answers as 409 idempotency-in-progress, and never with
// a conflict of the resource. Only one project exists after each round.
func TestIdempotencyClaimRaceAnswersTaken(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		fp := sha256.Sum256([]byte("race"))
		for round := range raceRounds {
			key := fmt.Sprintf("race-%d", round)
			type attempt struct {
				project identity.ProjectRecord
				ctx     idempotencyclaim.Claim
				grant   identity.RoleBindingRecord
			}
			var attempts [2]attempt
			for i := range attempts {
				claim, _ := g.claimCtx(key, fp, g.user.ID)
				p := g.newProject(fmt.Sprintf("race-%d-%d", round, i))
				attempts[i] = attempt{project: p, ctx: claim, grant: g.bindingFor(p.ID, g.user.ID)}
			}
			var (
				wg    sync.WaitGroup
				start = make(chan struct{})
				errs  [2]error
			)
			for i := range attempts {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					a := attempts[i]
					errs[i] = g.s.CreateProjectWithGrant(idempotencyclaim.With(g.ctx(), a.ctx), scope, a.project, a.grant,
						[]audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")})
				}()
			}
			close(start)
			wg.Wait()

			won, taken := 0, 0
			for i, err := range errs {
				switch {
				case err == nil:
					won++
				case errors.Is(err, idempotencyclaim.ErrTaken):
					taken++
				default:
					t.Fatalf("round %d attempt %d = %v, want success or ErrTaken (never a conflict of the resource)", round, i, err)
				}
			}
			if won != 1 || taken != 1 {
				t.Fatalf("round %d: %d committed and %d refused as taken; want one of each", round, won, taken)
			}
			created := 0
			for i := range attempts {
				if _, err := g.s.GetProject(g.ctx(), scope, attempts[i].project.ID); err == nil {
					created++
				}
			}
			if created != 1 {
				t.Fatalf("round %d created %d projects under one key, want 1", round, created)
			}
		}
	})
}

// The crash test runs the change in a child process, as the API process would, and kills the
// child after the change committed and before the response is sent, then retries the request
// with the same key against the same database. The test binary is the child: it runs
// TestIdempotencyCrashChild when the variables below are set.
const (
	envCrashStore   = "ORCON_CRASH_STORE" // sqlite or postgres
	envCrashTarget  = "ORCON_CRASH_TARGET"
	envCrashOrg     = "ORCON_CRASH_ORG"
	envCrashUser    = "ORCON_CRASH_USER"
	envCrashKey     = "ORCON_CRASH_KEY"
	envCrashPrincip = "ORCON_CRASH_PRINCIPAL"
)

// TestIdempotencyCrashChild is the child of TestIdempotencyCrashAfterCommit. It does nothing in
// a normal run.
func TestIdempotencyCrashChild(t *testing.T) {
	kind := os.Getenv(envCrashStore)
	if kind == "" {
		t.Skip("not the child of the crash test")
	}
	ctx := t.Context()
	var s storetest.Store
	switch kind {
	case "sqlite":
		st, err := sqlite.Open(ctx, sqlite.Options{Path: os.Getenv(envCrashTarget), BinaryVersion: storetest.BinaryVersion()})
		if err != nil {
			t.Fatalf("child: open sqlite: %v", err)
		}
		s = st
	case "postgres":
		st, err := postgres.Open(ctx, postgres.Options{DSN: os.Getenv(envCrashTarget), BinaryVersion: storetest.BinaryVersion()})
		if err != nil {
			t.Fatalf("child: open postgres: %v", err)
		}
		s = st
	default:
		t.Fatalf("child: unknown store %q", kind)
	}
	org := mustParseID(t, os.Getenv(envCrashOrg))
	user := mustParseID(t, os.Getenv(envCrashUser))
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := id.NewV7Generator(time.Now)
	p := identity.ProjectRecord{
		ID: identity.ProjectID(ids.New()), OrgID: identity.OrgID(org), Name: "Crashed", Slug: "crashed", Handle: "crashed",
		IsProtected: true, CreatedAt: now, UpdatedAt: now,
	}
	grant := identity.RoleBindingRecord{
		ID: identity.RoleBindingID(ids.New()), OrgID: p.OrgID, ScopeKind: identity.RoleScopeProject, ProjectID: p.ID,
		UserID: identity.UserID(user), RoleName: identity.RoleProjectAdmin, CreatedAt: now, UpdatedAt: now,
	}
	claim := idempotencyclaim.Claim{
		ID: ids.New(), Requester: "PRINCIPAL:" + os.Getenv(envCrashPrincip), Value: os.Getenv(envCrashKey),
		Fingerprint: sha256.Sum256([]byte("crash")),
	}
	err := s.CreateProjectWithGrant(idempotencyclaim.With(ctx, claim), scopeOf(p.OrgID), p, grant,
		[]audit.Entry{audittest.Org(t, id.UUID(p.OrgID), "project.created")})
	if err != nil {
		t.Fatalf("child: the change failed before it committed: %v", err)
	}
	// The change is committed; the response has not been sent. The process dies here.
	_, _ = fmt.Fprintf(os.Stdout, "COMMITTED %s\n", p.ID)
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	select {} // not reached: SIGKILL cannot be handled
}

func mustParseID(t *testing.T, s string) id.UUID {
	t.Helper()
	u, err := id.Parse(s)
	if err != nil {
		t.Fatalf("parse identity %q: %v", s, err)
	}
	return u
}

// TestIdempotencyCrashAfterCommit (SPEC-005 Amendment 8 section 8-2 item 5, E-CONV-2): the process
// is killed after the change committed and before the response was sent. The record of the key
// stays in progress and names the resource; a retry of the same key does not create a second
// resource (the claim is taken) and is answered from the resource that exists. On both stores.
func TestIdempotencyCrashAfterCommit(t *testing.T) {
	if os.Getenv(envCrashStore) != "" {
		t.Skip("running as the child of the crash test")
	}
	if wantSQLite() {
		t.Run("sqlite", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "orcon.db")
			open := func() *sqlite.Store {
				s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: storetest.BinaryVersion()})
				if err != nil {
					t.Fatalf("open sqlite store: %v", err)
				}
				return s
			}
			first := open()
			g := newGrantFixture(t, func(*testing.T) storetest.Store { return first })
			// SQLite holds the database file exclusively: the parent lets go of it while the child works.
			if err := first.Close(); err != nil {
				t.Fatalf("close before the child runs: %v", err)
			}
			child := runCrashChild(t, "sqlite", path, g)
			second := open()
			t.Cleanup(func() { _ = second.Close() })
			g.s = second
			checkCrashRecovery(t, g, child)
		})
	}
	if wantPostgres() {
		t.Run("postgres", func(t *testing.T) {
			dsn := newPostgresDatabase(t)
			g := newGrantFixture(t, func(t *testing.T) storetest.Store {
				t.Helper()
				return storetest.OpenPostgres(t, dsn)
			})
			child := runCrashChild(t, "postgres", dsn, g)
			checkCrashRecovery(t, g, child)
		})
	}
}

// crashChild is what the killed child left: the identity of the project it committed and the key it claimed.
type crashChild struct {
	project identity.ProjectID
	key     identity.IdempotencyKey
}

// runCrashChild runs the child until it kills itself and checks that it did so after the commit.
func runCrashChild(t *testing.T, kind, target string, g *grantFixture) crashChild {
	t.Helper()
	const keyValue = "crash-key"
	//nolint:gosec // G204: the test binary runs itself as the child of the crash test; no input reaches the command line
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestIdempotencyCrashChild$", "-test.count=1")
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "ORCON_TEST_POSTGRES")
	}),
		envCrashStore+"="+kind, envCrashTarget+"="+target, envCrashOrg+"="+g.orgA.String(),
		envCrashUser+"="+g.user.ID.String(), envCrashPrincip+"="+g.user.ID.String(), envCrashKey+"="+keyValue)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the child was expected to be killed, got err %v with output %s", err, out)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("the child did not die by SIGKILL: %v (output %s)", exit, out)
	}
	line, found := strings.CutPrefix(strings.TrimSpace(string(out)), "COMMITTED ")
	if !found {
		t.Fatalf("the child was killed before its change committed: %s", out)
	}
	return crashChild{
		project: identity.ProjectID(mustParseID(t, line)),
		key:     identity.IdempotencyKey{Requester: "PRINCIPAL:" + g.user.ID.String(), Value: keyValue},
	}
}

// checkCrashRecovery is the retry after the crash.
func checkCrashRecovery(t *testing.T, g *grantFixture, child crashChild) {
	t.Helper()
	scope := scopeOf(g.orgA)
	if _, err := g.s.GetProject(g.ctx(), scope, child.project); err != nil {
		t.Fatalf("the committed project is missing after the crash: %v", err)
	}
	rec, found, err := g.s.LookupIdempotency(g.ctx(), scope, child.key, time.Now())
	if err != nil || !found {
		t.Fatalf("lookup after the crash = %v, %v; want the record", found, err)
	}
	wantRef := "project:" + id.UUID(child.project).String()
	if rec.Completed || rec.ResourceRef != wantRef {
		t.Fatalf("record = %+v, want in progress and naming %s", rec, wantRef)
	}

	// The retry runs the change again under the same key: the claim is taken, nothing is written.
	fp := sha256.Sum256([]byte("crash"))
	claim := idempotencyclaim.Claim{ID: g.ids.New(), Requester: child.key.Requester, Value: child.key.Value, Fingerprint: fp}
	retry := g.newProject("crashed-again")
	err = g.s.CreateProjectWithGrant(idempotencyclaim.With(g.ctx(), claim), scope, retry, g.bindingFor(retry.ID, g.user.ID),
		[]audit.Entry{audittest.Org(t, id.UUID(g.orgA), "project.created")})
	if !errors.Is(err, idempotencyclaim.ErrTaken) {
		t.Fatalf("the retry under the same key = %v, want ErrTaken", err)
	}
	if _, err := g.s.GetProject(g.ctx(), scope, retry.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("the retry created a second project: %v", err)
	}

	// The answer to the retry is built from the resource the record names; completing the record
	// stores it, and the next repeat replays it.
	ref, ok := strings.CutPrefix(rec.ResourceRef, "project:")
	if !ok {
		t.Fatalf("resource reference %q is not a project", rec.ResourceRef)
	}
	same, err := g.s.GetProject(g.ctx(), scope, identity.ProjectID(mustParseID(t, ref)))
	if err != nil || same.ID != child.project {
		t.Fatalf("project named by the record = %+v, %v; want the committed project", same, err)
	}
	resp := identity.IdempotencyResponse{Status: 201, ContentType: "application/json", Location: "/api/v1/projects/" + ref, Body: []byte(`{}`)}
	if err := g.s.CompleteIdempotency(g.ctx(), scope, child.key, identity.IdempotencyCompletion{ID: g.ids.New(), Fingerprint: fp, Response: resp}, time.Now()); err != nil {
		t.Fatalf("complete after the rebuild: %v", err)
	}
	done, found, err := g.s.LookupIdempotency(g.ctx(), scope, child.key, time.Now())
	if err != nil || !found || !done.Completed || done.Response.Location != resp.Location {
		t.Fatalf("record after the rebuild = %+v, %v, %v; want completed with the project's address", done, found, err)
	}
}
