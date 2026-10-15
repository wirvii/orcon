// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/exprflow"
	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/chain"
	"github.com/wirvii/orcon/internal/connector/declarative"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
)

// WebhookNodeType is the trigger node type of a public webhook address.
const WebhookNodeType = "orcon.webhook@1"

// webhookBodyCap is the size of one event the webhook trigger accepts.
const webhookBodyCap = 1 << 20

// ErrNoCredentialStore means a node names a credential and this process has no
// credential store to resolve it from. The credential store is a later cut of
// SPEC-003; until it arrives, no node can use a credential.
var ErrNoCredentialStore = errors.New("no credential store is available in this build")

// engineStore is everything the composition root takes from the database: the
// store of the execution engine and of the trigger registry, the flow reads of
// the trigger service, the journal of external effects and the rate buckets.
// Both adapters implement it.
type engineStore interface {
	durable.Store
	durable.Revisions
	trigger.Store
	trigger.FlowSource
	journal.Store
	store.LimitBuckets
}

// engineOptions are the choices of the process that the engine wiring takes
// from outside: which roles run the workers, the sandbox of the expressions,
// and the network policy of the outgoing calls.
type engineOptions struct {
	role    config.Role
	logger  *slog.Logger
	version string
	// sandbox runs the expressions the fast path cannot; nil means those fail.
	sandbox expr.Sandbox
	// senderPolicy returns the egress policy of one destination of orcon.http@1.
	senderPolicy func(*url.URL) egress.Policy
	// credentials resolves the credential a node names; nil uses the refusing
	// resolver.
	credentials credentialResolver
	// models resolves the model of the AI nodes; nil uses the production
	// resolver, which reaches each provider through the egress client.
	models chain.Models
}

// engine is the execution engine of a process: the durable engine that runs the
// executions, and the trigger service that starts them for accepted events.
type engine struct {
	durable *durable.Engine
	trigger *trigger.Service
	logger  *slog.Logger
	// workers is true when the process runs the claim loops (standalone).
	workers bool
}

// newEngine wires the engine over a store. It creates no goroutine: [engine.run]
// starts the loops of the roles that run them.
func newEngine(st engineStore, opts engineOptions) (*engine, error) {
	if opts.logger == nil {
		opts.logger = slog.New(slog.DiscardHandler)
	}
	clk := clock.System{}
	ids := id.NewV7Generator(clk.Now)

	routing := exprflow.New(expr.New(expr.Options{Sandbox: opts.sandbox}))
	client, err := egress.New(egress.Config{Logger: opts.logger})
	if err != nil {
		return nil, fmt.Errorf("wire egress: %w", err)
	}
	credentials := opts.credentials
	if credentials == nil {
		credentials = refuseCredentials
	}
	policy := opts.senderPolicy
	if policy == nil {
		policy = publicHTTPSPolicy
	}
	httpNode, err := httpdispatch.New(httpdispatch.Config{
		Journal: st,
		Buckets: st,
		SenderFor: func(u *url.URL) declarative.Sender {
			return client.Sender(policy(u))
		},
		Credential: credentials,
		Bucket:     limit.Spec{Capacity: 60, RefillPerSecond: 10},
		IDs:        ids,
		Clock:      clk,
	})
	if err != nil {
		return nil, fmt.Errorf("wire orcon.http@1: %w", err)
	}
	models := opts.models
	if models == nil {
		models = modelResolver{client: client, credentials: credentials}
	}
	llmNode, err := chain.NewLLM(chain.Config{Models: models})
	if err != nil {
		return nil, fmt.Errorf("wire orcon.ai.llm@1: %w", err)
	}
	agentStore, ok := st.(agent.Store)
	if !ok {
		return nil, errors.New("wire orcon.ai.agent@1: the store keeps no agent runs")
	}
	// The agent runs its tools through the same runners as the flow: a tool is
	// a node type the agent may call, here the HTTP node.
	agentNode, err := agent.New(agent.Config{
		Models: models, Store: agentStore, Clock: clk,
		Runners: map[string]execution.NodeRunner{httpdispatch.HTTPNodeType: httpNode},
	})
	if err != nil {
		return nil, fmt.Errorf("wire orcon.ai.agent@1: %w", err)
	}
	runner := logFailures{next: httpdispatch.NewRouter(map[string]execution.NodeRunner{
		httpdispatch.HTTPNodeType: httpNode,
		chain.LLMNodeType:         llmNode,
		agent.NodeType:            agentNode,
	}), logger: opts.logger}

	workers := opts.role == config.RoleStandalone
	cfg := durable.Config{
		Worker: execution.WorkerID(ids.New()), Role: strings.ToUpper(string(opts.role)), Host: hostName(), Version: opts.version,
		Workers: workers, Reaper: workers, Dispatcher: workers,
		Clock: clk, IDs: ids, Logger: opts.logger,
	}
	deps := execution.Deps{Conditions: routing, Router: routing, Runner: runner}
	eng, err := durable.New(&cfg, st, st, flowReader{}, deps, durable.PlanLimits{})
	if err != nil {
		return nil, fmt.Errorf("wire the durable engine: %w", err)
	}

	verifier, err := newWhatsAppVerifier(st, credentials)
	if err != nil {
		return nil, fmt.Errorf("wire the WhatsApp trigger: %w", err)
	}
	starter := triggerStarter{engine: eng}
	svc, err := trigger.New(trigger.Config{
		Store: st, Flows: st, Catalog: triggerCatalog{}, Credentials: noTriggerCredentials{},
		Settings: trigger.Settings{}, Starter: starter, Clock: clk, IDs: ids, Verifier: verifier,
	})
	if err != nil {
		return nil, fmt.Errorf("wire the trigger service: %w", err)
	}
	return &engine{durable: eng, trigger: svc, logger: opts.logger, workers: workers}, nil
}

// logFailures logs every activation that fails, with the node it ran, before the
// engine records the failure: the event of the execution carries no reason.
type logFailures struct {
	next   execution.NodeRunner
	logger *slog.Logger
}

// Execute implements [execution.NodeRunner].
func (r logFailures) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	out, err := r.next.Execute(ctx, call)
	if err != nil {
		r.logger.WarnContext(ctx, "node failed", "node", call.Node.ID, "type", call.Node.Type,
			"execution", call.Execution, "error", err)
	}
	return out, err
}

// run runs the claim, reaper and dispatcher loops of the engine until ctx ends
// (standalone). A process that is not standalone only starts executions, so run
// returns at once for it.
func (e *engine) run(ctx context.Context) error {
	if !e.workers {
		return nil
	}
	return e.durable.Run(ctx)
}

// ingress is the public ingress of the engine: the trigger service.
func (e *engine) ingress() *trigger.Service { return e.trigger }

// semanticVersion matches the binary versions the database records (MAJOR.MINOR.PATCH,
// with an optional pre-release tag).
var semanticVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// binaryVersion is the version the database records for this build. A build
// without a release version (the default "dev") records 0.0.0.
func binaryVersion(v string) string {
	if semanticVersion.MatchString(v) {
		return v
	}
	return "0.0.0"
}

// openStore opens the database the configuration names, the way the server does.
//
//nolint:ireturn // the engine runs over either adapter through the one port
func openStore(ctx context.Context, cfg *config.Config, version string) (engineStore, func() error, error) {
	version = binaryVersion(version)
	if cfg.Value("store.driver").Text() == "postgres" {
		s, err := postgres.Open(ctx, postgres.Options{DSN: cfg.Secret("store.postgres.dsn").Reveal(), BinaryVersion: version})
		if err != nil {
			return nil, nil, fmt.Errorf("open postgres: %w", err)
		}
		return s, s.Close, nil
	}
	s, err := sqlite.Open(ctx, sqlite.Options{Path: cfg.Value("store.sqlite.path").Text(), BinaryVersion: version})
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	return s, s.Close, nil
}

// triggerStarter starts the executions the trigger service accepts. It is the
// adapter between the trigger area and the durable engine.
type triggerStarter struct {
	engine *durable.Engine
}

// StartManual implements [trigger.Starter]. Manual runs of the editor arrive with the
// editor cut; this build starts only accepted events.
func (triggerStarter) StartManual(context.Context, trigger.ManualRun, []byte) (string, error) {
	return "", trigger.ErrNoStarter
}

// StartTriggered implements [trigger.Starter]: the body of the event becomes the
// items of the execution, and the execution is pinned to the accepted version.
func (s triggerStarter) StartTriggered(ctx context.Context, run trigger.TriggeredRun, body []byte) (string, error) {
	items, err := bodyItems(body)
	if err != nil {
		return "", err
	}
	id, err := s.engine.Start(ctx, durable.StartRequest{
		OrgID: run.Org, ProjectID: run.ProjectID, FlowID: run.FlowID, FlowVersionID: run.Version,
		Mode: "TRIGGER", Start: run.Node, Input: execution.NewBatch(items...),
	})
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// bodyItems turns a JSON body into the items of an execution: an array holds one
// item per element; any other value is one item.
func bodyItems(body []byte) ([]execution.Item, error) {
	v, err := jsonvalue.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", trigger.ErrBadBody, err)
	}
	if v.Kind() != jsonvalue.KindArray {
		return []execution.Item{{JSON: v}}, nil
	}
	out := make([]execution.Item, 0, v.Len())
	for _, it := range v.Items() {
		out = append(out, execution.Item{JSON: it})
	}
	return out, nil
}

// noTriggerCredentials reports that no trigger needs a credential this build cannot use.
type noTriggerCredentials struct{}

// Missing implements [trigger.Credentials].
func (noTriggerCredentials) Missing(context.Context, identity.OrgID, identity.ProjectID, string, jsonvalue.Value) ([]string, error) {
	return nil, nil
}

// refuseCredentials is the credential port until the credential store arrives.
func refuseCredentials(context.Context, identity.OrgID, string) (httpdispatch.Credential, error) {
	return httpdispatch.Credential{}, ErrNoCredentialStore
}

// publicHTTPSPolicy grants one destination over https on its own host. Private
// and loopback addresses stay refused.
func publicHTTPSPolicy(u *url.URL) egress.Policy {
	return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"https"}}
}

// flowReader turns stored flow documents into the document of this build's
// format (flow.Read), as the durable engine asks for them.
type flowReader struct{}

// Read implements [durable.DocumentReader].
func (flowReader) Read(data []byte) (*flow.Document, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("read flow document: %w", err)
	}
	doc, err := flow.Read(v)
	if err != nil {
		return nil, fmt.Errorf("read flow document: %w", err)
	}
	return &doc, nil
}

// Version implements [durable.DocumentReader].
func (flowReader) Version() int { return flow.FormatMajor }

func hostName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
