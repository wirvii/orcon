// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package journal

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// States of an external operation (ADR-0012 section 2). They are the values of
// external_operation.state.
const (
	StatePrepared    = "PREPARED"
	StateDispatching = "DISPATCHING"
	StateSucceeded   = "SUCCEEDED"
	StateFailed      = "FAILED"
	StateUncertain   = "UNCERTAIN"
)

// Group sizes of the write path (ADR-0012 section 4, E-EOSC-2).
const (
	intentGroup    = 1000
	markGroup      = 1000
	outcomeGroup   = 512
	correlationLen = 26
)

// Defaults of [Config].
const (
	defaultAttemptTimeout = 30 * time.Second
	defaultGrace          = 5 * time.Second
	defaultConcurrency    = 16
	// The send guard is the margin kept before the dispatch deadline: a send
	// that would start later than this is not started at all (ADR-0012 section 5).
	sendGuard = time.Second
	// The recovery scan reads operations in pages of listPage rows.
	listPage = 512
)

// Errors of the journal.
var (
	// ErrNotPrepared means a DISPATCHING mark moved fewer rows than requested:
	// some items were not PREPARED. The mark is rolled back as a whole, so no
	// item of that window is left DISPATCHING, and nothing of it is sent.
	ErrNotPrepared = errors.New("journal: some operations were not PREPARED")
)

// Idempotency is how the external system deduplicates repeated requests
// (ADR-0012 section 1).
type Idempotency string

// Idempotency modes.
const (
	IdempotencyNone        Idempotency = "NONE"
	IdempotencyProviderKey Idempotency = "PROVIDER_KEY"
	IdempotencyNaturalKey  Idempotency = "NATURAL_KEY"
)

// Contract is the part of the operation contract the journal needs.
type Contract struct {
	Idempotency Idempotency
	// KeyTTL is how long the provider keeps a key. After it, a repeated request
	// with that key is no longer deduplicated and must not be sent (ADR-0012 §6).
	KeyTTL time.Duration
	// Reconcilable is true when the provider can look an operation up by its
	// correlation key (verification.reconcile).
	Reconcilable bool
}

// Attempt is one send of an item: the stored operation carries the correlation
// key and the retry epoch.
type Attempt struct {
	Operation store.ExternalOperation
	Contract  Contract
}

// Reply is what the provider answered. Outcome is one of the terminal states of
// an operation; anything else is read as UNCERTAIN.
type Reply struct {
	Outcome  string
	Response string
}

// Provider is the external system. Send performs one request; it runs outside
// every transaction, and its context ends at the attempt timeout. Lookup asks
// whether the operation took effect, for contracts with reconciliation.
type Provider interface {
	Send(ctx context.Context, attempt *Attempt) (Reply, error)
	Lookup(ctx context.Context, attempt *Attempt) (Reply, error)
}

// Config are the timing and concurrency bounds of a [Journal].
type Config struct {
	// AttemptTimeout bounds one send; it sets the dispatch deadline.
	AttemptTimeout time.Duration
	// Grace is the wait after the dispatch deadline before a recovery acts.
	Grace time.Duration
	// Concurrency is the number of sends in flight for one group.
	Concurrency int
}

// Summary counts the outcomes of one call.
type Summary struct {
	Succeeded int
	Failed    int
	Uncertain int
	// Pending are items left in DISPATCHING: not sent, or sent with an error
	// that a later recovery may repeat with the same key.
	Pending int
}

// Store is the set of store operations the journal uses. The storage adapters
// implement it; each call is one transaction of its own.
type Store interface {
	InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error
	ListExternalOperations(ctx context.Context, org orgscope.Org, node store.NodeExecutionID, fromItem int64, fromID store.ExternalOperationID, limit int) ([]store.ExternalOperation, error)
}

// Journal writes and recovers the operations of one store.
type Journal struct {
	db       Store
	provider Provider
	clock    clock.Clock
	cfg      Config
	// page is the page size of the recovery scan: listPage, unless a test shrinks it.
	page int
}

// New returns a journal over st and provider. Zero config fields take their
// defaults.
func New(db Store, provider Provider, clk clock.Clock, cfg Config) (*Journal, error) {
	if db == nil || provider == nil {
		return nil, errors.New("journal: a store and a provider are required")
	}
	if clk == nil {
		clk = clock.System{}
	}
	if cfg.AttemptTimeout <= 0 {
		cfg.AttemptTimeout = defaultAttemptTimeout
	}
	if cfg.Grace <= 0 {
		cfg.Grace = defaultGrace
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultConcurrency
	}
	return &Journal{db: db, provider: provider, clock: clk, cfg: cfg, page: listPage}, nil
}

// CorrelationKey derives the key of an item (ADR-0012 section 3): base32 of
// SHA-256 over the identity of the item and its retry epoch, cut to 26
// characters. The same item gets the same key in every attempt and worker.
func CorrelationKey(executionID, nodeID, scopePath, tokenID string, itemIndex, retryEpoch int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d", executionID, nodeID, scopePath, tokenID, itemIndex, retryEpoch)))
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return strings.ToLower(enc[:correlationLen])
}

// Prepare records the intent of every operation as PREPARED, before anything is
// sent. Groups of at most 1000 operations share one transaction. A repeated
// correlation key fails with the error of the store and nothing of that group
// is written.
func (j *Journal) Prepare(ctx context.Context, org orgscope.Org, ops []store.ExternalOperation) error {
	for start := 0; start < len(ops); start += intentGroup {
		group := ops[start:min(start+intentGroup, len(ops))]
		err := j.db.InTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
			return tx.AppendJournal(ctx, group)
		})
		if err != nil {
			return fmt.Errorf("journal: prepare: %w", err)
		}
	}
	return nil
}

// PrepareTx records the intent of the operations inside the caller's transaction,
// which is the checkpoint before an effect (ADR-0012 section 4, ADR-0013 section
// 6). The intent commits with the checkpoint, before anything is sent. The
// checkpoint may run again after a crash: an operation whose correlation key the
// journal already holds keeps its row, so the caller reloads the rows of the node
// execution rather than trusting the ids it generated.
func (j *Journal) PrepareTx(ctx context.Context, tx store.Tx, ops []store.ExternalOperation) error {
	for start := 0; start < len(ops); start += intentGroup {
		group := ops[start:min(start+intentGroup, len(ops))]
		if err := tx.InsertJournalIfAbsent(ctx, group); err != nil {
			return fmt.Errorf("journal: prepare: %w", err)
		}
	}
	return nil
}

// Dispatch sends the PREPARED operations of ops and records their outcomes. Items
// go out in windows of at most 1000: each window is marked DISPATCHING in its
// own transaction, committed before its first send, and its outcomes are
// recorded in groups of at most 512 afterwards. Sends inside a window run with
// the configured concurrency. A send that would start after the guard of the
// dispatch deadline is not started (ADR-0012 section 5).
func (j *Journal) Dispatch(ctx context.Context, org orgscope.Org, ops []store.ExternalOperation, c Contract) (Summary, error) {
	var sum Summary
	for start := 0; start < len(ops); start += markGroup {
		window := ops[start:min(start+markGroup, len(ops))]
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		if err := j.dispatchWindow(ctx, org, window, c, &sum); err != nil {
			return sum, err
		}
	}
	return sum, nil
}

func (j *Journal) dispatchWindow(ctx context.Context, org orgscope.Org, window []store.ExternalOperation, c Contract, sum *Summary) error {
	ids := make([]store.ExternalOperationID, len(window))
	for i := range window {
		ids[i] = window[i].ID
	}
	err := j.db.InTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
		moved, err := tx.MarkDispatching(ctx, ids, j.cfg.AttemptTimeout)
		if err != nil {
			return err
		}
		// A partial mark is rolled back, not committed: an item that did move
		// must not stay DISPATCHING while the rest of the window is refused.
		if moved != int64(len(window)) {
			return fmt.Errorf("%w: %d of %d", ErrNotPrepared, moved, len(window))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("journal: mark dispatching: %w", err)
	}
	// The mark is committed: the guard starts from the moment the worker observes it.
	observed := j.clock.Now()
	outcomes := j.send(ctx, window, c, observed)
	applied, err := j.record(ctx, org, outcomes, j.clock.Now(), sum)
	sum.Pending += len(window) - applied
	return err
}

// outcome is the result of one send, ready to be recorded.
type outcome struct {
	id       store.ExternalOperationID
	state    string
	response string
}

// send runs the sends of a window with bounded concurrency. It holds no
// transaction. The send guard is checked again each time a slot is acquired,
// immediately before the send, so an item that waited for a slot past the guard
// is not sent. Every send ends at the attempt timeout, measured from observed.
// Items whose send is not started or fails in a way a later recovery may repeat
// keep no outcome and stay in DISPATCHING.
func (j *Journal) send(ctx context.Context, window []store.ExternalOperation, c Contract, observed time.Time) []outcome {
	guard := observed.Add(j.cfg.AttemptTimeout - sendGuard)
	limit := observed.Add(j.cfg.AttemptTimeout)
	results := make([]*outcome, len(window))
	sem := make(chan struct{}, j.cfg.Concurrency)
	var wg sync.WaitGroup
	for i := range window {
		sem <- struct{}{}
		now := j.clock.Now()
		if !now.Before(guard) {
			<-sem
			break
		}
		wg.Add(1)
		go func(i int, timeout time.Duration) {
			defer wg.Done()
			defer func() { <-sem }()
			op := window[i]
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			reply, err := j.provider.Send(sctx, &Attempt{Operation: op, Contract: c})
			results[i] = classify(op.ID, c, reply, err)
		}(i, limit.Sub(now))
	}
	wg.Wait()
	out := make([]outcome, 0, len(window))
	for _, r := range results {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out
}

// classify maps a provider answer to the state the item moves to. A transport
// error on an item without a provider key is UNCERTAIN: the request may have
// been applied. With a provider key the item stays DISPATCHING, so that a
// recovery repeats the request with the same key.
func classify(id store.ExternalOperationID, c Contract, reply Reply, err error) *outcome {
	if err != nil {
		if c.Idempotency == IdempotencyProviderKey {
			return nil
		}
		return &outcome{id: id, state: StateUncertain}
	}
	switch reply.Outcome {
	case StateSucceeded, StateFailed, StateUncertain:
		return &outcome{id: id, state: reply.Outcome, response: reply.Response}
	default:
		return &outcome{id: id, state: StateUncertain, response: reply.Response}
	}
}

// record writes the outcomes in groups of at most 512, each in one transaction,
// and returns how many were applied. An outcome whose operation is no longer
// DISPATCHING is not applied and not counted.
func (j *Journal) record(ctx context.Context, org orgscope.Org, outcomes []outcome, now time.Time, sum *Summary) (int, error) {
	applied := 0
	for start := 0; start < len(outcomes); start += outcomeGroup {
		group := outcomes[start:min(start+outcomeGroup, len(outcomes))]
		err := j.db.InTx(ctx, org, func(ctx context.Context, tx store.Tx) error {
			for _, o := range group {
				ok, err := tx.ResolveOperation(ctx, o.id, StateDispatching, o.state, o.response, now)
				if err != nil {
					return err
				}
				if ok {
					applied++
					count(sum, o.state)
				}
			}
			return nil
		})
		if err != nil {
			return applied, fmt.Errorf("journal: record outcomes: %w", err)
		}
	}
	return applied, nil
}

func count(sum *Summary, state string) {
	switch state {
	case StateSucceeded:
		sum.Succeeded++
	case StateFailed:
		sum.Failed++
	default:
		sum.Uncertain++
	}
}
