// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Call is one invocation of an operation. Parameters are the values of the
// operation's parameter schema; Credentials are the decrypted fields of the
// credential the call runs under. Credentials are never returned, logged or
// mapped to items.
type Call struct {
	Operation   string
	Parameters  map[string]jsonvalue.Value
	Credentials map[string]string
}

// Options are the ports and clocks of an [Interpreter].
type Options struct {
	Sender Sender
	// Now returns the current time, used for request signatures. Default:
	// time.Now.
	Now func() time.Time
	// Sleep waits before a retry. It returns early when ctx ends. Default: a
	// timer that honors the Retry-After of the provider when given.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Sink receives the items of each page as soon as they are mapped. A sink keeps
// the memory of a call independent of its number of pages (ADR-0030 §6).
type Sink func(items []jsonvalue.Value) error

// Result is what a successful call produced.
type Result struct {
	// Items are collected only when Execute is given no sink.
	Items []jsonvalue.Value
	// Pages is the number of pages requested.
	Pages int
	// Warnings are non-secret notices, such as a truncated page walk.
	Warnings []string
}

// Interpreter runs the operations of one validated definition.
type Interpreter struct {
	def  Definition
	opts Options
	ops  map[string]Operation
}

// WithSender returns an interpreter for the same definition that sends through
// s. It is how the conformance kit replays a recording.
func (in *Interpreter) WithSender(s Sender) (*Interpreter, error) {
	opts := in.opts
	opts.Sender = s
	return New(in.def, opts)
}

// New validates def and returns an interpreter for it.
func New(def Definition, opts Options) (*Interpreter, error) {
	if err := Validate(def); err != nil {
		return nil, err
	}
	if opts.Sender == nil {
		return nil, errors.New("declarative: a sender is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepCtx
	}
	return &Interpreter{def: def, opts: opts, ops: def.Operations}, nil
}

// Execute runs one operation. With a nil sink the items are returned in the
// Result; with a sink they are passed on page by page. A failure is returned
// as a *[CallError].
func (in *Interpreter) Execute(ctx context.Context, call Call, sink Sink) (Result, error) {
	op, ok := in.ops[call.Operation]
	if !ok {
		return Result{}, &CallError{Class: ClassPermanent, Code: "ValidationFailed", Message: "the operation does not exist"}
	}
	st := &walk{in: in, op: op, call: call, sink: sink}
	if err := st.run(ctx); err != nil {
		return st.result, err
	}
	return st.result, nil
}

// walk holds the state of one Execute call across its pages.
type walk struct {
	in     *Interpreter
	op     Operation
	call   Call
	sink   Sink
	result Result
	// offset, page and cursor are the position of the next page.
	offset int
	page   int
	cursor string
	next   string
}

func (w *walk) run(ctx context.Context) error {
	pg := w.op.Pagination
	if pg == nil {
		_, err := w.fetchPage(ctx, nil)
		return err
	}
	maxPages := pg.MaxPages
	if maxPages == 0 {
		maxPages = DefaultMaxPages
	}
	w.offset = pg.Start
	w.page = pg.Start
	if pg.Strategy == "PAGE" && w.page == 0 {
		w.page = 1
	}
	for {
		if w.result.Pages >= maxPages {
			if pg.OnMaxPages == "TRUNCATE" {
				w.result.Warnings = append(w.result.Warnings, "the page walk stopped at maxPages")
				return nil
			}
			return &CallError{Class: ClassPermanent, Message: "the page walk reached maxPages"}
		}
		more, err := w.fetchPage(ctx, pg)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
}

// fetchPage requests one page (or the only page when pg is nil), maps its items
// and reports whether another page follows.
func (w *walk) fetchPage(ctx context.Context, pg *Pagination) (bool, error) {
	body, resp, err := w.in.send(ctx, w.op, w.call, pg, w)
	if err != nil {
		return false, err
	}
	w.result.Pages++
	items, err := w.mapItems(body)
	if err != nil {
		return false, err
	}
	if err := w.deliver(items); err != nil {
		return false, err
	}
	if pg == nil {
		return false, nil
	}
	return w.advance(pg, body, resp, len(items))
}

// advance computes the position of the next page and reports whether there is
// one. The stop rule follows the strategy: a short or empty page ends OFFSET and
// PAGE, a missing cursor or link ends the other three.
func (w *walk) advance(pg *Pagination, body jsonvalue.Value, resp HTTPResponse, n int) (bool, error) {
	switch pg.Strategy {
	case "OFFSET":
		size := pg.PageSize
		if size <= 0 {
			size = n
		}
		if n == 0 || n < size {
			return false, nil
		}
		w.offset += n
		return true, nil
	case "PAGE":
		if n == 0 {
			return false, nil
		}
		w.page++
		return true, nil
	case "CURSOR":
		c, err := w.readCursor(pg, body, resp)
		if err != nil || c == "" {
			return false, err
		}
		w.cursor = c
		return true, nil
	case "LINK_HEADER":
		link := nextLink(resp.Header["link"])
		if link == "" {
			return false, nil
		}
		w.next = link
		return true, nil
	case "NEXT_URL":
		v, ok, err := lookup(body, pg.Path)
		if err != nil {
			return false, &CallError{Class: ClassPermanent, Code: "ProviderError", Message: "the next URL path is invalid"}
		}
		s, isStr := v.Str()
		if !ok || !isStr || s == "" {
			return false, nil
		}
		w.next = s
		return true, nil
	}
	return false, nil
}

func (w *walk) readCursor(pg *Pagination, body jsonvalue.Value, resp HTTPResponse) (string, error) {
	if pg.From == "HEADER" {
		if vals := resp.Header[strings.ToLower(pg.Header)]; len(vals) > 0 {
			return vals[0], nil
		}
		return "", nil
	}
	v, ok, err := lookup(body, pg.Path)
	if err != nil {
		return "", &CallError{Class: ClassPermanent, Code: "ProviderError", Message: "the cursor path is invalid"}
	}
	if !ok {
		return "", nil
	}
	if s, isStr := v.Str(); isStr {
		return s, nil
	}
	if n, isNum := v.NumberText(); isNum {
		return n, nil
	}
	return "", nil
}

// mapItems applies the response selection of the operation to a body.
func (w *walk) mapItems(body jsonvalue.Value) ([]jsonvalue.Value, error) {
	sel := body
	if p := w.op.Response.Select; p != "" && p != "$" {
		v, ok, err := lookup(body, p)
		if err != nil {
			return nil, &CallError{Class: ClassPermanent, Code: "ProviderError", Message: "the response path is invalid"}
		}
		if !ok {
			return w.emptyItems(), nil
		}
		sel = v
	}
	var items []jsonvalue.Value
	if sel.Kind() == jsonvalue.KindArray {
		items = sel.Items()
		if len(items) == 0 {
			return w.emptyItems(), nil
		}
	} else {
		items = []jsonvalue.Value{sel}
	}
	if len(w.op.Response.Map) == 0 {
		return items, nil
	}
	out := make([]jsonvalue.Value, 0, len(items))
	for _, item := range items {
		enriched, err := enrich(item, w.op.Response.Map)
		if err != nil {
			return nil, &CallError{Class: ClassPermanent, Code: "ProviderError", Message: "the response map path is invalid"}
		}
		out = append(out, enriched)
	}
	return out, nil
}

// enrich adds the mapped keys to an item and keeps the provider's record as it
// is (ADR-0030 section 5; NM-10): a key that the record already has is replaced
// by the mapped value, and a path that finds nothing adds nothing. An item that
// is not an object is returned unchanged.
func enrich(item jsonvalue.Value, mapping map[string]string) (jsonvalue.Value, error) {
	if item.Kind() != jsonvalue.KindObject {
		return item, nil
	}
	out := item
	for _, k := range slices.Sorted(maps.Keys(mapping)) {
		v, ok, err := firstFound(item, mapping[k])
		if err != nil {
			return jsonvalue.Value{}, err
		}
		if ok {
			out = out.With(k, v)
		}
	}
	return out, nil
}

// project builds the output object of one event from a decode map: each key
// takes the value at its path, and a path that finds nothing is left out.
// Unlike [enrich], the record is not kept.
func project(item jsonvalue.Value, mapping map[string]string) (jsonvalue.Value, error) {
	keys := slices.Sorted(maps.Keys(mapping))
	members := make([]jsonvalue.Member, 0, len(keys))
	for _, k := range keys {
		v, ok, err := firstFound(item, mapping[k])
		if err != nil {
			return jsonvalue.Value{}, err
		}
		if ok {
			members = append(members, jsonvalue.Member{Key: k, Value: v})
		}
	}
	return jsonvalue.Object(members...)
}

// firstFound returns the value of the first alternative of expr that finds one.
func firstFound(item jsonvalue.Value, expr string) (jsonvalue.Value, bool, error) {
	for _, alt := range alternatives(expr) {
		v, ok, err := lookup(item, alt)
		if err != nil || ok {
			return v, ok, err
		}
	}
	return jsonvalue.Value{}, false, nil
}

func (w *walk) emptyItems() []jsonvalue.Value {
	if w.op.Response.Empty == "ONE_EMPTY_ITEM" {
		o, _ := jsonvalue.Object()
		return []jsonvalue.Value{o}
	}
	return nil
}

func (w *walk) deliver(items []jsonvalue.Value) error {
	if w.sink != nil {
		if len(items) == 0 {
			return nil
		}
		return w.sink(items)
	}
	w.result.Items = append(w.result.Items, items...)
	return nil
}

// send performs one operation request with the retries its safety allows, and
// returns the decoded body. It never retries a write without a key.
func (in *Interpreter) send(ctx context.Context, op Operation, call Call, pg *Pagination, w *walk) (jsonvalue.Value, HTTPResponse, error) {
	limit := op.MaxAttempts
	if limit == 0 {
		limit = DefaultMaxAttempts
	}
	retryable := op.Safety == SafetyReadOnly || op.Safety == SafetyIdempotencyKey
	for attempt := 1; ; attempt++ {
		body, resp, fail := in.attempt(ctx, op, call, pg, w)
		if fail == nil {
			return body, resp, nil
		}
		if !retryable || fail.Class != ClassTransient || attempt >= limit {
			return jsonvalue.Value{}, resp, fail
		}
		wait := fail.RetryAfter
		if wait == 0 {
			wait = time.Duration(attempt) * 200 * time.Millisecond
		}
		if err := in.opts.Sleep(ctx, wait); err != nil {
			return jsonvalue.Value{}, resp, &CallError{Class: ClassTransient, Code: "Timeout", Message: "the call ended while waiting to retry"}
		}
	}
}

// attempt builds and sends one request and classifies its outcome.
func (in *Interpreter) attempt(ctx context.Context, op Operation, call Call, pg *Pagination, w *walk) (jsonvalue.Value, HTTPResponse, *CallError) {
	req, err := in.buildRequest(op, call, pg, w)
	if err != nil {
		return jsonvalue.Value{}, HTTPResponse{}, requestFailure(err)
	}
	resp, err := in.opts.Sender.Send(ctx, req)
	if err != nil {
		return jsonvalue.Value{}, HTTPResponse{}, classifyTransport(op, err)
	}
	if f := classifyResponse(op, resp); f != nil {
		return jsonvalue.Value{}, resp, f
	}
	body, err := jsonvalue.Parse(resp.Body)
	if err != nil && len(resp.Body) > 0 {
		return jsonvalue.Value{}, resp, &CallError{Class: ClassPermanent, Code: "ProviderError", Status: resp.Status, Message: "the response is not JSON"}
	}
	if len(resp.Body) == 0 {
		body = jsonvalue.Null()
	}
	return body, resp, nil
}

// requestFailure maps an error raised while building a request. None of the
// messages contains a credential value or a rendered parameter.
func requestFailure(err error) *CallError {
	switch {
	case errors.Is(err, ErrMissingField):
		return &CallError{Class: ClassPermanent, Code: "AuthInvalid", Message: "a credential field is missing"}
	case errors.Is(err, ErrUnsupported):
		return &CallError{Class: ClassPermanent, Code: "Unsupported", Message: "the connector uses a feature this interpreter does not run"}
	default:
		return &CallError{Class: ClassPermanent, Code: "ValidationFailed", Message: "a parameter or template cannot be used"}
	}
}

// buildRequest renders the operation under the call's parameters and the
// pagination position, then applies the connector's authentication.
func (in *Interpreter) buildRequest(op Operation, call Call, pg *Pagination, w *walk) (HTTPRequest, error) {
	params := scope{params: call.Parameters}
	req := HTTPRequest{Method: op.Request.Method, Header: map[string][]string{}}

	if w != nil && w.next != "" {
		// LINK_HEADER and NEXT_URL: the provider named the next page. A
		// relative reference resolves against the base URL (RFC 3986). The
		// network policy still checks the result on send.
		next, err := url.Parse(w.next)
		if err != nil {
			return req, fmt.Errorf("%w: next page", ErrTemplate)
		}
		base, err := url.Parse(in.def.HTTP.BaseURL)
		if err != nil {
			return req, fmt.Errorf("%w: base URL", ErrTemplate)
		}
		req.URL = base.ResolveReference(next).String()
	} else {
		path, err := params.render(op.Request.Path)
		if err != nil {
			return req, err
		}
		u, err := url.Parse(strings.TrimRight(in.def.HTTP.BaseURL, "/") + path)
		if err != nil {
			return req, fmt.Errorf("%w: URL", ErrTemplate)
		}
		q := u.Query()
		for k, tpl := range op.Request.Query {
			v, err := params.render(tpl)
			if err != nil {
				return req, err
			}
			q.Set(k, v)
		}
		if pg != nil && w != nil {
			addPagePosition(q, pg, w)
		}
		u.RawQuery = q.Encode()
		req.URL = u.String()
	}

	if err := in.addHeaders(op, params, &req); err != nil {
		return req, err
	}
	if err := in.addBody(op, params, &req); err != nil {
		return req, err
	}
	if op.Safety == SafetyIdempotencyKey {
		key, err := params.render(op.IdempotencyKey)
		if err != nil {
			return req, err
		}
		setHeader(&req, op.IdempotencyHeader, key)
	}

	credScope := scope{params: call.Parameters, credentials: call.Credentials}
	for name, tpl := range in.def.HTTP.Auth.Headers {
		v, err := credScope.render(tpl)
		if err != nil {
			return req, err
		}
		setHeader(&req, name, v)
	}
	if err := applyAuth(in.def.HTTP.Auth, call.Credentials, &req, in.opts.Now()); err != nil {
		return req, err
	}
	return req, nil
}

func addPagePosition(q url.Values, pg *Pagination, w *walk) {
	switch pg.Strategy {
	case "OFFSET":
		q.Set(pg.Param, strconv.Itoa(w.offset))
		if pg.LimitParam != "" && pg.PageSize > 0 {
			q.Set(pg.LimitParam, strconv.Itoa(pg.PageSize))
		}
	case "PAGE":
		q.Set(pg.Param, strconv.Itoa(w.page))
		if pg.LimitParam != "" && pg.PageSize > 0 {
			q.Set(pg.LimitParam, strconv.Itoa(pg.PageSize))
		}
	case "CURSOR":
		if w.cursor != "" {
			q.Set(pg.Param, w.cursor)
		}
	}
}

func (in *Interpreter) addHeaders(op Operation, params scope, req *HTTPRequest) error {
	for k, tpl := range in.def.HTTP.Headers {
		v, err := params.render(tpl)
		if err != nil {
			return err
		}
		setHeader(req, k, v)
	}
	for k, tpl := range op.Request.Headers {
		v, err := params.render(tpl)
		if err != nil {
			return err
		}
		setHeader(req, k, v)
	}
	return nil
}

func (in *Interpreter) addBody(op Operation, params scope, req *HTTPRequest) error {
	b := op.Request.Body
	if b == nil {
		return nil
	}
	if b.Type == "RAW" {
		text, err := params.render(b.Raw)
		if err != nil {
			return err
		}
		req.Body = []byte(text)
		return nil
	}
	if b.Type == "FORM" {
		form := url.Values{}
		for k, n := range b.Fields {
			tpl, ok := n.Value.Str()
			if !ok {
				return fmt.Errorf("%w: a form field must be a string", ErrTemplate)
			}
			v, err := params.render(tpl)
			if err != nil {
				return err
			}
			form.Set(k, v)
		}
		req.Body = []byte(form.Encode())
		setHeader(req, "Content-Type", "application/x-www-form-urlencoded")
		return nil
	}
	members := make([]jsonvalue.Member, 0, len(b.Fields))
	for _, k := range slices.Sorted(maps.Keys(b.Fields)) {
		v, err := params.renderNode(b.Fields[k].Value)
		if err != nil {
			return err
		}
		members = append(members, jsonvalue.Member{Key: k, Value: v})
	}
	obj, err := jsonvalue.Object(members...)
	if err != nil {
		return fmt.Errorf("%w: body", ErrTemplate)
	}
	raw, err := jsonvalue.Marshal(obj)
	if err != nil {
		return fmt.Errorf("%w: body", ErrTemplate)
	}
	req.Body = raw
	setHeader(req, "Content-Type", "application/json")
	return nil
}

// nextLink returns the target of the Link header relation "next" (RFC 8288).
func nextLink(values []string) string {
	for _, h := range values {
		for _, part := range strings.Split(h, ",") {
			seg := strings.Split(strings.TrimSpace(part), ";")
			if len(seg) < 2 {
				continue
			}
			target := strings.TrimSpace(seg[0])
			if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
				continue
			}
			for _, param := range seg[1:] {
				p := strings.TrimSpace(param)
				if strings.EqualFold(p, `rel="next"`) || strings.EqualFold(p, "rel=next") {
					return target[1 : len(target)-1]
				}
			}
		}
	}
	return ""
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
