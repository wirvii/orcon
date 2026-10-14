// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Store names the store a run targets.
type Store string

// The stores under test.
const (
	StoreSQLite   Store = "sqlite"
	StorePostgres Store = "postgres"
)

// ErrUnknownStore reports a -store value that is not a store.
var ErrUnknownStore = errors.New("isolation: unknown store")

// ErrRLSOnlyNeedsPostgres reports the rls-only mode on a store without row-level security.
var ErrRLSOnlyNeedsPostgres = errors.New("isolation: the rls-only mode needs -store=postgres")

// ParseStore turns a -store flag value into a Store.
func ParseStore(s string) (Store, error) {
	switch st := Store(s); st {
	case StoreSQLite, StorePostgres:
		return st, nil
	default:
		return "", fmt.Errorf("%w %q (want sqlite or postgres)", ErrUnknownStore, s)
	}
}

// Org is one of the two organizations of a run.
type Org struct {
	ID   string
	Name string
	slug string
}

// Sentinel returns a value that appears nowhere else, to plant in a field of
// the organization's data (a flow name, a project, a credential label) and to
// search for in whatever the other organization receives or stores.
func (o Org) Sentinel(label string) string {
	return "orcon-sentinel-" + o.slug + "-" + label
}

// Env is what a case runs against.
type Env struct {
	Store   Store
	RLSOnly bool // wall 3 alone: the filter of wall 1 is off (build tag isolation_rls_only)
	A, B    Org  // A attacks, B is attacked
	// Editor opens a fresh backend for the editor cases. The test driver sets it for the store
	// under test; a run without it fails the editor cases rather than passing them vacuously.
	Editor EditorOpener
}

// NewEnv builds the environment of a run. The argument rlsOnly is the build-tag
// constant [RLSOnly]; it is a parameter so the combination check can be tested.
func NewEnv(store Store, rlsOnly bool) (*Env, error) {
	if rlsOnly && store != StorePostgres {
		return nil, ErrRLSOnlyNeedsPostgres
	}
	return &Env{
		Store:   store,
		RLSOnly: rlsOnly,
		A:       Org{ID: "0192b8e0-0000-7000-8000-00000000000a", Name: "Organization A", slug: "a"},
		B:       Org{ID: "0192b8e0-0000-7000-8000-00000000000b", Name: "Organization B", slug: "b"},
	}, nil
}

// Leaks returns the forbidden values that appear in haystack.
func Leaks(haystack string, forbidden ...string) []string {
	var found []string
	for _, f := range forbidden {
		if f != "" && strings.Contains(haystack, f) {
			found = append(found, f)
		}
	}
	return found
}

// AssertNoLeak fails the test for every forbidden value found in haystack. The
// argument where says what was searched ("response body", "address.collision event").
func AssertNoLeak(tb testing.TB, where, haystack string, forbidden ...string) {
	tb.Helper()
	for _, f := range Leaks(haystack, forbidden...) {
		tb.Errorf("%s carries %q, a value of another organization", where, f)
	}
}

// Kind tells what a Subject is.
type Kind int

// The kinds of subject.
const (
	KindRoute Kind = iota
	KindStoreMethod
)

func (k Kind) String() string {
	if k == KindRoute {
		return "route"
	}
	return "store method"
}

// Subject is something that handles organization data and so needs an
// isolation case: an HTTP route ("POST /v1/projects/{projectId}/flows") or a
// store method ("FlowStore.Get").
type Subject struct {
	Kind Kind
	Name string
}

func (s Subject) String() string { return s.Kind.String() + " " + s.Name }

// Route returns the subject of an HTTP route.
func Route(name string) Subject { return Subject{Kind: KindRoute, Name: name} }

// StoreMethod returns the subject of a store method.
func StoreMethod(name string) Subject { return Subject{Kind: KindStoreMethod, Name: name} }

// Surface lists everything that needs a case. Later cuts implement it from the
// OpenAPI document (routes) and from the store interfaces (methods).
type Surface interface {
	Routes() []string
	StoreMethods() []string
}

// Subjects returns the subjects of a surface, sorted.
func Subjects(s Surface) []Subject {
	var out []Subject
	for _, r := range s.Routes() {
		out = append(out, Route(r))
	}
	for _, m := range s.StoreMethods() {
		out = append(out, StoreMethod(m))
	}
	sortSubjects(out)
	return out
}

// Case is one isolation case: Run acts as a member of env.A against env.B and
// fails tb on any access or leak.
type Case struct {
	Subject Subject
	Run     func(ctx context.Context, tb testing.TB, env *Env)
}

// Registry maps subjects to cases.
type Registry struct {
	cases map[Subject]Case
}

// ErrDuplicateCase reports two cases for one subject.
var ErrDuplicateCase = errors.New("isolation: duplicate case")

// ErrInvalidCase reports a case without a name or a body.
var ErrInvalidCase = errors.New("isolation: a case needs a subject name and a Run function")

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{cases: map[Subject]Case{}} }

// Add registers a case.
func (r *Registry) Add(c Case) error {
	if c.Subject.Name == "" || c.Run == nil {
		return ErrInvalidCase
	}
	if _, dup := r.cases[c.Subject]; dup {
		return fmt.Errorf("%w for %s", ErrDuplicateCase, c.Subject)
	}
	r.cases[c.Subject] = c
	return nil
}

// Missing returns the subjects of the surface that have no case.
func (r *Registry) Missing(s Surface) []Subject {
	var out []Subject
	for _, sub := range Subjects(s) {
		if _, ok := r.cases[sub]; !ok {
			out = append(out, sub)
		}
	}
	return out
}

// Orphans returns the cases whose subject is no longer on the surface, so a
// renamed or removed route cannot leave a case that tests nothing.
func (r *Registry) Orphans(s Surface) []Subject {
	on := map[Subject]bool{}
	for _, sub := range Subjects(s) {
		on[sub] = true
	}
	var out []Subject
	for sub := range r.cases {
		if !on[sub] {
			out = append(out, sub)
		}
	}
	sortSubjects(out)
	return out
}

// Len is the number of registered cases.
func (r *Registry) Len() int { return len(r.cases) }

// Run runs every case as a subtest, in subject order.
func (r *Registry) Run(ctx context.Context, t *testing.T, env *Env) {
	t.Helper()
	subjects := make([]Subject, 0, len(r.cases))
	for sub := range r.cases {
		subjects = append(subjects, sub)
	}
	sortSubjects(subjects)
	for _, sub := range subjects {
		c := r.cases[sub]
		t.Run(sub.String(), func(t *testing.T) {
			c.Run(ctx, t, env)
		})
	}
}

func sortSubjects(s []Subject) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Kind != s[j].Kind {
			return s[i].Kind < s[j].Kind
		}
		return s[i].Name < s[j].Name
	})
}

// NoSurface is the surface of a tree without routes or store methods that
// handle organization data, which is the tree today. A cut that adds the first
// route or store method replaces it in [DefaultSurface].
type NoSurface struct{}

// Routes returns nothing.
func (NoSurface) Routes() []string { return nil }

// StoreMethods returns nothing.
func (NoSurface) StoreMethods() []string { return nil }

// DefaultSurface is what TestRegistryComplete compares [BuildRegistry] with.
var DefaultSurface Surface = EditorSurface{}
