// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The audit area stores pending entries in the transaction of the action, through
// [insertPendingOrg] and [insertPendingInstance], and seals them through the
// [audit.Store] methods below. The sealer runs in the writer, so a commit sees
// the tail it checks (orcon-db CN-1).

// insertPendingOrg writes one pending entry of an organization chain inside the
// transaction of the action. The entry must belong to the organization of the scope.
func insertPendingOrg(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, e audit.Entry) error {
	if !e.Chain.Valid() || e.Chain.IsInstance() || e.Chain.Org() != org.UUID() {
		return fmt.Errorf("audit entry: %w", audit.ErrInvalidEntry)
	}
	if err := q.InsertAuditPending(ctx, pendingParams(uuidBytes(e.Chain.Org()), e)); err != nil {
		return fmt.Errorf("insert audit pending: %w", err)
	}
	return nil
}

// insertPendingInstance writes one pending entry of the instance chain inside the transaction of the action.
func insertPendingInstance(ctx context.Context, q *sqlcgen.Queries, e audit.Entry) error {
	if !e.Chain.IsInstance() {
		return fmt.Errorf("audit entry: %w", audit.ErrInvalidEntry)
	}
	if err := q.InsertInstanceAuditPending(ctx, sqlcgen.InsertInstanceAuditPendingParams{
		ID: uuidBytes(e.ID), OccurredAt: micros(e.OccurredAt), ActorPrincipalID: actorPrincipal(e.Actor),
		ActorKind: actorKind(e.Actor), ActionName: e.Action, Outcome: string(e.Outcome),
		BodyDocument: e.Body, BodyDocumentFormat: audit.Format,
		CreatedAt: micros(e.OccurredAt), UpdatedAt: micros(e.OccurredAt),
	}); err != nil {
		return fmt.Errorf("insert instance audit pending: %w", err)
	}
	return nil
}

// pendingParams builds the row of an organization pending entry.
func pendingParams(orgID []byte, e audit.Entry) sqlcgen.InsertAuditPendingParams {
	return sqlcgen.InsertAuditPendingParams{
		ID: uuidBytes(e.ID), OrgID: orgID, OccurredAt: micros(e.OccurredAt),
		ActorPrincipalID: actorPrincipal(e.Actor), ActorKind: actorKind(e.Actor), ActionName: e.Action,
		Outcome: string(e.Outcome), BodyDocument: e.Body, BodyDocumentFormat: audit.Format,
		CreatedAt: micros(e.OccurredAt), UpdatedAt: micros(e.OccurredAt),
	}
}

func actorPrincipal(a audit.Actor) []byte {
	if a.IsAnonymous() {
		return nil
	}
	return uuidBytes(a.PrincipalID)
}

func actorKind(a audit.Actor) sql.NullString {
	if a.IsAnonymous() {
		return sql.NullString{}
	}
	return sql.NullString{String: string(a.Kind), Valid: true}
}

// AppendInstanceEntry implements [identity.AuditLog]: a denied or failed attempt is
// audited in its own transaction (R-AUD-02).
func (s *Store) AppendInstanceEntry(ctx context.Context, entry audit.Entry) error {
	return s.write(ctx, "append audit entry", func(ctx context.Context, q *sqlcgen.Queries) error {
		return insertPendingInstance(ctx, q, entry)
	})
}

// PendingChains implements [audit.Store].
func (s *Store) PendingChains(ctx context.Context, limit int) ([]audit.Chain, error) {
	var chains []audit.Chain
	err := s.read(ctx, "list pending audit chains", func(ctx context.Context, q *sqlcgen.Queries) error {
		orgs, err := q.ListAuditPendingOrgs(ctx, int64(limit))
		if err != nil {
			return err
		}
		for _, b := range orgs {
			org, err := uuidOf[id.UUID](b)
			if err != nil {
				return err
			}
			chains = append(chains, audit.OrganizationChain(org))
		}
		n, err := q.CountInstanceAuditPending(ctx)
		if err != nil {
			return err
		}
		if n > 0 && len(chains) < limit {
			chains = append(chains, audit.InstanceChain())
		}
		return nil
	})
	return chains, err
}

// ListPending implements [audit.Store].
func (s *Store) ListPending(ctx context.Context, chain audit.Chain, limit int) ([]audit.Entry, error) {
	var out []audit.Entry
	err := s.read(ctx, "list pending audit", func(ctx context.Context, q *sqlcgen.Queries) error {
		if chain.IsInstance() {
			rows, err := q.ListInstanceAuditPending(ctx, int64(limit))
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				e, err := storedEntry(chain, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument)
				if err != nil {
					return err
				}
				out = append(out, e)
			}
			return nil
		}
		rows, err := q.ListAuditPending(ctx, uuidBytes(chain.Org()), int64(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			e, err := storedEntry(chain, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

// storedEntry rebuilds an entry from the columns the adapter reads; the chain is the one the query named.
func storedEntry(chain audit.Chain, rawID []byte, occurred int64, principal []byte, kind sql.NullString, action, outcome string, body []byte) (audit.Entry, error) {
	eid, err := uuidOf[id.UUID](rawID)
	if err != nil {
		return audit.Entry{}, err
	}
	var actor audit.Actor
	if principal != nil {
		pid, err := uuidOf[id.UUID](principal)
		if err != nil {
			return audit.Entry{}, err
		}
		actor = audit.Actor{PrincipalID: pid, Kind: audit.ActorKind(kind.String)}
	}
	return audit.Entry{
		ID: eid, Chain: chain, OccurredAt: time.UnixMicro(occurred).UTC(), Actor: actor,
		Action: action, Outcome: audit.Outcome(outcome), Body: body,
	}, nil
}

// Tail implements [audit.Store].
func (s *Store) Tail(ctx context.Context, chain audit.Chain) (audit.Tail, error) {
	var tail audit.Tail
	err := s.read(ctx, "read audit tail", func(ctx context.Context, q *sqlcgen.Queries) error {
		t, _, err := lastSealed(ctx, q, chain)
		tail = t
		return err
	})
	return tail, err
}

// lastSealed returns the last sealed event of the chain, or the zero tail when none is sealed.
func lastSealed(ctx context.Context, q *sqlcgen.Queries, chain audit.Chain) (audit.Tail, bool, error) {
	if chain.IsInstance() {
		rows, err := q.ListLastInstanceAuditEvent(ctx)
		if err != nil {
			return audit.Tail{}, false, err
		}
		return tailOf(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash })
	}
	rows, err := q.ListLastAuditEvent(ctx, uuidBytes(chain.Org()))
	if err != nil {
		return audit.Tail{}, false, err
	}
	return tailOf(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash })
}

func tailOf[R any](rows []R, at func(i int) (int64, []byte)) (audit.Tail, bool, error) {
	if len(rows) == 0 {
		return audit.Tail{}, false, nil
	}
	seq, h := at(0)
	var t audit.Tail
	t.Sequence = seq
	copy(t.Hash[:], h)
	return t, true, nil
}

// CommitBlock implements [audit.Store]: one transaction in the writer. It re-reads the tail
// first and refuses the batch when another sealer moved it.
func (s *Store) CommitBlock(ctx context.Context, chain audit.Chain, expect audit.Tail, batch audit.Batch) error {
	return s.write(ctx, "commit audit block", func(ctx context.Context, q *sqlcgen.Queries) error {
		now, found, err := lastSealed(ctx, q, chain)
		if err != nil {
			return err
		}
		if !found {
			now = audit.Tail{}
		}
		if now != expect {
			return audit.ErrTailMoved
		}
		if chain.IsInstance() {
			return commitInstance(ctx, q, batch)
		}
		return commitOrg(ctx, q, chain.Org(), batch)
	})
}

func commitOrg(ctx context.Context, q *sqlcgen.Queries, org id.UUID, batch audit.Batch) error {
	orgID := uuidBytes(org)
	for i := range batch.Events {
		ev := &batch.Events[i]
		if err := q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
			OrgID: orgID, SequenceNumber: ev.Sequence, ID: uuidBytes(ev.Entry.ID), OccurredAt: micros(ev.Entry.OccurredAt),
			ActorPrincipalID: actorPrincipal(ev.Entry.Actor), ActorKind: actorKind(ev.Entry.Actor),
			ActionName: ev.Entry.Action, Outcome: string(ev.Entry.Outcome), BodyDocument: ev.Entry.Body,
			BodyDocumentFormat: audit.Format, PreviousHash: ev.PreviousHash[:], Hash: ev.Hash[:], CreatedAt: micros(batch.Block.SealedAt),
		}); err != nil {
			return fmt.Errorf("insert audit event: %w", err)
		}
	}
	b := batch.Block
	if err := q.InsertAuditBlock(ctx, sqlcgen.InsertAuditBlockParams{
		OrgID: orgID, ID: uuidBytes(b.ID), FirstSequenceNumber: b.FirstSequenceNumber,
		LastSequenceNumber: b.LastSequenceNumber, HeadHash: b.HeadHash[:], Mac: b.MAC[:],
		KeyVersion: b.KeyVersion, CreatedAt: micros(b.SealedAt),
	}); err != nil {
		return fmt.Errorf("insert audit block: %w", err)
	}
	for _, pid := range batch.PendingIDs {
		if err := q.DeleteAuditPending(ctx, orgID, uuidBytes(pid)); err != nil {
			return fmt.Errorf("delete audit pending: %w", err)
		}
	}
	return nil
}

func commitInstance(ctx context.Context, q *sqlcgen.Queries, batch audit.Batch) error {
	for i := range batch.Events {
		ev := &batch.Events[i]
		if err := q.InsertInstanceAuditEvent(ctx, sqlcgen.InsertInstanceAuditEventParams{
			SequenceNumber: ev.Sequence, ID: uuidBytes(ev.Entry.ID), OccurredAt: micros(ev.Entry.OccurredAt),
			ActorPrincipalID: actorPrincipal(ev.Entry.Actor), ActorKind: actorKind(ev.Entry.Actor),
			ActionName: ev.Entry.Action, Outcome: string(ev.Entry.Outcome), BodyDocument: ev.Entry.Body,
			BodyDocumentFormat: audit.Format, PreviousHash: ev.PreviousHash[:], Hash: ev.Hash[:], CreatedAt: micros(batch.Block.SealedAt),
		}); err != nil {
			return fmt.Errorf("insert instance audit event: %w", err)
		}
	}
	b := batch.Block
	if err := q.InsertInstanceAuditBlock(ctx, sqlcgen.InsertInstanceAuditBlockParams{
		ID: uuidBytes(b.ID), FirstSequenceNumber: b.FirstSequenceNumber, LastSequenceNumber: b.LastSequenceNumber,
		HeadHash: b.HeadHash[:], Mac: b.MAC[:], KeyVersion: b.KeyVersion, CreatedAt: micros(b.SealedAt),
	}); err != nil {
		return fmt.Errorf("insert instance audit block: %w", err)
	}
	for _, pid := range batch.PendingIDs {
		if err := q.DeleteInstanceAuditPending(ctx, uuidBytes(pid)); err != nil {
			return fmt.Errorf("delete instance audit pending: %w", err)
		}
	}
	return nil
}

// ListSealed implements [audit.Store].
func (s *Store) ListSealed(ctx context.Context, chain audit.Chain, after int64, limit int) ([]audit.Sealed, error) {
	var out []audit.Sealed
	err := s.read(ctx, "list audit events", func(ctx context.Context, q *sqlcgen.Queries) error {
		if chain.IsInstance() {
			rows, err := q.ListInstanceAuditEvents(ctx, after, int64(limit))
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				ev, err := sealedRow(chain, r.SequenceNumber, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument, r.PreviousHash, r.Hash)
				if err != nil {
					return err
				}
				out = append(out, ev)
			}
			return nil
		}
		rows, err := q.ListAuditEvents(ctx, uuidBytes(chain.Org()), after, int64(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			ev, err := sealedRow(chain, r.SequenceNumber, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument, r.PreviousHash, r.Hash)
			if err != nil {
				return err
			}
			out = append(out, ev)
		}
		return nil
	})
	return out, err
}

func sealedRow(chain audit.Chain, seq int64, rawID []byte, occurred int64, principal []byte, kind sql.NullString,
	action, outcome string, body, prev, hash []byte) (audit.Sealed, error) {
	e, err := storedEntry(chain, rawID, occurred, principal, kind, action, outcome, body)
	if err != nil {
		return audit.Sealed{}, err
	}
	var ev audit.Sealed
	ev.Sequence = seq
	ev.Entry = e
	copy(ev.PreviousHash[:], prev)
	copy(ev.Hash[:], hash)
	return ev, nil
}

// ListBlocks implements [audit.Store].
func (s *Store) ListBlocks(ctx context.Context, chain audit.Chain, after int64, limit int) ([]audit.Block, error) {
	var out []audit.Block
	err := s.read(ctx, "list audit blocks", func(ctx context.Context, q *sqlcgen.Queries) error {
		if chain.IsInstance() {
			rows, err := q.ListInstanceAuditBlocks(ctx, after, int64(limit))
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				b, err := blockRow(chain, r.ID, r.FirstSequenceNumber, r.LastSequenceNumber, r.HeadHash, r.Mac, r.KeyVersion, r.CreatedAt)
				if err != nil {
					return err
				}
				out = append(out, b)
			}
			return nil
		}
		rows, err := q.ListAuditBlocks(ctx, uuidBytes(chain.Org()), after, int64(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			b, err := blockRow(chain, r.ID, r.FirstSequenceNumber, r.LastSequenceNumber, r.HeadHash, r.Mac, r.KeyVersion, r.CreatedAt)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return nil
	})
	return out, err
}

func blockRow(chain audit.Chain, rawID []byte, first, last int64, head, mac []byte, version, sealed int64) (audit.Block, error) {
	bid, err := uuidOf[id.UUID](rawID)
	if err != nil {
		return audit.Block{}, err
	}
	b := audit.Block{
		ID: bid, Chain: chain, FirstSequenceNumber: first, LastSequenceNumber: last,
		KeyVersion: version, SealedAt: time.UnixMicro(sealed).UTC(),
	}
	copy(b.HeadHash[:], head)
	copy(b.MAC[:], mac)
	return b, nil
}
