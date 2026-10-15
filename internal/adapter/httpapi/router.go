// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/adapter/httpapi/reference"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
)

// ErrNoIdempotencyStore means the contract has idempotent operations and no store was given.
var ErrNoIdempotencyStore = errors.New("the contract has operations that take an Idempotency-Key and Deps.Idempotency is nil")

// ErrRole means the role does not serve the API.
var ErrRole = errors.New("this role does not serve the API")

// Deps are the collaborators of the API handler. The composition root builds
// them; nothing here reads configuration or the environment.
type Deps struct {
	// Role is the process role; only api and standalone serve the API.
	Role config.Role
	// Logger receives the structured log of the pipeline.
	Logger *slog.Logger
	// Clock drives the rate limits.
	Clock clock.Clock
	// IDs generates request ids.
	IDs id.Generator
	// Authenticator resolves credentials; nil means identity.RejectAll.
	Authenticator identity.Authenticator
	// Authorizer decides permissions; nil means authz.DenyAll.
	Authorizer authz.Authorizer
	// Catalog, when set, makes New fail if the contract names a permission
	// that is not in it.
	Catalog *authz.Catalog
	// Server implements the operations of the contract; nil answers 503
	// service-unavailable to all of them (Unavailable).
	Server apigen.StrictServerInterface
	// Limits are the rate limits; the zero value means middleware.DefaultLimits.
	Limits middleware.Limits
	// LimiterOptions adjust the rate limiter, for example the divisor.
	LimiterOptions []middleware.LimiterOption
	// Auth names where requests carry credentials; the zero value means
	// middleware.DefaultAuthConfig.
	Auth middleware.AuthConfig
	// Idempotency keeps the idempotency records. It is required when the contract has
	// an operation that takes an Idempotency-Key: there is no silent default, because
	// a store in the memory of the process survives neither a restart nor a second api
	// process. The composition root wires the durable store, or
	// idempotency.UnavailableStore while there is none.
	Idempotency idempotency.Store
	// Reference is the file system of the reference site; nil means the
	// embedded one.
	Reference fs.FS
}

// RouteKind says what a registered route is.
type RouteKind string

// The kinds of route a [Router] registers.
const (
	// KindOperation is the route of an operation of the contract.
	KindOperation RouteKind = "operation"
	// KindReference is part of the reference site at /api/reference.
	KindReference RouteKind = "reference"
	// KindFallback answers every other path under /api/ with a problem document.
	KindFallback RouteKind = "fallback"
)

// Route is one pattern registered on the router.
type Route struct {
	// Pattern is the mux pattern, for example "GET /api/v1/flows/{flowId}".
	Pattern string
	// Kind says what the route is.
	Kind RouteKind
}

// Router serves everything under /api/: the operations of the contract through
// the request pipeline, and the reference site.
type Router struct {
	handler http.Handler
	routes  []Route
	spec    *Spec
}

// ServeHTTP implements [http.Handler].
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) { rt.handler.ServeHTTP(w, r) }

// Routes returns every pattern registered, in registration order.
func (rt *Router) Routes() []Route { return append([]Route(nil), rt.routes...) }

// Spec returns the contract the router was built from.
func (rt *Router) Spec() *Spec { return rt.spec }

// New builds the router of the contract embedded in the binary.
func New(deps *Deps) (*Router, error) {
	raw, err := apigen.GetSpecJSON()
	if err != nil {
		return nil, fmt.Errorf("read the embedded contract: %w", err)
	}
	return build(deps, raw, func(mux apigen.ServeMux, baseURL string) {
		server := deps.Server
		if server == nil {
			server = Unavailable{}
		}
		handler := apigen.NewStrictHandlerWithOptions(server, nil, apigen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  errorHandler{logger: deps.Logger}.request,
			ResponseErrorHandlerFunc: errorHandler{logger: deps.Logger}.response,
		})
		apigen.HandlerWithOptions(handler, apigen.StdHTTPServerOptions{
			BaseURL:          baseURL,
			BaseRouter:       mux,
			ErrorHandlerFunc: errorHandler{logger: deps.Logger}.request,
		})
	})
}

// build assembles the router. The register function adds the operation routes
// to mux: New passes the generated registration and tests pass their own, so
// the pipeline can be exercised on synthetic contracts.
func build(supplied *Deps, rawSpec []byte, register func(mux apigen.ServeMux, baseURL string)) (*Router, error) {
	if supplied.Role != config.RoleAPI && supplied.Role != config.RoleStandalone {
		return nil, fmt.Errorf("%w: %q", ErrRole, supplied.Role)
	}
	deps := supplied.withDefaults()
	spec, err := ParseSpec(rawSpec)
	if err != nil {
		return nil, err
	}
	if deps.Idempotency == nil {
		for i := range spec.Operations {
			if spec.Operations[i].IdempotencyKey != "" {
				return nil, fmt.Errorf("%w (%s)", ErrNoIdempotencyStore, spec.Operations[i].ID)
			}
		}
	}
	if err := checkPermissions(spec, deps.Catalog); err != nil {
		return nil, err
	}
	site, err := referenceFS(deps)
	if err != nil {
		return nil, err
	}

	validators, err := newBodyValidators(spec, rawSpec)
	if err != nil {
		return nil, err
	}
	limiter := middleware.NewLimiter(deps.Clock, deps.Limits, deps.LimiterOptions...)
	rec := &recorder{mux: http.NewServeMux(), ops: map[string]Operation{}}
	for i := range spec.Operations {
		rec.ops[spec.Operations[i].Pattern] = spec.Operations[i]
	}
	rec.wrap = func(op *Operation, h http.Handler) http.Handler {
		return middleware.Chain(h, operationChain(deps, limiter, op, bodyMiddleware(validators, op))...)
	}
	register(rec, spec.BasePath)
	if len(rec.unknown) > 0 {
		sort.Strings(rec.unknown)
		return nil, fmt.Errorf("%w: the generated routes %v are not operations of the contract", ErrSpec, rec.unknown)
	}

	if err := checkParity(spec, rec.routes); err != nil {
		return nil, err
	}

	routes := rec.routes
	mux := rec.mux
	routes = append(routes, mountReference(mux, reference.Handler(site))...)
	mux.Handle("/api/", fallback(mux))
	routes = append(routes, Route{Pattern: "/api/", Kind: KindFallback})

	handler := middleware.Chain(mux,
		middleware.RequestID(deps.IDs),
		middleware.Recover(deps.Logger),
		middleware.LimitAddress(limiter),
	)
	return &Router{handler: handler, routes: routes, spec: spec}, nil
}

// withDefaults returns a copy of d with the secure defaults filled in.
func (d *Deps) withDefaults() *Deps {
	c := *d
	d = &c
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	if d.Clock == nil {
		d.Clock = clock.System{}
	}
	if d.IDs == nil {
		d.IDs = id.NewV7Generator(d.Clock.Now)
	}
	if d.Authenticator == nil {
		d.Authenticator = identity.RejectAll{}
	}
	if d.Authorizer == nil {
		d.Authorizer = authz.DenyAll{}
	}
	if d.Limits == (middleware.Limits{}) {
		d.Limits = middleware.DefaultLimits()
	}
	if d.Auth == (middleware.AuthConfig{}) {
		d.Auth = middleware.DefaultAuthConfig()
	}
	return d
}

func referenceFS(deps *Deps) (fs.FS, error) {
	if deps.Reference != nil {
		return deps.Reference, nil
	}
	site, err := reference.Site()
	if err != nil {
		return nil, fmt.Errorf("open the reference site: %w", err)
	}
	return site, nil
}

// checkParity fails when an operation of the contract has no route (R-API-01).
// The converse, a route that is no operation, is caught while registering.
func checkParity(spec *Spec, routes []Route) error {
	registered := make(map[string]bool, len(routes))
	for _, r := range routes {
		registered[r.Pattern] = true
	}
	var missing []string
	for i := range spec.Operations {
		if !registered[spec.Operations[i].Pattern] {
			missing = append(missing, spec.Operations[i].Pattern)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: the operations %v have no route", ErrSpec, missing)
	}
	return nil
}

// checkPermissions fails when the contract names a permission the catalog
// does not know (the lint checks the same at build time).
func checkPermissions(spec *Spec, cat *authz.Catalog) error {
	if cat == nil {
		return nil
	}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		for _, p := range op.Requirement.Permissions {
			if !cat.Has(p) {
				return fmt.Errorf("%w: %s names the permission %q, which is not in the catalog", ErrSpec, op.ID, p)
			}
		}
	}
	return nil
}

// operationChain is the per-operation part of the pipeline (ADR-0052 section 4,
// steps 3 to 6): authentication, principal rate limit, stability gate and
// authorization. Operations that need no authentication skip all of them but
// the stability gate; the per-address limit of the global chain protects them.
func operationChain(deps *Deps, limiter *middleware.Limiter, op *Operation, body middleware.Middleware) []middleware.Middleware {
	// Step 1 of the pipeline: the body limit of the operation, before anything reads the request.
	chain := []middleware.Middleware{withOperation(op), middleware.BodyLimit(BodyLimitFor(op.ID))}
	if !op.Public {
		chain = append(chain,
			middleware.Authenticate(deps.Authenticator, deps.Logger, deps.Auth),
			middleware.LimitPrincipal(limiter, op.Class),
		)
	}
	if op.Stability == "experimental" {
		chain = append(chain, middleware.ExperimentalGate(op.ExperimentalFeature))
	}
	if !op.Public {
		chain = append(chain, middleware.Authorize(deps.Authorizer, deps.Logger, op.Requirement, resourceOf(op)))
	}
	if body != nil {
		// After authorization: a caller who may not call the operation learns nothing of its schema.
		chain = append(chain, body)
	}
	if !op.Public && op.IdempotencyKey != "" {
		chain = append(chain, idempotency.Middleware(idempotency.Config{
			Store: deps.Idempotency, Logger: deps.Logger, Required: op.IdempotencyKey == "required",
			IssuesSecret: op.IssuesSecret, Route: op.Pattern,
		}))
	}
	return chain
}

// resourceOf names the resource of a request after the first path parameter of
// the operation: {flowId} is the resource kind flow with the id in the URL.
func resourceOf(op *Operation) middleware.ResourceOf {
	return func(r *http.Request) authz.ResourceRef {
		if len(op.PathParams) == 0 {
			return authz.ResourceRef{}
		}
		name := op.PathParams[0]
		return authz.ResourceRef{Kind: strings.TrimSuffix(name, "Id"), ID: r.PathValue(name)}
	}
}

// recorder is the router the generated code registers on. It remembers every
// pattern and wraps each handler with the pipeline of its operation.
type recorder struct {
	mux     *http.ServeMux
	ops     map[string]Operation
	wrap    func(*Operation, http.Handler) http.Handler
	routes  []Route
	unknown []string
}

// HandleFunc implements [apigen.ServeMux].
func (rec *recorder) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	op, ok := rec.ops[pattern]
	if !ok {
		rec.unknown = append(rec.unknown, pattern)
		return
	}
	rec.routes = append(rec.routes, Route{Pattern: pattern, Kind: KindOperation})
	rec.mux.Handle(pattern, rec.wrap(&op, http.HandlerFunc(handler)))
}

// ServeHTTP implements [apigen.ServeMux].
func (rec *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) { rec.mux.ServeHTTP(w, r) }

// probeMethods are the methods the fallback tries to find which a path allows.
var probeMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}

// fallback answers every request under /api/ that no route claims: 405
// method-not-allowed with an Allow header when the path exists for other
// methods, 404 not-found when it does not (ERR-6).
func fallback(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allowed []string
		for _, m := range probeMethods {
			probe := r.Clone(r.Context())
			probe.Method = m
			if _, pattern := mux.Handler(probe); pattern != "" && pattern != "/api/" {
				allowed = append(allowed, m)
			}
		}
		if len(allowed) > 0 {
			problem.Write(w, r, problem.MethodNotAllowed, problem.Header("Allow", strings.Join(allowed, ", ")))
			return
		}
		problem.Write(w, r, problem.NotFound)
	}
}

// mountReference registers the reference site and the redirect from
// /api/reference to /api/reference/.
func mountReference(mux *http.ServeMux, site http.Handler) []Route {
	mux.Handle("GET /api/reference", http.HandlerFunc(reference.Redirect))
	mux.Handle("GET /api/reference/", site)
	return []Route{
		{Pattern: "GET /api/reference", Kind: KindReference},
		{Pattern: "GET /api/reference/", Kind: KindReference},
	}
}
