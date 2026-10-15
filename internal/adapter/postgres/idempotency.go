// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The idempotency records (ADR-0053 section 4, SPEC-005 Amendment 8 section 8-2). A change
// that takes an Idempotency-Key claims its record in its own transaction, so the change and
// the record commit together; the response is stored afterwards with CompleteIdempotency.

// httpStatus is the column value of an HTTP status, a three-digit code.
func httpStatus(status int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(status), Valid: true} //nolint:gosec // an HTTP status is a three-digit code, which fits in int32
}

// isUniqueViolation reports whether err is a unique-constraint violation of the driver.
func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == codeUnique
}

// claimIdempotency writes the record of the idempotency claim carried by ctx, in the
// transaction of the change, with the resource the change creates. Without a claim it does
// nothing. An expired record of the same key is reclaimed; a live one, or one that another
// transaction wrote first, fails with [idempotencyclaim.ErrTaken] and the caller rolls back.
func claimIdempotency(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, resourceRef string, now time.Time) error {
	claim, ok := idempotencyclaim.From(ctx)
	if !ok {
		return nil
	}
	scope := pgUUID(org.UUID())
	existing, err := q.GetIdempotencyRecord(ctx, scope, claim.Requester, claim.Value)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = q.InsertIdempotencyRecord(ctx, sqlcgen.InsertIdempotencyRecordParams{
			ID: pgUUID(claim.ID), OrgID: scope, Requester: claim.Requester, IdempotencyKey: claim.Value,
			Fingerprint: claim.Fingerprint[:], Status: "IN_PROGRESS", ResourceReference: nullText(resourceRef),
			IsSecretIssued: false, ExpiresAt: timestamp(now.Add(identity.IdempotencyTTL)), CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		})
		// Two requests that carry the same key race to insert the record; the loser meets the unique
		// index of the key once the winner commits. That is the key being taken, not a conflict of
		// the resource (BL-127): translate it here, before the generic mapping of constraint violations.
		if errors.Is(err, identity.ErrConflict) || isUniqueViolation(err) {
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
		ID: pgUUID(claim.ID), Fingerprint: claim.Fingerprint[:], ResourceReference: nullText(resourceRef),
		ExpiresAt: timestamp(now.Add(identity.IdempotencyTTL)), CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		OrgID: scope, Requester: claim.Requester, IdempotencyKey: claim.Value, Now: timestamp(now),
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
	err := s.inScope(ctx, "lookup idempotency", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetIdempotencyRecord(ctx, pgUUID(org.UUID()), key.Requester, key.Value)
		if errors.Is(err, pgx.ErrNoRows) {
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
			Status: int(row.ResponseStatus.Int32), ContentType: row.ResponseContentType.String,
			Location: row.ResponseLocation.String, ETag: row.ResponseEtag.String, Body: row.ResponseBody,
			SecretIssued: row.IsSecretIssued,
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
	return s.inScope(ctx, "complete idempotency", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkIdempotencyRecordCompleted(ctx, sqlcgen.MarkIdempotencyRecordCompletedParams{
			ResponseStatus: httpStatus(c.Response.Status), ResponseContentType: nullText(c.Response.ContentType),
			ResponseLocation: nullText(c.Response.Location), ResponseEtag: nullText(c.Response.ETag), ResponseBody: c.Response.Body,
			IsSecretIssued: c.Response.SecretIssued,
			UpdatedAt:      timestamp(now), OrgID: pgUUID(org.UUID()), Requester: key.Requester, IdempotencyKey: key.Value,
		})
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		return q.InsertIdempotencyRecord(ctx, sqlcgen.InsertIdempotencyRecordParams{
			ID: pgUUID(c.ID), OrgID: pgUUID(org.UUID()), Requester: key.Requester, IdempotencyKey: key.Value,
			Fingerprint: c.Fingerprint[:], Status: "COMPLETED", ResponseStatus: httpStatus(c.Response.Status),
			ResponseContentType: nullText(c.Response.ContentType), ResponseLocation: nullText(c.Response.Location),
			ResponseEtag: nullText(c.Response.ETag), ResponseBody: c.Response.Body, IsSecretIssued: c.Response.SecretIssued,
			ExpiresAt: timestamp(now.Add(identity.IdempotencyTTL)), CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		})
	})
}
