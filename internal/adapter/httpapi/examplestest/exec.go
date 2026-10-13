// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Defaults of Config.
const (
	defaultAPIPrefix    = "/api/v1"
	defaultPollInterval = 250 * time.Millisecond
	maxSSELine          = 1 << 20
	eventBuffer         = 64
)

// terminalStatuses are the final states of an Operation (LRO-2).
var terminalStatuses = map[string]bool{"SUCCEEDED": true, "FAILED": true, "CANCELED": true}

// Config describes one run of the examples.
type Config struct {
	// ContractName is the contract file name shown in messages.
	ContractName string
	// Contract is the bundled contract as JSON.
	Contract []byte
	// BaseURL is the server under test, for example "http://127.0.0.1:8080".
	BaseURL string
	// APIPrefix is the server URL path of the API; default "/api/v1".
	APIPrefix string
	// Token is the non-interactive credential sent as a bearer token.
	Token string
	// DB names the database engine behind the server ("sqlite" or "postgres"); it is
	// reported in every message.
	DB string
	// Scenarios overrides the registered scenarios; nil uses the ones added with
	// RegisterScenario.
	Scenarios map[string]Scenario
	// OrgID is the id of the organization the credential belongs to; scenarios
	// read it through Client.OrgID.
	OrgID string
	// HTTPClient overrides the HTTP client.
	HTTPClient *http.Client
	// PollInterval is the wait between polls of a long-running operation.
	PollInterval time.Duration
	// OperationTimeout bounds the wait for a terminal operation; it is capped at 60 s.
	OperationTimeout time.Duration
}

// failure is why one example failed.
type failure struct {
	kind    Kind
	pointer string
	detail  string
}

func fail(kind Kind, format string, args ...any) *failure {
	return &failure{kind: kind, detail: fmt.Sprintf(format, args...)}
}

type runner struct {
	cfg       Config
	contract  *contract
	client    *Client
	scenarios map[string]Scenario
	byName    map[string][]exampleDef
}

func newRunner(cfg Config) (*runner, error) {
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = defaultAPIPrefix
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.OperationTimeout <= 0 || cfg.OperationTimeout > maxOperationWait {
		cfg.OperationTimeout = maxOperationWait
	}
	if cfg.ContractName == "" {
		cfg.ContractName = "contract"
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("config: BaseURL is required")
	}
	c, err := loadContract(cfg.ContractName, cfg.Contract)
	if err != nil {
		return nil, err
	}
	scenarios := cfg.Scenarios
	if scenarios == nil {
		scenarios = registered()
	}
	return &runner{
		cfg:       cfg,
		contract:  c,
		client:    NewClient(cfg.BaseURL, cfg.APIPrefix, cfg.Token, cfg.HTTPClient).WithOrg(cfg.OrgID),
		scenarios: scenarios,
	}, nil
}

// runExample runs the scenario and the case; nil means the example passed.
func (r *runner) runExample(ctx context.Context, def exampleDef) *failure {
	bindings := Bindings{}
	if def.Scenario != "" {
		sc, ok := r.scenarios[def.Scenario]
		if !ok {
			return fail(KindScenario, "scenario %q is not registered", def.Scenario)
		}
		b, err := sc(ctx, r.client)
		if err != nil {
			return fail(KindScenario, "scenario %q failed: %v", def.Scenario, err)
		}
		bindings = b
		if bindings == nil {
			bindings = Bindings{}
		}
	}
	if def.Stream != nil {
		return r.runStream(ctx, def, bindings)
	}
	return r.perform(ctx, def, bindings, true)
}

// request builds the call of an example with the bindings applied.
func (r *runner) request(def exampleDef, bindings Bindings) (Request, *failure) {
	path := def.Path
	for _, name := range sortedKeys(def.PathParams) {
		raw := fmt.Sprint(def.PathParams[name])
		if _, bound := bindings[raw]; !bound && isUUID(raw) {
			return Request{}, fail(KindUnbound, "path parameter %s carries the id %s and no scenario binding resolves it", name, raw)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(bindings.bind(raw)))
	}
	if strings.Contains(path, "{") {
		return Request{}, fail(KindContract, "path %s still has unfilled parameters after request.pathParams", path)
	}
	req := Request{Method: def.Method, Path: path}
	if len(def.Query) > 0 {
		req.Query = url.Values{}
		for _, k := range sortedKeys(def.Query) {
			req.Query.Set(k, bindings.bind(fmt.Sprint(def.Query[k])))
		}
	}
	if len(def.Headers) > 0 {
		req.Header = http.Header{}
		for _, k := range sortedKeys(def.Headers) {
			req.Header.Set(k, bindings.bind(fmt.Sprint(def.Headers[k])))
		}
	}
	if def.RequestBody != "" {
		body, _, err := r.contract.requestExample(def)
		if err != nil {
			return Request{}, fail(KindContract, "%v", err)
		}
		req.Body = bindings.bindValue(body)
	}
	return req, nil
}

// perform sends the case and checks status, body and, for long-running
// operations, the terminal state. Stream triggers pass checkOperation false.
func (r *runner) perform(ctx context.Context, def exampleDef, bindings Bindings, checkOperation bool) *failure {
	req, f := r.request(def, bindings)
	if f != nil {
		return f
	}
	resp, err := r.client.Do(ctx, req)
	if err != nil {
		return fail(KindRequest, "%v", err)
	}
	if resp.Status != def.Status {
		return fail(KindStatus, "want status %d, got %d (%s)", def.Status, resp.Status, preview(resp.Body))
	}
	if f := r.checkBody(def, bindings, resp); f != nil {
		return f
	}
	if checkOperation && def.Operation != nil {
		return r.awaitOperation(ctx, def, resp)
	}
	return nil
}

// checkBody validates the body against the response schema and compares the example.
func (r *runner) checkBody(def exampleDef, bindings Bindings, resp *Response) *failure {
	c := r.contract
	respObj, respPtr, ok, err := c.response(def)
	if err != nil {
		return fail(KindContract, "%v", err)
	}
	if !ok {
		return fail(KindContract, "operation declares no response for status %d", def.Status)
	}
	schemaPtr, err := c.responseSchema(respObj, respPtr)
	if err != nil {
		return fail(KindContract, "%v", err)
	}
	if len(resp.Body) == 0 {
		if def.ResponseRef != "" {
			return fail(KindValue, "the example %q expects a body and the response is empty", def.ResponseRef)
		}
		return nil
	}
	if schemaPtr == "" && def.ResponseRef == "" {
		return nil
	}
	got, err := jsonschema.UnmarshalJSON(strings.NewReader(string(resp.Body)))
	if err != nil {
		return fail(KindSchema, "the response body is not JSON: %v", err)
	}
	var schemaNode map[string]any
	if schemaPtr != "" {
		if n, found := c.lookup(schemaPtr); found {
			schemaNode, _ = n.(map[string]any)
		}
		if c.isEnvelope(got, schemaNode, schemaPtr) {
			return fail(KindEnvelope, "the response is wrapped in an envelope; resources are returned bare")
		}
		instance, reason, err := c.validate(schemaPtr, got)
		if err != nil {
			return fail(KindContract, "%v", err)
		}
		if reason != "" {
			return &failure{kind: KindSchema, pointer: instance, detail: "the body does not validate against the response schema: " + reason}
		}
	}
	if def.ResponseRef == "" {
		return nil
	}
	want, _, err := c.namedExample(respObj, respPtr, def.ResponseRef)
	if err != nil {
		return fail(KindContract, "%v", err)
	}
	if d := c.compare(bindings.bindValue(want), got, schemaNode, schemaPtr, ""); d != nil {
		return &failure{kind: KindValue, pointer: d.pointer, detail: d.reason}
	}
	return nil
}

func preview(body []byte) string {
	const limit = 120
	s := strings.TrimSpace(string(body))
	if len(s) > limit {
		s = s[:limit] + "..."
	}
	return s
}

// awaitOperation polls the Location of a 202 answer until the Operation is terminal.
func (r *runner) awaitOperation(ctx context.Context, def exampleDef, first *Response) *failure {
	location := first.Header.Get("Location")
	if location == "" {
		return fail(KindOperation, "the %d answer has no Location header to poll", first.Status)
	}
	ctx, cancel := context.WithTimeout(ctx, r.cfg.OperationTimeout)
	defer cancel()
	last := ""
	for {
		resp, err := r.client.send(ctx, http.MethodGet, location, nil, nil, nil)
		if err != nil {
			if ctx.Err() != nil {
				return fail(KindOperation, "operation at %s was not terminal within %s (last status %q)", location, r.cfg.OperationTimeout, last)
			}
			return fail(KindOperation, "poll %s: %v", location, err)
		}
		if resp.Status != http.StatusOK {
			return fail(KindOperation, "poll %s answered %d", location, resp.Status)
		}
		var op struct {
			Status string `json:"status"`
		}
		if err := resp.Decode(&op); err != nil {
			return fail(KindOperation, "%v", err)
		}
		last = op.Status
		if terminalStatuses[op.Status] {
			if op.Status != def.Operation.TerminalStatus {
				return fail(KindOperation, "operation ended %s, the example expects %s", op.Status, def.Operation.TerminalStatus)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return fail(KindOperation, "operation at %s was not terminal within %s (last status %q)", location, r.cfg.OperationTimeout, last)
		case <-time.After(r.cfg.PollInterval):
		}
	}
}

// runStream opens the event stream of def, runs the trigger example and waits
// for the expected event types in order.
func (r *runner) runStream(ctx context.Context, def exampleDef, bindings Bindings) *failure {
	trigger, f := r.findTrigger(def)
	if f != nil {
		return f
	}
	req, f := r.request(def, bindings)
	if f != nil {
		return f
	}
	ctx, cancel := context.WithTimeout(ctx, def.Stream.Within)
	defer cancel()

	events := make(chan string, eventBuffer)
	connected := make(chan *failure, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.readStream(ctx, def, req, events, connected)
	}()
	// The reader exits when ctx is canceled; waiting here keeps it owned.
	defer wg.Wait()
	defer cancel()

	select {
	case f := <-connected:
		if f != nil {
			return f
		}
	case <-ctx.Done():
		return fail(KindStream, "the stream did not open within %s", def.Stream.Within)
	}
	// The trigger runs with the stream example's bindings and without a scenario of its own.
	if f := r.perform(ctx, trigger, bindings, false); f != nil {
		return fail(KindStream, "trigger example %q failed: %s: %s", trigger.Name, f.kind, f.detail)
	}
	var seen []string
	next := 0
	for next < len(def.Stream.ExpectTypes) {
		select {
		case typ := <-events:
			seen = append(seen, typ)
			if typ == def.Stream.ExpectTypes[next] {
				next++
			}
		case <-ctx.Done():
			return fail(KindStream, "the stream delivered %v, expected %v in order within %s", seen, def.Stream.ExpectTypes, def.Stream.Within)
		}
	}
	return nil
}

func (r *runner) findTrigger(def exampleDef) (exampleDef, *failure) {
	candidates := r.byName[def.Stream.Trigger]
	switch len(candidates) {
	case 0:
		return exampleDef{}, fail(KindContract, "stream.trigger %q names no example", def.Stream.Trigger)
	case 1:
		return candidates[0], nil
	default:
		return exampleDef{}, fail(KindContract, "stream.trigger %q is ambiguous: %d examples carry that name", def.Stream.Trigger, len(candidates))
	}
}

// readStream opens the stream, reports the outcome once on connected and then
// sends the type of each event until ctx ends or the stream closes.
func (r *runner) readStream(ctx context.Context, def exampleDef, req Request, events chan<- string, connected chan<- *failure) {
	hr, err := r.client.newRequest(ctx, req.Method, r.client.prefix+req.Path, req.Query, req.Header, nil)
	if err != nil {
		connected <- fail(KindRequest, "%v", err)
		return
	}
	hr.Header.Set("Accept", "text/event-stream")
	resp, err := r.client.http.Do(hr)
	if err != nil {
		connected <- fail(KindRequest, "open stream: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != def.Status {
		connected <- fail(KindStatus, "want status %d, got %d opening the stream", def.Status, resp.StatusCode)
		return
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		connected <- fail(KindStream, "the stream answered Content-Type %q, want text/event-stream", ct)
		return
	}
	connected <- nil

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELine)
	typ, data := "", ""
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if typ == "" {
				typ = typeOfData(data)
			}
			if typ != "" {
				select {
				case events <- typ:
				case <-ctx.Done():
					return
				}
			}
			typ, data = "", ""
		case strings.HasPrefix(line, "event:"):
			typ = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

// typeOfData reads the type member of an event whose frame has no event: line.
func typeOfData(data string) string {
	var payload struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return ""
	}
	return payload.Type
}
