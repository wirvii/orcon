// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// NodeEnv is the nondeterminism of one node activation: the instant the
// activation started and a seed. `$now`, `Date.now()`, `Math.random()` and
// generated ids derive from them (ADR-0013 section 7, ADR-0018 section 5), so
// an activation recomputed from a recorded environment produces the same
// output. A zero Time means the interpreter had no clock.
type NodeEnv struct {
	Time time.Time
	Seed uint64
}

// seedOf derives the default seed of an activation from its node execution id,
// so recomputing a step without a record still yields the same seed. The id is
// the leading half of a SHA-256, so its first 64 bits are uniformly spread.
func seedOf(ne NodeExecutionID) uint64 {
	s := string(ne)
	var seed uint64
	n := 0
	for i := 0; i < len(s) && n < 16; i++ {
		c := s[i]
		var d uint64
		switch {
		case c >= '0' && c <= '9':
			d = uint64(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint64(c-'a') + 10
		case c == '-':
			continue
		default:
			return fallbackSeed(s)
		}
		seed = seed<<4 | d
		n++
	}
	if n < 16 {
		return fallbackSeed(s)
	}
	return seed
}

// fallbackSeed is FNV-1a for node execution ids that are not hash ids, which
// only tests construct.
func fallbackSeed(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// envOf returns the environment of an activation.
func (r *run) envOf(ne NodeExecutionID) NodeEnv {
	if r.rec != nil {
		return r.rec.env(ne)
	}
	env := NodeEnv{Seed: seedOf(ne)}
	if c := r.in.deps.Clock; c != nil {
		env.Time = clock.Micros(c.Now())
	}
	return env
}

// EnvRecord is the environment of the activation of a node execution, to be
// persisted at the next confirmation point.
type EnvRecord struct {
	NodeExecution NodeExecutionID
	Env           NodeEnv
}

// ActivationEmission is a live token an activation sends along an edge.
type ActivationEmission struct {
	Edge    flow.EdgeID
	Payload PayloadRef
}

// Activation is the outcome of one activation of a node that calls a port:
// what it emitted, or the failure it raised, and the batches it stored. It is
// the one part of a step that cannot be recomputed for free (it may have
// called a connector), so a worker keeps it when it has to apply the same step
// again to a newer state of the execution (recomputation, ADR-0013 section 7).
type Activation struct {
	NodeExecution NodeExecutionID
	Emissions     []ActivationEmission
	// Failure is set when the activation failed the node. Its Cause is dropped.
	Failure *Failure
	Puts    []PayloadWrite
	// Env is the environment the activation made, when it made one: replaying
	// the activation keeps the record that goes with it.
	Env *NodeEnv
}

// needsPorts reports whether activating a node of this plan calls a port.
func (n *planNode) needsPorts() bool {
	switch n.kind {
	case kindActivity, kindRouting, kindExclusive, kindInclusive:
		return true
	case kindStart, kindEnd, kindParallel, kindLoop:
		return false
	default:
		return false
	}
}

// recorder keeps the activations and environments of a session.
type recorder struct {
	plan *Plan
	clk  clock.Clock
	// oracle holds the activations to replay instead of calling ports.
	oracle map[NodeExecutionID]*Activation
	// acts are the activations that ran or replayed in this session.
	acts   []*Activation
	byNode map[NodeExecutionID]*Activation
	// known holds the environments recorded by earlier sessions.
	known   map[NodeExecutionID]NodeEnv
	created []EnvRecord
	// queued lists the tokens made available to claim, in order.
	queued []TokenID
}

func newRecorder(plan *Plan, clk clock.Clock, opt SessionOptions) *recorder {
	return &recorder{
		plan: plan, clk: clk, oracle: opt.Oracle, known: opt.Known,
		byNode: make(map[NodeExecutionID]*Activation),
	}
}

// ready reports whether the node execution can run: it is replayed, or the
// session is live.
func (rc *recorder) ready(ne NodeExecutionID) bool {
	if rc.oracle == nil {
		return true
	}
	_, ok := rc.oracle[ne]
	return ok
}

// env returns the recorded environment of a node execution, or makes and
// remembers a new one.
func (rc *recorder) env(ne NodeExecutionID) NodeEnv {
	if env, ok := rc.known[ne]; ok {
		return env
	}
	env := NodeEnv{Seed: seedOf(ne)}
	if rc.clk != nil {
		env.Time = clock.Micros(rc.clk.Now())
	}
	if rc.known == nil {
		rc.known = make(map[NodeExecutionID]NodeEnv)
	}
	rc.known[ne] = env
	rc.created = append(rc.created, EnvRecord{NodeExecution: ne, Env: env})
	return env
}

// createdEnv returns the environment this session made for a node execution.
func (rc *recorder) createdEnv(ne NodeExecutionID) *NodeEnv {
	for i := len(rc.created) - 1; i >= 0; i-- {
		if rc.created[i].NodeExecution == ne {
			env := rc.created[i].Env
			return &env
		}
	}
	return nil
}

// activateNode activates a node. A durable session replays a recorded
// activation, or records the one it runs.
func (r *run) activateNode(ctx context.Context, t *token, n *planNode, ne *nodeExec) ([]emission, error) {
	if r.rec == nil || !n.needsPorts() {
		return r.activate(ctx, t, n, ne)
	}
	if act, ok := r.rec.oracle[ne.id]; ok {
		return r.replay(ctx, act)
	}
	mark := r.owned.mark()
	ems, err := r.activate(ctx, t, n, ne)
	act := &Activation{NodeExecution: ne.id, Puts: r.owned.since(mark)}
	var fe *failureError
	switch {
	case err == nil:
		act.Emissions = make([]ActivationEmission, len(ems))
		for i, em := range ems {
			act.Emissions[i] = ActivationEmission{Edge: r.plan.edges[em.edge].edge.ID, Payload: em.payload}
		}
	case errors.As(err, &fe):
		f := *fe.failure
		f.Cause = nil
		act.Failure = &f
	default:
		return nil, err
	}
	act.Env = r.rec.createdEnv(ne.id)
	r.rec.acts = append(r.rec.acts, act)
	r.rec.byNode[ne.id] = act
	return ems, err
}

// replay applies a recorded activation: it stores the batches again and
// returns the emissions, or raises the recorded failure.
func (r *run) replay(ctx context.Context, act *Activation) ([]emission, error) {
	for _, w := range act.Puts {
		if err := r.put(ctx, w); err != nil {
			return nil, err
		}
	}
	r.rec.acts = append(r.rec.acts, act)
	r.rec.byNode[act.NodeExecution] = act
	if act.Env != nil {
		r.rec.created = append(r.rec.created, EnvRecord{NodeExecution: act.NodeExecution, Env: *act.Env})
	}
	if act.Failure != nil {
		f := *act.Failure
		return nil, raise(&f)
	}
	ems := make([]emission, len(act.Emissions))
	for i, e := range act.Emissions {
		idx, ok := r.plan.edgeIndex[e.Edge]
		if !ok {
			return nil, fmt.Errorf("%w: recorded activation of %s names edge %q, which the flow does not have", ErrInvalidRequest, act.NodeExecution, e.Edge)
		}
		ems[i] = emission{edge: idx, payload: e.Payload}
	}
	return ems, nil
}

// PayloadKind says what a stored batch belongs to.
type PayloadKind uint8

// Payload kinds.
const (
	// PayloadOutput is the batch a node execution emitted on a port.
	PayloadOutput PayloadKind = iota + 1
	// PayloadInput is a batch a token starts with: the execution input, the
	// combined input of a join continuation or of a loop iteration that merged
	// its returns.
	PayloadInput
)

// PayloadWrite is a batch to store under a reference, with its owner.
type PayloadWrite struct {
	Ref  PayloadRef
	Kind PayloadKind
	// NodeExecution and Port name the owner of a PayloadOutput.
	NodeExecution NodeExecutionID
	Port          string
	// Token names the owner of a PayloadInput.
	Token TokenID
	Batch Batch
}

// ownedPayloads is the payload store of a durable session: it learns who owns
// each batch.
type ownedPayloads interface {
	PutOwned(ctx context.Context, w PayloadWrite) error
	mark() int
	since(mark int) []PayloadWrite
}

// put stores a batch with its owner.
func (r *run) put(ctx context.Context, w PayloadWrite) error {
	if r.owned != nil {
		return r.owned.PutOwned(ctx, w)
	}
	return r.payloads.Put(ctx, w.Ref, w.Batch)
}

// sessionPayloads shadows the durable payload store with the batches a
// session wrote and no commit holds yet.
type sessionPayloads struct {
	base    Payloads
	written map[PayloadRef]Batch
	log     []PayloadWrite
}

func newSessionPayloads(base Payloads) *sessionPayloads {
	return &sessionPayloads{base: base, written: make(map[PayloadRef]Batch)}
}

// Put implements [Payloads]. A batch stored without an owner cannot be placed
// in storage, so the interpreter never calls it on a session.
func (p *sessionPayloads) Put(context.Context, PayloadRef, Batch) error {
	return fmt.Errorf("%w: a batch needs an owner in a durable session", ErrPort)
}

// PutOwned stores a batch once per reference.
func (p *sessionPayloads) PutOwned(ctx context.Context, w PayloadWrite) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put payload %s: %w", w.Ref, err)
	}
	if _, ok := p.written[w.Ref]; ok {
		return nil
	}
	p.written[w.Ref] = w.Batch
	p.log = append(p.log, w)
	return nil
}

// Get implements [Payloads].
func (p *sessionPayloads) Get(ctx context.Context, ref PayloadRef) (Batch, error) {
	if b, ok := p.written[ref]; ok {
		return b, nil
	}
	if p.base == nil {
		return Batch{}, fmt.Errorf("get payload %s: %w", ref, ErrPayloadNotFound)
	}
	return p.base.Get(ctx, ref)
}

func (p *sessionPayloads) mark() int { return len(p.log) }

func (p *sessionPayloads) since(mark int) []PayloadWrite {
	return append([]PayloadWrite(nil), p.log[mark:]...)
}

// String renders the environment for logs and tests.
func (e NodeEnv) String() string {
	return e.Time.Format(time.RFC3339Nano) + "/" + strconv.FormatUint(e.Seed, 16)
}
