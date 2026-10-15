// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The audit area writes a pending entry in the transaction of its action and seals
// the chains in the organization context of each chain (ADR-0042, orcon-db TI-6).
// The sealer reads the organizations that have pending rows through the engine
// function orcon_engine.list_audit_pending_orgs; every other statement runs under the
// application role with the organization set, so row-level security holds.

// insertPendingOrg writes one pending entry of an organization chain inside the transaction of the action.
func insertPendingOrg(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, e audit.Entry) error {
	if !e.Chain.Valid() || e.Chain.IsInstance() || e.Chain.Org() != org.UUID() {
		return fmt.Errorf("audit entry: %w", audit.ErrInvalidEntry)
	}
	if err := q.InsertAuditPending(ctx, sqlcgen.InsertAuditPendingParams{
		ID: pgUUID(e.ID), OrgID: pgUUID(e.Chain.Org()), OccurredAt: timestamp(e.OccurredAt),
		ActorPrincipalID: pgPrincipal(e.Actor), ActorKind: pgKind(e.Actor), ActionName: e.Action,
		Outcome: string(e.Outcome), BodyDocument: e.Body, BodyDocumentFormat: audit.Format,
		CreatedAt: timestamp(e.OccurredAt), UpdatedAt: timestamp(e.OccurredAt),
	}); err != nil {
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
		ID: pgUUID(e.ID), OccurredAt: timestamp(e.OccurredAt), ActorPrincipalID: pgPrincipal(e.Actor),
		ActorKind: pgKind(e.Actor), ActionName: e.Action, Outcome: string(e.Outcome),
		BodyDocument: e.Body, BodyDocumentFormat: audit.Format,
		CreatedAt: timestamp(e.OccurredAt), UpdatedAt: timestamp(e.OccurredAt),
	}); err != nil {
		return fmt.Errorf("insert instance audit pending: %w", err)
	}
	return nil
}

func pgPrincipal(a audit.Actor) pgtype.UUID {
	if a.IsAnonymous() {
		return pgtype.UUID{}
	}
	return pgUUID(a.PrincipalID)
}

func pgKind(a audit.Actor) pgtype.Text {
	if a.IsAnonymous() {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(a.Kind), Valid: true}
}

// AppendInstanceEntry implements [identity.AuditLog]: a denied or failed attempt is
// audited in its own transaction (R-AUD-02).
func (s *Store) AppendInstanceEntry(ctx context.Context, entry audit.Entry) error {
	return s.instanceTx(ctx, "append audit entry", func(ctx context.Context, q *sqlcgen.Queries) error {
		return insertPendingInstance(ctx, q, entry)
	})
}

// PendingChains implements [audit.Store]. The organizations come from the engine
// function, because the application role sees one organization at a time.
func (s *Store) PendingChains(ctx context.Context, limit int) ([]audit.Chain, error) {
	const op = "list pending audit chains"
	chains, err := s.pendingOrgChains(ctx, op, limit)
	if err != nil {
		return nil, err
	}
	if len(chains) < limit {
		var n int64
		if err := s.instance(ctx, op, func(ctx context.Context, q *sqlcgen.Queries) error {
			var err error
			n, err = q.CountInstanceAuditPending(ctx)
			return err
		}); err != nil {
			return nil, err
		}
		if n > 0 {
			chains = append(chains, audit.InstanceChain())
		}
	}
	return chains, nil
}

// pendingOrgChains reads the organizations with pending rows through the engine function.
func (s *Store) pendingOrgChains(ctx context.Context, op string, limit int) ([]audit.Chain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("%s: %w", op, ErrClosed)
	}
	rows, err := s.pool.Query(ctx, "SELECT org_id FROM orcon_engine.list_audit_pending_orgs($1)", pgLimit(limit))
	if err != nil {
		return nil, mapError(op, err)
	}
	defer rows.Close()
	var chains []audit.Chain
	for rows.Next() {
		var org pgtype.UUID
		if err := rows.Scan(&org); err != nil {
			return nil, mapError(op, err)
		}
		chains = append(chains, audit.OrganizationChain(fromPGUUID[id.UUID](org)))
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(op, err)
	}
	return chains, nil
}

// ListPending implements [audit.Store].
func (s *Store) ListPending(ctx context.Context, chain audit.Chain, limit int) ([]audit.Entry, error) {
	const op = "list pending audit"
	var out []audit.Entry
	if chain.IsInstance() {
		err := s.instance(ctx, op, func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListInstanceAuditPending(ctx, pgLimit(limit))
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
	err := s.inOrg(ctx, op, chain.Org(), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAuditPending(ctx, pgUUID(chain.Org()), pgLimit(limit))
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

// pgLimit converts a page size to the int32 the queries take, clamped to the range of int32.
func pgLimit(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// storedEntry rebuilds an entry from the columns the adapter reads. A stored actor of a kind
// the format does not know is refused: the CHECK constraint of the table already forbids it.
func storedEntry(chain audit.Chain, rawID pgtype.UUID, occurred pgtype.Timestamptz, principal pgtype.UUID, kind pgtype.Text,
	action, outcome string, body []byte) (audit.Entry, error) {
	var actor audit.Actor
	if principal.Valid {
		if !audit.ActorKind(kind.String).Valid() {
			return audit.Entry{}, fmt.Errorf("stored actor kind %q: %w", kind.String, audit.ErrInvalidEntry)
		}
		actor = audit.Actor{PrincipalID: fromPGUUID[id.UUID](principal), Kind: audit.ActorKind(kind.String)}
	}
	return audit.Entry{
		ID: fromPGUUID[id.UUID](rawID), Chain: chain, OccurredAt: instant(occurred), Actor: actor,
		Action: action, Outcome: audit.Outcome(outcome), Body: body,
	}, nil
}

// Tail implements [audit.Store].
func (s *Store) Tail(ctx context.Context, chain audit.Chain) (audit.Tail, error) {
	var tail audit.Tail
	if chain.IsInstance() {
		err := s.instance(ctx, "read audit tail", func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListLastInstanceAuditEvent(ctx)
			if err != nil {
				return err
			}
			tail = tailOfRows(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash })
			return nil
		})
		return tail, err
	}
	err := s.inOrg(ctx, "read audit tail", chain.Org(), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListLastAuditEvent(ctx, pgUUID(chain.Org()))
		if err != nil {
			return err
		}
		tail = tailOfRows(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash })
		return nil
	})
	return tail, err
}

func tailOfRows[R any](rows []R, at func(i int) (int64, []byte)) audit.Tail {
	if len(rows) == 0 {
		return audit.Tail{}
	}
	seq, h := at(0)
	t := audit.Tail{Sequence: seq}
	copy(t.Hash[:], h)
	return t
}

// CommitBlock implements [audit.Store]. The tail is read again inside the transaction; a
// racing sealer that committed first makes the insert conflict, which is reported as
// [audit.ErrTailMoved] so the pass is skipped.
func (s *Store) CommitBlock(ctx context.Context, chain audit.Chain, expect audit.Tail, batch audit.Batch) error {
	const op = "commit audit block"
	var err error
	if chain.IsInstance() {
		err = s.instanceTx(ctx, op, func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListLastInstanceAuditEvent(ctx)
			if err != nil {
				return err
			}
			if tailOfRows(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash }) != expect {
				return audit.ErrTailMoved
			}
			return commitInstance(ctx, q, batch)
		})
	} else {
		err = s.inOrg(ctx, op, chain.Org(), false, func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListLastAuditEvent(ctx, pgUUID(chain.Org()))
			if err != nil {
				return err
			}
			if tailOfRows(rows, func(i int) (int64, []byte) { return rows[i].SequenceNumber, rows[i].Hash }) != expect {
				return audit.ErrTailMoved
			}
			return commitOrg(ctx, q, chain.Org(), batch)
		})
	}
	if errors.Is(err, flow.ErrConflict) {
		return audit.ErrTailMoved
	}
	return err
}

func commitOrg(ctx context.Context, q *sqlcgen.Queries, org id.UUID, batch audit.Batch) error {
	for i := range batch.Events {
		ev := &batch.Events[i]
		if err := q.InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
			OrgID: pgUUID(org), SequenceNumber: ev.Sequence, ID: pgUUID(ev.Entry.ID), OccurredAt: timestamp(ev.Entry.OccurredAt),
			ActorPrincipalID: pgPrincipal(ev.Entry.Actor), ActorKind: pgKind(ev.Entry.Actor),
			ActionName: ev.Entry.Action, Outcome: string(ev.Entry.Outcome), BodyDocument: ev.Entry.Body,
			BodyDocumentFormat: audit.Format, PreviousHash: ev.PreviousHash[:], Hash: ev.Hash[:],
			CreatedAt: timestamp(batch.Block.SealedAt),
		}); err != nil {
			return fmt.Errorf("insert audit event: %w", err)
		}
	}
	b := batch.Block
	if err := q.InsertAuditBlock(ctx, sqlcgen.InsertAuditBlockParams{
		OrgID: pgUUID(org), ID: pgUUID(b.ID), FirstSequenceNumber: b.FirstSequenceNumber,
		LastSequenceNumber: b.LastSequenceNumber, HeadHash: b.HeadHash[:], Mac: b.MAC[:],
		KeyVersion: b.KeyVersion, CreatedAt: timestamp(b.SealedAt),
	}); err != nil {
		return fmt.Errorf("insert audit block: %w", err)
	}
	for _, pid := range batch.PendingIDs {
		if err := q.DeleteAuditPending(ctx, pgUUID(org), pgUUID(pid)); err != nil {
			return fmt.Errorf("delete audit pending: %w", err)
		}
	}
	return nil
}

func commitInstance(ctx context.Context, q *sqlcgen.Queries, batch audit.Batch) error {
	for i := range batch.Events {
		ev := &batch.Events[i]
		if err := q.InsertInstanceAuditEvent(ctx, sqlcgen.InsertInstanceAuditEventParams{
			SequenceNumber: ev.Sequence, ID: pgUUID(ev.Entry.ID), OccurredAt: timestamp(ev.Entry.OccurredAt),
			ActorPrincipalID: pgPrincipal(ev.Entry.Actor), ActorKind: pgKind(ev.Entry.Actor),
			ActionName: ev.Entry.Action, Outcome: string(ev.Entry.Outcome), BodyDocument: ev.Entry.Body,
			BodyDocumentFormat: audit.Format, PreviousHash: ev.PreviousHash[:], Hash: ev.Hash[:],
			CreatedAt: timestamp(batch.Block.SealedAt),
		}); err != nil {
			return fmt.Errorf("insert instance audit event: %w", err)
		}
	}
	b := batch.Block
	if err := q.InsertInstanceAuditBlock(ctx, sqlcgen.InsertInstanceAuditBlockParams{
		ID: pgUUID(b.ID), FirstSequenceNumber: b.FirstSequenceNumber, LastSequenceNumber: b.LastSequenceNumber,
		HeadHash: b.HeadHash[:], Mac: b.MAC[:], KeyVersion: b.KeyVersion, CreatedAt: timestamp(b.SealedAt),
	}); err != nil {
		return fmt.Errorf("insert instance audit block: %w", err)
	}
	for _, pid := range batch.PendingIDs {
		if err := q.DeleteInstanceAuditPending(ctx, pgUUID(pid)); err != nil {
			return fmt.Errorf("delete instance audit pending: %w", err)
		}
	}
	return nil
}

// ListSealed implements [audit.Store].
func (s *Store) ListSealed(ctx context.Context, chain audit.Chain, after int64, limit int) ([]audit.Sealed, error) {
	const op = "list audit events"
	var out []audit.Sealed
	add := func(seq int64, rawID pgtype.UUID, occurred pgtype.Timestamptz, principal pgtype.UUID, kind pgtype.Text,
		action, outcome string, body, prev, hash []byte) error {
		e, err := storedEntry(chain, rawID, occurred, principal, kind, action, outcome, body)
		if err != nil {
			return err
		}
		ev := audit.Sealed{Sequence: seq, Entry: e}
		copy(ev.PreviousHash[:], prev)
		copy(ev.Hash[:], hash)
		out = append(out, ev)
		return nil
	}
	if chain.IsInstance() {
		err := s.instance(ctx, op, func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListInstanceAuditEvents(ctx, after, pgLimit(limit))
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				if err := add(r.SequenceNumber, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument, r.PreviousHash, r.Hash); err != nil {
					return err
				}
			}
			return nil
		})
		return out, err
	}
	err := s.inOrg(ctx, op, chain.Org(), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAuditEvents(ctx, pgUUID(chain.Org()), after, pgLimit(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			if err := add(r.SequenceNumber, r.ID, r.OccurredAt, r.ActorPrincipalID, r.ActorKind, r.ActionName, r.Outcome, r.BodyDocument, r.PreviousHash, r.Hash); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// ListBlocks implements [audit.Store].
func (s *Store) ListBlocks(ctx context.Context, chain audit.Chain, after int64, limit int) ([]audit.Block, error) {
	const op = "list audit blocks"
	var out []audit.Block
	add := func(rawID pgtype.UUID, first, last int64, head, mac []byte, version int64, sealed pgtype.Timestamptz) {
		b := audit.Block{
			ID: fromPGUUID[id.UUID](rawID), Chain: chain, FirstSequenceNumber: first, LastSequenceNumber: last,
			KeyVersion: version, SealedAt: instant(sealed),
		}
		copy(b.HeadHash[:], head)
		copy(b.MAC[:], mac)
		out = append(out, b)
	}
	if chain.IsInstance() {
		err := s.instance(ctx, op, func(ctx context.Context, q *sqlcgen.Queries) error {
			rows, err := q.ListInstanceAuditBlocks(ctx, after, pgLimit(limit))
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				add(r.ID, r.FirstSequenceNumber, r.LastSequenceNumber, r.HeadHash, r.Mac, r.KeyVersion, r.CreatedAt)
			}
			return nil
		})
		return out, err
	}
	err := s.inOrg(ctx, op, chain.Org(), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAuditBlocks(ctx, pgUUID(chain.Org()), after, pgLimit(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			add(r.ID, r.FirstSequenceNumber, r.LastSequenceNumber, r.HeadHash, r.Mac, r.KeyVersion, r.CreatedAt)
		}
		return nil
	})
	return out, err
}
