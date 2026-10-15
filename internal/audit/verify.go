// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Each read of the chain during verification fetches at most this many rows.
const verifyPage = 500

// Reason says what a violation is. The values are stable: alerts and the SIEM
// consumers match on them.
type Reason string

// Violation reasons.
const (
	// ReasonSequence: the sequence numbers do not run 1, 2, 3 and so on, with a gap, a repeat or a reorder.
	ReasonSequence Reason = "SEQUENCE"
	// ReasonLink: an event's previous hash is not the hash of the event before it.
	ReasonLink Reason = "LINK"
	// ReasonHash: an event's hash does not match its previous hash, sequence and body.
	ReasonHash Reason = "HASH"
	// ReasonColumns: the stored action, outcome, time or actor differ from the signed body.
	ReasonColumns Reason = "COLUMNS"
	// ReasonBlockRange: block ranges overlap, leave a gap or are inverted.
	ReasonBlockRange Reason = "BLOCK_RANGE"
	// ReasonBlockHead: a block's head hash is not the hash of its last event.
	ReasonBlockHead Reason = "BLOCK_HEAD"
	// ReasonBlockMAC: a block's MAC does not verify under the key of its version.
	ReasonBlockMAC Reason = "BLOCK_MAC"
	// ReasonMissingEvent: a block closes a sequence number that no stored event has.
	ReasonMissingEvent Reason = "MISSING_EVENT"
	// ReasonUnsealed: stored events lie beyond the last block.
	ReasonUnsealed Reason = "UNSEALED"
	// ReasonTruncated: the chain ends before the last anchor, so its tail was removed.
	ReasonTruncated Reason = "TRUNCATED"
	// ReasonAnchor: the event at the anchor's sequence number is not the one the anchor recorded.
	ReasonAnchor Reason = "ANCHOR_MISMATCH"
)

// Anchor is a hash recorded outside the database at some earlier moment (R-AUD-05).
// Without one, a truncation of the newest events is not detectable.
type Anchor struct {
	Sequence int64
	Hash     [32]byte
}

// Violation is one place where the stored chain disagrees with itself or with its anchor.
type Violation struct {
	Sequence int64
	Reason   Reason
	Detail   string
}

// Report is the result of one verification.
type Report struct {
	Chain      Chain
	Events     int64
	Blocks     int
	LastSeq    int64
	Violations []Violation
}

// OK reports whether the chain verified without a violation.
func (r Report) OK() bool { return len(r.Violations) == 0 }

// Verifier recomputes the links and the MACs of a chain from its stored rows.
type Verifier struct {
	store Store
	keys  KeySource
}

// NewVerifier returns a verifier over its ports.
func NewVerifier(store Store, keys KeySource) *Verifier {
	return &Verifier{store: store, keys: keys}
}

// walk carries the state of one verification pass over the events of a chain.
type walk struct {
	chain    Chain
	rep      *Report
	blocks   map[int64]Block
	seen     map[int64]bool
	macKeys  map[int64][]byte
	expect   int64
	prev     [32]byte
	anchor   *Anchor
	anchorOK bool
	keys     KeySource
}

// Verify reads the whole chain and reports every violation it finds. A storage or key
// error stops the verification with an error, because a report would then be partial.
func (v *Verifier) Verify(ctx context.Context, chain Chain, anchor *Anchor) (Report, error) {
	rep := Report{Chain: chain}
	blocks, err := v.readBlocks(ctx, chain, &rep)
	if err != nil {
		return rep, err
	}
	w := &walk{
		chain: chain, rep: &rep, blocks: make(map[int64]Block, len(blocks)), seen: make(map[int64]bool, len(blocks)),
		macKeys: make(map[int64][]byte), expect: 1, anchor: anchor, keys: v.keys,
	}
	defer w.wipeKeys()
	for i := range blocks {
		w.blocks[blocks[i].LastSequenceNumber] = blocks[i]
	}
	last, err := v.walkEvents(ctx, w)
	if err != nil {
		return rep, err
	}
	rep.LastSeq = last
	checkBlocks(&rep, blocks, w.seen, last)
	if anchor != nil {
		switch {
		case anchor.Sequence > last:
			addViolation(&rep, anchor.Sequence, ReasonTruncated, fmt.Sprintf("chain ends at %d, the anchor is at %d", last, anchor.Sequence))
		case !w.anchorOK:
			addViolation(&rep, anchor.Sequence, ReasonAnchor, "the anchored event is missing")
		}
	}
	return rep, nil
}

// walkEvents reads every event in order and checks each one; it returns the last sequence number read.
func (v *Verifier) walkEvents(ctx context.Context, w *walk) (int64, error) {
	var last int64
	for after := int64(0); ; {
		page, err := v.store.ListSealed(ctx, w.chain, after, verifyPage)
		if err != nil {
			return last, fmt.Errorf("verify chain: read events: %w", err)
		}
		if len(page) == 0 {
			return last, nil
		}
		for i := range page {
			e := &page[i]
			after = e.Sequence
			last = e.Sequence
			w.rep.Events++
			if err := w.check(ctx, e); err != nil {
				return last, err
			}
		}
	}
}

// check verifies one event against the expected position, the previous hash, its
// columns, and the block that closes it when it is a block's last event.
func (w *walk) check(ctx context.Context, e *Sealed) error {
	if e.Sequence != w.expect {
		addViolation(w.rep, e.Sequence, ReasonSequence, fmt.Sprintf("expected sequence %d", w.expect))
	}
	if e.PreviousHash != w.prev {
		addViolation(w.rep, e.Sequence, ReasonLink, "previous hash does not match the event before")
	}
	if chainHash(e.PreviousHash, e.Sequence, e.Entry.Body) != e.Hash {
		addViolation(w.rep, e.Sequence, ReasonHash, "hash does not match previous hash, sequence and body")
	}
	if detail := columnMismatch(e); detail != "" {
		addViolation(w.rep, e.Sequence, ReasonColumns, detail)
	}
	if w.anchor != nil && e.Sequence == w.anchor.Sequence {
		w.anchorOK = true
		if subtle.ConstantTimeCompare(e.Hash[:], w.anchor.Hash[:]) != 1 {
			addViolation(w.rep, e.Sequence, ReasonAnchor, "hash differs from the external anchor")
		}
	}
	if b, ok := w.blocks[e.Sequence]; ok {
		if err := w.checkBlock(ctx, b, e.Hash); err != nil {
			return err
		}
	}
	w.expect = e.Sequence + 1
	w.prev = e.Hash
	return nil
}

// checkBlock verifies a block against the hash of its last event and against its MAC.
func (w *walk) checkBlock(ctx context.Context, b Block, head [32]byte) error {
	w.seen[b.LastSequenceNumber] = true
	w.rep.Blocks++
	if b.HeadHash != head {
		addViolation(w.rep, b.LastSequenceNumber, ReasonBlockHead, "block head is not the hash of its last event")
	}
	key, err := w.macKey(ctx, b.KeyVersion)
	if err != nil {
		return err
	}
	want := blockMAC(key, w.chain, b.LastSequenceNumber, b.HeadHash)
	if subtle.ConstantTimeCompare(want[:], b.MAC[:]) != 1 {
		addViolation(w.rep, b.LastSequenceNumber, ReasonBlockMAC, "block MAC does not verify")
	}
	return nil
}

// macKey returns the MAC key of one version, fetching it at most once per verification.
func (w *walk) macKey(ctx context.Context, version int64) ([]byte, error) {
	if k, ok := w.macKeys[version]; ok {
		return k, nil
	}
	k, err := w.keys.MACKey(ctx, w.chain, version)
	if err != nil {
		return nil, fmt.Errorf("verify chain: mac key version %d: %w", version, err)
	}
	w.macKeys[version] = k
	return k, nil
}

func (w *walk) wipeKeys() {
	for _, k := range w.macKeys {
		clear(k)
	}
}

// readBlocks reads every block of the chain and checks that their ranges tile the sequence without overlap.
func (v *Verifier) readBlocks(ctx context.Context, chain Chain, rep *Report) ([]Block, error) {
	var out []Block
	for after := int64(0); ; {
		page, err := v.store.ListBlocks(ctx, chain, after, verifyPage)
		if err != nil {
			return nil, fmt.Errorf("verify chain: read blocks: %w", err)
		}
		if len(page) == 0 {
			break
		}
		out = append(out, page...)
		after = page[len(page)-1].LastSequenceNumber
	}
	var next int64 = 1
	for i := range out {
		b := &out[i]
		if b.FirstSequenceNumber != next || b.LastSequenceNumber < b.FirstSequenceNumber {
			addViolation(rep, b.LastSequenceNumber, ReasonBlockRange,
				fmt.Sprintf("block covers %d..%d, expected to start at %d", b.FirstSequenceNumber, b.LastSequenceNumber, next))
		}
		next = b.LastSequenceNumber + 1
	}
	return out, nil
}

// checkBlocks reports blocks whose last event is absent, and events that no block seals.
func checkBlocks(rep *Report, blocks []Block, seen map[int64]bool, last int64) {
	for i := range blocks {
		if !seen[blocks[i].LastSequenceNumber] {
			addViolation(rep, blocks[i].LastSequenceNumber, ReasonMissingEvent, "no stored event has this sequence number")
		}
	}
	if len(blocks) > 0 && blocks[len(blocks)-1].LastSequenceNumber < last {
		addViolation(rep, last, ReasonUnsealed, "events lie beyond the last block")
	}
}

func addViolation(rep *Report, seq int64, reason Reason, detail string) {
	rep.Violations = append(rep.Violations, Violation{Sequence: seq, Reason: reason, Detail: detail})
}

// columnMismatch compares the columns that the sealer copied from the entry with the
// body that the hash covers. A change to a column alone is then detectable.
func columnMismatch(e *Sealed) string {
	doc, err := jsonvalue.Parse(e.Entry.Body)
	if err != nil {
		return "body is not valid JSON"
	}
	if s, ok := stringMember(doc, "action"); !ok || s != e.Entry.Action {
		return "action differs from the body"
	}
	if s, ok := stringMember(doc, "outcome"); !ok || s != string(e.Entry.Outcome) {
		return "outcome differs from the body"
	}
	if s, ok := stringMember(doc, "occurredAt"); !ok || !sameInstant(s, e.Entry.OccurredAt) {
		return "occurrence time differs from the body"
	}
	actor, hasActor := doc.Get("actor")
	switch {
	case e.Entry.Actor.IsAnonymous() && hasActor:
		return "body names an actor the row does not"
	case !e.Entry.Actor.IsAnonymous() && !hasActor:
		return "row names an actor the body does not"
	case hasActor:
		kind, _ := stringMember(actor, "kind")
		pid, _ := stringMember(actor, "principalId")
		if kind != string(e.Entry.Actor.Kind) || pid != e.Entry.Actor.PrincipalID.String() {
			return "actor differs from the body"
		}
	}
	return ""
}

func stringMember(v jsonvalue.Value, key string) (string, bool) {
	m, ok := v.Get(key)
	if !ok {
		return "", false
	}
	return m.Str()
}

// sameInstant compares the body's time text with the stored time, both in the one
// canonical spelling the entry was built with (microseconds, UTC).
func sameInstant(text string, t time.Time) bool {
	return text == t.UTC().Truncate(time.Microsecond).Format(timeLayout)
}
