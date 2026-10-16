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
	"strings"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/exprflow"
	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/chain"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/connector/declarative"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/credential"
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
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

// WebhookNodeType is the trigger node type of a public webhook address.
const WebhookNodeType = "orcon.webhook@1"

// webhookBodyCap is the size of one event the webhook trigger accepts.
const webhookBodyCap = 1 << 20

// ErrNoCredentialStore means a node names a credential and this process has no
// credential store to resolve it from.
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
	// resolver is the credential service of the process: every orcon.http@1
	// activation that names a credential resolves it through it, for the project of
	// the execution and the host of the request (ADR-0043).
	resolver credential.Resolver
	// credentials replaces the resolver in tests; nil uses the resolver, or the
	// refusing function when there is none either.
	credentials func(ctx context.Context, req httpdispatch.CredentialRequest) (httpdispatch.Credential, error)
	// models resolves the model of the AI nodes; nil uses the production
	// resolver, which reaches each provider through the egress client with the
	// API key of the stored credential the model names.
	models chain.Models
	// modelEndpoint points every model provider at a scripted server in tests;
	// nil in production. It does not change how the credential is resolved.
	modelEndpoint *modelEndpoint
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
	switch {
	case credentials != nil:
	case opts.resolver != nil:
		credentials = resolveCredentials(opts.resolver, st)
	default:
		credentials = refuseCredentials
	}
	inbound := refuseInbound
	if opts.resolver != nil {
		inbound = resolveInbound(opts.resolver, st)
	}
	policy := opts.senderPolicy
	if policy == nil {
		policy = publicHTTPSPolicy
	}
	httpNode, err := httpdispatch.New(httpdispatch.Config{
		Journal: st,
		Scopes:  st,
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
		models = modelResolver{client: client, credentials: credentials, endpoint: opts.modelEndpoint}
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
	budgets, ok := st.(cost.Store)
	if !ok {
		return nil, errors.New("wire orcon.ai.agent@1: the store keeps no AI budgets")
	}
	agentNode, err := agent.New(agent.Config{
		Models: models, Store: agentStore, Orgs: st, Clock: clk,
		Prices: newModelPrices(), Budgets: budgets,
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

	verifier, err := newWhatsAppVerifier(st, inbound)
	if err != nil {
		return nil, fmt.Errorf("wire the WhatsApp trigger: %w", err)
	}
	starter := triggerStarter{engine: eng}
	svc, err := trigger.New(trigger.Config{
		Store: st, Flows: st, Catalog: triggerCatalog{}, Credentials: connectorTriggerCredentials{resolver: opts.resolver},
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

// connectorTriggerCredentials is the [trigger.Credentials] of this build. The native triggers need no
// credential. The WhatsApp trigger verifies every inbound request with the stored credential its
// credential parameter names, so publication and activation check, for the project of the flow,
// that the parameter is set, that the credential exists and is usable there, and that it holds the
// appSecret field the signature check reads. A process without a credential store cannot verify
// anything, so it reports the credential as missing.
type connectorTriggerCredentials struct {
	resolver credential.Resolver
}

// Missing implements [trigger.Credentials].
func (c connectorTriggerCredentials) Missing(ctx context.Context, org orgscope.Org, project identity.ProjectID, nodeType string, parameters jsonvalue.Value) ([]string, error) {
	if nodeType != whatsappTriggerType {
		return nil, nil
	}
	param, ok := parameters.Get(credentialParameter)
	name, isText := param.Str()
	if !ok || !isText || name == "" {
		return []string{"WhatsApp credential (the node names none)"}, nil
	}
	if c.resolver == nil {
		return []string{name + " (this process has no credential store)"}, nil
	}
	cid, err := id.Parse(name)
	if err != nil {
		return []string{name + " (not a credential identifier)"}, nil //nolint:nilerr // a malformed identifier is a finding about the node, not a failure to check
	}
	res, err := c.resolver.Resolve(ctx, org, project, cid)
	switch {
	case errors.Is(err, credential.ErrNotFound), errors.Is(err, credential.ErrNotUsable):
		return []string{name + " (not found or not usable in this project)"}, nil
	case err != nil:
		return nil, fmt.Errorf("resolve the credential of the WhatsApp trigger: %w", err)
	}
	if secret, has := res.Secret("appSecret"); !has || secret == "" {
		return []string{res.Name + " (it has no appSecret field)"}, nil
	}
	return nil, nil
}

// refuseCredentials is the credential port until the credential store arrives.
func refuseCredentials(context.Context, httpdispatch.CredentialRequest) (httpdispatch.Credential, error) {
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
