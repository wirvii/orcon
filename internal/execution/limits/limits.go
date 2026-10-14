// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package limits holds the concurrency limits a worker applies before it runs an
// activation (ADR-0014 section 2, SPEC-001 R-FAIR-02). All limits must grant; the
// strictest one decides, and the one that refused is reported, so that the
// operator sees what throttles an execution (limit.throttled, throttle_kind).
//
// Two levels are enforced here, in process: the node level (item concurrency of
// one activation, a semaphore per node execution) and the worker level (tokens in
// flight and the memory budget, with admission stopping at 85 % of the budget).
// The execution level (leased tokens per execution) is checked in the claim
// transaction of the store, and the external-service level belongs to the
// dispatch cut, because it is shared between workers through a database table.
package limits

import (
	"errors"
	"sync"
)

// Level names a limit. The values are the public spelling of throttle_kind
// (ADR-0080 section 3).
type Level string

// Levels of the limits.
const (
	LevelNode            Level = "NODE"
	LevelExecution       Level = "EXECUTION"
	LevelWorker          Level = "WORKER"
	LevelExternalService Level = "EXTERNAL_SERVICE"
)

// AdmissionShare is the fraction of the memory budget at which a worker stops
// claiming new work (ADR-0014 section 2.3).
const AdmissionShare = 0.85

// ErrThrottled is returned by Acquire when a limit refused the work. The caller
// checks it with errors.Is and reads the Throttle for the level.
var ErrThrottled = errors.New("limits: throttled")

// Throttle says which limit refused and for which key.
type Throttle struct {
	Level Level
	Key   string
}

// Config are the caps of one worker. Zero values mean no cap at that level.
type Config struct {
	// NodeConcurrency caps the items of one node execution in flight.
	NodeConcurrency int
	// WorkerTokens caps the tokens a worker holds at once.
	WorkerTokens int
	// MemoryBudget is the bytes a worker may hold for in-flight work; zero means
	// no memory admission.
	MemoryBudget int64
}

// Limiter enforces the in-process limits of one worker. It is safe for
// concurrent use.
type Limiter struct {
	cfg Config

	mu       sync.Mutex
	tokens   int
	memory   int64
	perNode  map[string]int
	observer func(Throttle)
}

// New returns a limiter with cfg. The observer, when not nil, receives each
// throttle; it is called without the limiter's lock held.
func New(cfg Config, observer func(Throttle)) *Limiter {
	return &Limiter{cfg: cfg, perNode: map[string]int{}, observer: observer}
}

// Grant is what Acquire hands back: call Release exactly once when the work ends.
type Grant struct {
	l     *Limiter
	node  string
	bytes int64
	once  sync.Once
}

// Release returns the capacity of the grant. Calling it again does nothing.
func (g *Grant) Release() {
	g.once.Do(func() {
		g.l.mu.Lock()
		defer g.l.mu.Unlock()
		g.l.tokens--
		g.l.memory -= g.bytes
		g.l.perNode[g.node]--
		if g.l.perNode[g.node] == 0 {
			delete(g.l.perNode, g.node)
		}
	})
}

// Acquire admits one token of node (a node execution key) that needs bytes of
// memory. It checks the levels in strictest-first order: node, worker tokens,
// then memory admission. A refusal returns ErrThrottled and the level that
// refused; nothing is held in that case.
func (l *Limiter) Acquire(node string, bytes int64) (*Grant, error) {
	l.mu.Lock()
	var refused *Throttle
	switch {
	case l.cfg.NodeConcurrency > 0 && l.perNode[node] >= l.cfg.NodeConcurrency:
		refused = &Throttle{Level: LevelNode, Key: node}
	case l.cfg.WorkerTokens > 0 && l.tokens >= l.cfg.WorkerTokens:
		refused = &Throttle{Level: LevelWorker, Key: "worker"}
	case l.cfg.MemoryBudget > 0 && float64(l.memory+bytes) > AdmissionShare*float64(l.cfg.MemoryBudget):
		refused = &Throttle{Level: LevelWorker, Key: "memory"}
	}
	if refused != nil {
		l.mu.Unlock()
		if l.observer != nil {
			l.observer(*refused)
		}
		return nil, &refusal{throttle: *refused}
	}
	l.tokens++
	l.memory += bytes
	l.perNode[node]++
	l.mu.Unlock()
	return &Grant{l: l, node: node, bytes: bytes}, nil
}

// refusal carries the Throttle and matches ErrThrottled.
type refusal struct{ throttle Throttle }

func (r *refusal) Error() string { return "limits: throttled by " + string(r.throttle.Level) }

func (r *refusal) Is(target error) bool { return target == ErrThrottled }

// ThrottleOf returns the limit that refused an Acquire, when err is a refusal.
func ThrottleOf(err error) (Throttle, bool) {
	var r *refusal
	if errors.As(err, &r) {
		return r.throttle, true
	}
	return Throttle{}, false
}

// InFlight reports the tokens currently granted.
func (l *Limiter) InFlight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tokens
}
