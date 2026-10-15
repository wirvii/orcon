// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The idempotency records (ADR-0053 section 4, SPEC-005 Amendment 8 section 8-2). A change
// that takes an Idempotency-Key claims its record in its own transaction, so the change and
// the record commit together; the response is stored afterwards with CompleteIdempotency.

// claimIdempotency writes the record of the idempotency claim carried by ctx, in the
// transaction of the change, with the resource the change creates. Without a claim it does
// nothing. An expired record of the same key is reclaimed; a live one, or one that another
// transaction wrote first, fails with [idempotencyclaim.ErrTaken] and the caller rolls back.
func claimIdempotency(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, resourceRef string, now time.Time) error {
	claim, ok := idempotencyclaim.From(ctx)
	if !ok {
		return nil
	}
	scope, err := scoped(org)
	if err != nil {
		return err
	}
	existing, err := q.GetIdempotencyRecord(ctx, scope, claim.Requester, claim.Value)
	switch {
	case errors.Is(err, sql.ErrNoRows) || errors.Is(err, identity.ErrNotFound):
		err = q.InsertIdempotencyRecord(ctx, sqlcgen.InsertIdempotencyRecordParams{
			ID: uuidBytes(claim.ID), OrgID: scope, Requester: claim.Requester, IdempotencyKey: claim.Value,
			Fingerprint: claim.Fingerprint[:], Status: "IN_PROGRESS", ResourceReference: nullText(resourceRef),
			IsSecretIssued: 0, ExpiresAt: micros(now.Add(identity.IdempotencyTTL)), CreatedAt: micros(now), UpdatedAt: micros(now),
		})
		if errors.Is(err, identity.ErrConflict) {
			return idempotencyclaim.ErrTaken
		}
		if err != nil {
			return fmt.Errorf("claim idempotency key: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("claim idempotency key: %w", err)
	case instant(existing.ExpiresAt).After(now):
		return idempotencyclaim.ErrTaken
	}
	n, err := q.ClaimExpiredIdempotencyRecord(ctx, sqlcgen.ClaimExpiredIdempotencyRecordParams{
		ID: uuidBytes(claim.ID), Fingerprint: claim.Fingerprint[:], ResourceReference: nullText(resourceRef),
		ExpiresAt: micros(now.Add(identity.IdempotencyTTL)), CreatedAt: micros(now), UpdatedAt: micros(now),
		OrgID: scope, Requester: claim.Requester, IdempotencyKey: claim.Value, Now: micros(now),
	})
	if err != nil {
		return fmt.Errorf("reclaim idempotency key: %w", err)
	}
	if n == 0 {
		return idempotencyclaim.ErrTaken
	}
	return nil
}

// LookupIdempotency implements [identity.IdempotencyStore]. A record past its expiry is absent.
func (s *Store) LookupIdempotency(ctx context.Context, org orgscope.Org, key identity.IdempotencyKey, now time.Time) (identity.IdempotencyRecord, bool, error) {
	var out identity.IdempotencyRecord
	found := false
	err := s.read(ctx, "lookup idempotency", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetIdempotencyRecord(ctx, scope, key.Requester, key.Value)
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, identity.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if !instant(row.ExpiresAt).After(now) {
			return nil
		}
		copy(out.Fingerprint[:], row.Fingerprint)
		out.Completed = row.Status == "COMPLETED"
		out.ResourceRef = row.ResourceReference.String
		out.Response = identity.IdempotencyResponse{
			Status: int(row.ResponseStatus.Int64), ContentType: row.ResponseContentType.String,
			Location: row.ResponseLocation.String, ETag: row.ResponseEtag.String, Body: row.ResponseBody,
			SecretIssued: row.IsSecretIssued != 0,
		}
		out.CreatedAt = instant(row.CreatedAt)
		found = true
		return nil
	})
	return out, found, err
}

// CompleteIdempotency implements [identity.IdempotencyStore]. It completes the record that the
// transaction of the change claimed; without one in progress it stores a completed record.
func (s *Store) CompleteIdempotency(ctx context.Context, org orgscope.Org, key identity.IdempotencyKey, c identity.IdempotencyCompletion, now time.Time) error {
	return s.write(ctx, "complete idempotency", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		n, err := q.MarkIdempotencyRecordCompleted(ctx, sqlcgen.MarkIdempotencyRecordCompletedParams{
			ResponseStatus: sql.NullInt64{Int64: int64(c.Response.Status), Valid: true}, ResponseContentType: nullText(c.Response.ContentType),
			ResponseLocation: nullText(c.Response.Location), ResponseEtag: nullText(c.Response.ETag), ResponseBody: c.Response.Body,
			IsSecretIssued: boolInt(c.Response.SecretIssued),
			UpdatedAt:      micros(now), OrgID: scope, Requester: key.Requester, IdempotencyKey: key.Value,
		})
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		return q.InsertIdempotencyRecord(ctx, sqlcgen.InsertIdempotencyRecordParams{
			ID: uuidBytes(c.ID), OrgID: scope, Requester: key.Requester, IdempotencyKey: key.Value,
			Fingerprint: c.Fingerprint[:], Status: "COMPLETED", ResponseStatus: sql.NullInt64{Int64: int64(c.Response.Status), Valid: true},
			ResponseContentType: nullText(c.Response.ContentType), ResponseLocation: nullText(c.Response.Location),
			ResponseEtag: nullText(c.Response.ETag), ResponseBody: c.Response.Body, IsSecretIssued: boolInt(c.Response.SecretIssued),
			ExpiresAt: micros(now.Add(identity.IdempotencyTTL)), CreatedAt: micros(now), UpdatedAt: micros(now),
		})
	})
}
