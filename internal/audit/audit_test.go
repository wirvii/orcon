// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audit_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
)

var epoch = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// fixedClock always answers the same instant.
type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

// keys answers the same 32-byte key for every version: the tests replace it only to
// show that a different key fails verification.
type keys struct{ version int64 }

func (k keys) ActiveMACKey(_ context.Context, _ audit.Chain) (int64, []byte, error) {
	return k.version, fill(0x11), nil
}

func (k keys) MACKey(_ context.Context, _ audit.Chain, _ int64) ([]byte, error) {
	return fill(0x11), nil
}

func fill(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

// memStore keeps the chains in memory, with the same ordering and uniqueness rules the adapters enforce.
type memStore struct {
	pending map[audit.Chain][]audit.Entry
	events  map[audit.Chain][]audit.Sealed
	blocks  map[audit.Chain][]audit.Block
}

func newMemStore() *memStore {
	return &memStore{
		pending: map[audit.Chain][]audit.Entry{},
		events:  map[audit.Chain][]audit.Sealed{},
		blocks:  map[audit.Chain][]audit.Block{},
	}
}

func (m *memStore) PendingChains(_ context.Context, limit int) ([]audit.Chain, error) {
	var out []audit.Chain
	for c, p := range m.pending {
		if len(p) > 0 && len(out) < limit {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memStore) ListPending(_ context.Context, c audit.Chain, limit int) ([]audit.Entry, error) {
	p := append([]audit.Entry(nil), m.pending[c]...)
	if len(p) > limit {
		p = p[:limit]
	}
	return p, nil
}

func (m *memStore) Tail(_ context.Context, c audit.Chain) (audit.Tail, error) {
	ev := m.events[c]
	if len(ev) == 0 {
		return audit.Tail{}, nil
	}
	last := ev[len(ev)-1]
	return audit.Tail{Sequence: last.Sequence, Hash: last.Hash}, nil
}

func (m *memStore) CommitBlock(ctx context.Context, c audit.Chain, expect audit.Tail, b audit.Batch) error {
	tail, _ := m.Tail(ctx, c)
	if tail != expect {
		return audit.ErrTailMoved
	}
	m.events[c] = append(m.events[c], b.Events...)
	m.blocks[c] = append(m.blocks[c], b.Block)
	consumed := map[string]bool{}
	for _, pid := range b.PendingIDs {
		consumed[pid.String()] = true
	}
	var rest []audit.Entry
	for i := range m.pending[c] {
		if !consumed[m.pending[c][i].ID.String()] {
			rest = append(rest, m.pending[c][i])
		}
	}
	m.pending[c] = rest
	return nil
}

func (m *memStore) ListSealed(_ context.Context, c audit.Chain, after int64, limit int) ([]audit.Sealed, error) {
	var out []audit.Sealed
	for i := range m.events[c] {
		if m.events[c][i].Sequence > after && len(out) < limit {
			out = append(out, m.events[c][i])
		}
	}
	return out, nil
}

func (m *memStore) ListBlocks(_ context.Context, c audit.Chain, after int64, limit int) ([]audit.Block, error) {
	var out []audit.Block
	for i := range m.blocks[c] {
		if m.blocks[c][i].LastSequenceNumber > after && len(out) < limit {
			out = append(out, m.blocks[c][i])
		}
	}
	return out, nil
}

func entry(t *testing.T, gen *id.Sequence, c audit.Chain, at time.Time, action string) audit.Entry {
	t.Helper()
	e, err := audit.NewEntry(audit.EntryParams{
		ID: gen.New(), Chain: c, OccurredAt: at, Actor: audit.Actor{PrincipalID: gen.New(), Kind: audit.ActorUser},
		Action: action, Outcome: audit.OutcomeSuccess,
	})
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	return e
}

// seeded returns a store with one org chain of n sealed events in one block.
func seeded(t *testing.T, n int) (*memStore, audit.Chain) {
	t.Helper()
	ctx := t.Context()
	gen := id.NewSequence()
	c := audit.OrganizationChain(gen.New())
	st := newMemStore()
	for i := range n {
		st.pending[c] = append(st.pending[c], entry(t, gen, c, epoch.Add(time.Duration(i)*time.Second), "project.created"))
	}
	sealer := audit.NewSealer(st, keys{version: 1}, gen, fixedClock{at: epoch})
	if got, err := sealer.SealOnce(ctx); err != nil || got != n {
		t.Fatalf("SealOnce = %d, %v; want %d", got, err, n)
	}
	return st, c
}

func TestSealerChainsAndMacs(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	ev := st.events[c]
	if len(ev) != 3 {
		t.Fatalf("sealed %d events, want 3", len(ev))
	}
	for i, e := range ev {
		if e.Sequence != int64(i+1) {
			t.Fatalf("event %d has sequence %d", i, e.Sequence)
		}
	}
	if ev[0].PreviousHash != [32]byte{} || ev[1].PreviousHash != ev[0].Hash {
		t.Fatal("events are not linked")
	}
	if len(st.pending[c]) != 0 {
		t.Fatal("sealed pending rows were not deleted")
	}
	rep, err := audit.NewVerifier(st, keys{version: 1}).Verify(t.Context(), c, nil)
	if err != nil || !rep.OK() || rep.Events != 3 || rep.Blocks != 1 {
		t.Fatalf("Verify = %+v, %v; want a clean chain of 3 events in 1 block", rep, err)
	}
}

func TestSealerEmptyChainSealsNothing(t *testing.T) {
	t.Parallel()
	st := newMemStore()
	gen := id.NewSequence()
	got, err := audit.NewSealer(st, keys{version: 1}, gen, fixedClock{at: epoch}).SealOnce(t.Context())
	if err != nil || got != 0 {
		t.Fatalf("SealOnce on an empty store = %d, %v", got, err)
	}
}

// brokenChainKeys fails for one chain, as an organization without a key does.
type brokenChainKeys struct {
	keys
	broken audit.Chain
}

func (b brokenChainKeys) ActiveMACKey(ctx context.Context, c audit.Chain) (int64, []byte, error) {
	if c == b.broken {
		return 0, nil, errors.New("active ORG key: key not found")
	}
	return b.keys.ActiveMACKey(ctx, c)
}

// TestSealerSealsTheOtherChainsWhenOneFails: a chain that cannot be sealed is reported, and the
// chains after it are still sealed in the same pass.
func TestSealerSealsTheOtherChainsWhenOneFails(t *testing.T) {
	t.Parallel()
	st := newMemStore()
	gen := id.NewSequence()
	bad, good := audit.OrganizationChain(gen.New()), audit.OrganizationChain(gen.New())
	st.pending[bad] = []audit.Entry{entry(t, gen, bad, epoch, "org.created")}
	st.pending[good] = []audit.Entry{entry(t, gen, good, epoch, "org.created")}
	sealer := audit.NewSealer(st, brokenChainKeys{keys: keys{version: 1}, broken: bad}, gen, fixedClock{at: epoch})
	n, err := sealer.SealOnce(t.Context())
	if err == nil || !strings.Contains(err.Error(), bad.Label()) {
		t.Fatalf("SealOnce error = %v, want the failure of the broken chain", err)
	}
	if n != 1 || len(st.events[good]) != 1 || len(st.pending[good]) != 0 {
		t.Errorf("sealed %d events, good chain has %d events and %d pending; want the good chain sealed", n, len(st.events[good]), len(st.pending[good]))
	}
}

func TestSealerSkipsChainMovedUnderIt(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 2)
	gen := id.NewSequence()
	st.pending[c] = append(st.pending[c], entry(t, gen, c, epoch.Add(time.Hour), "project.created"))
	// A second sealer sealed meanwhile: the tail the first one reads is stale.
	stale := &staleTail{memStore: st, stale: audit.Tail{}}
	got, err := audit.NewSealer(stale, keys{version: 1}, gen, fixedClock{at: epoch}).SealOnce(t.Context())
	if err != nil || got != 0 {
		t.Fatalf("SealOnce with a moved chain = %d, %v; want 0 and no error", got, err)
	}
	if rep, _ := audit.NewVerifier(st, keys{version: 1}).Verify(t.Context(), c, nil); !rep.OK() {
		t.Fatalf("chain damaged by the skipped pass: %+v", rep.Violations)
	}
}

// staleTail reports an old tail, so CommitBlock refuses the batch like a racing sealer would.
type staleTail struct {
	*memStore
	stale audit.Tail
}

func (s *staleTail) Tail(_ context.Context, _ audit.Chain) (audit.Tail, error) { return s.stale, nil }

func (s *staleTail) CommitBlock(ctx context.Context, c audit.Chain, expect audit.Tail, b audit.Batch) error {
	return s.memStore.CommitBlock(ctx, c, expect, b)
}

func verify(t *testing.T, st *memStore, c audit.Chain, anchor *audit.Anchor) audit.Report {
	t.Helper()
	rep, err := audit.NewVerifier(st, keys{version: 1}).Verify(t.Context(), c, anchor)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return rep
}

func hasReason(rep audit.Report, r audit.Reason) bool {
	for _, v := range rep.Violations {
		if v.Reason == r {
			return true
		}
	}
	return false
}

func TestDetectsUpdate(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	st.events[c][1].Entry.Body = []byte(strings.Replace(string(st.events[c][1].Entry.Body), "project.created", "project.deleted", 1))
	if rep := verify(t, st, c, nil); !hasReason(rep, audit.ReasonHash) {
		t.Fatalf("edited body not detected: %+v", rep.Violations)
	}
}

func TestDetectsColumnChange(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 2)
	st.events[c][0].Entry.Outcome = audit.OutcomeFailure
	if rep := verify(t, st, c, nil); !hasReason(rep, audit.ReasonColumns) {
		t.Fatalf("changed outcome column not detected: %+v", rep.Violations)
	}
}

func TestDetectsDelete(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	st.events[c] = append(st.events[c][:1], st.events[c][2:]...)
	if rep := verify(t, st, c, nil); rep.OK() || !hasReason(rep, audit.ReasonSequence) {
		t.Fatalf("deleted event not detected: %+v", rep.Violations)
	}
}

func TestDetectsReorder(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	ev := st.events[c]
	ev[0], ev[1] = ev[1], ev[0]
	if rep := verify(t, st, c, nil); rep.OK() {
		t.Fatalf("reordered events not detected: %+v", rep.Violations)
	}
}

func TestDetectsInsert(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 2)
	gen := id.NewSequence()
	forged := st.events[c][0]
	forged.Entry = entry(t, gen, c, epoch, "role.created")
	forged.Sequence = 2
	st.events[c] = append([]audit.Sealed{st.events[c][0], forged}, st.events[c][1:]...)
	if rep := verify(t, st, c, nil); rep.OK() {
		t.Fatalf("inserted event not detected: %+v", rep.Violations)
	}
}

// TestDetectsRecomputedChainWithoutKey is the attack the MAC exists for: someone with
// database access rewrites an event and recomputes every hash after it, but holds no key.
func TestDetectsRecomputedChainWithoutKey(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	ev := st.events[c]
	forged := ev[0].Entry
	forged.Action = "role.created"
	forged.Body = []byte(strings.Replace(string(forged.Body), "project.created", "role.created", 1))
	ev[0].Entry = forged
	prev := [32]byte{}
	for i := range ev {
		ev[i].PreviousHash = prev
		ev[i].Hash = audit.ChainHashForTest(prev, ev[i].Sequence, ev[i].Entry.Body)
		prev = ev[i].Hash
	}
	st.blocks[c][0].HeadHash = prev
	if rep := verify(t, st, c, nil); !hasReason(rep, audit.ReasonBlockMAC) {
		t.Fatalf("recomputed chain without the key not detected: %+v", rep.Violations)
	}
}

func TestDetectsTruncationVsAnchor(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 4)
	anchorEvent := st.events[c][3]
	st.events[c] = st.events[c][:2]
	st.blocks[c] = nil
	anchor := &audit.Anchor{Sequence: anchorEvent.Sequence, Hash: anchorEvent.Hash}
	if rep := verify(t, st, c, anchor); !hasReason(rep, audit.ReasonTruncated) {
		t.Fatalf("truncation against the anchor not detected: %+v", rep.Violations)
	}
}

func TestAnchorMatchVerifies(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 3)
	e := st.events[c][1]
	if rep := verify(t, st, c, &audit.Anchor{Sequence: e.Sequence, Hash: e.Hash}); !rep.OK() {
		t.Fatalf("a matching anchor must verify: %+v", rep.Violations)
	}
}

// wrongKeys answers a key that is not the one the chain was sealed with.
type wrongKeys struct{}

func (wrongKeys) ActiveMACKey(context.Context, audit.Chain) (int64, []byte, error) {
	return 1, fill(0xEE), nil
}

func (wrongKeys) MACKey(context.Context, audit.Chain, int64) ([]byte, error) {
	return fill(0xEE), nil
}

func TestDetectsWrongMACKey(t *testing.T) {
	t.Parallel()
	st, c := seeded(t, 2)
	rep, err := audit.NewVerifier(st, wrongKeys{}).Verify(t.Context(), c, nil)
	if err != nil || !hasReason(rep, audit.ReasonBlockMAC) {
		t.Fatalf("a wrong key must fail the MAC: %+v, %v", rep.Violations, err)
	}
}

func TestNewEntryRejectsBadInput(t *testing.T) {
	t.Parallel()
	gen := id.NewSequence()
	c := audit.InstanceChain()
	base := audit.EntryParams{ID: gen.New(), Chain: c, OccurredAt: epoch, Action: "login.succeeded", Outcome: audit.OutcomeSuccess}
	cases := map[string]func(p *audit.EntryParams){
		"no id":          func(p *audit.EntryParams) { p.ID = id.UUID{} },
		"no chain":       func(p *audit.EntryParams) { p.Chain = audit.Chain{} },
		"bad action":     func(p *audit.EntryParams) { p.Action = "Login.Succeeded" },
		"action dot end": func(p *audit.EntryParams) { p.Action = "login." },
		"bad outcome":    func(p *audit.EntryParams) { p.Outcome = "ok" },
		"half actor":     func(p *audit.EntryParams) { p.Actor = audit.Actor{PrincipalID: gen.New()} },
		"no time":        func(p *audit.EntryParams) { p.OccurredAt = time.Time{} },
	}
	for name, mutate := range cases {
		p := base
		mutate(&p)
		if _, err := audit.NewEntry(p); !errors.Is(err, audit.ErrInvalidEntry) {
			t.Errorf("%s: err = %v, want ErrInvalidEntry", name, err)
		}
	}
}

func TestAuditFormatValuesUpperSnake(t *testing.T) {
	t.Parallel()
	gen := id.NewSequence()
	e, err := audit.NewEntry(audit.EntryParams{
		ID: gen.New(), Chain: audit.OrganizationChain(gen.New()), OccurredAt: epoch,
		Actor: audit.Actor{PrincipalID: gen.New(), Kind: audit.ActorServiceAccount}, Action: "project.created", Outcome: audit.OutcomeDenied,
	})
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	body := string(e.Body)
	for _, want := range []string{`"format":"orcon/audit@1"`, `"outcome":"DENIED"`, `"kind":"SERVICE_ACCOUNT"`, `"action":"project.created"`, `"chain":"ORGANIZATION"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s: %s", want, body)
		}
	}
	for _, kind := range []audit.ActorKind{audit.ActorUser, audit.ActorServiceAccount, audit.ActorSCIMClient, audit.ActorAPIKey, audit.ActorSystem} {
		if !kind.Valid() || kind != audit.ActorKind(strings.ToUpper(string(kind))) {
			t.Errorf("actor kind %q is not a valid UPPER_SNAKE value", kind)
		}
	}
}

func TestValidAction(t *testing.T) {
	t.Parallel()
	good := []string{"login.succeeded", "flow.published", "credential.used", "a.b_c.d9", "logout"}
	bad := []string{"", "lo", "Login.succeeded", "login..x", ".login.x", "login.x.", "login succeeded"}
	for _, g := range good {
		if !audit.ValidAction(g) {
			t.Errorf("ValidAction(%q) = false", g)
		}
	}
	for _, b := range bad {
		if audit.ValidAction(b) {
			t.Errorf("ValidAction(%q) = true", b)
		}
	}
}
