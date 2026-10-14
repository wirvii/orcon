// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package problem is the one place that knows the HTTP status of an error of
// the public API (API-4): the catalog of problem codes (ADR-0053 section 3) and
// the writer of application/problem+json documents (RFC 9457).
//
// It is an adapter package: the catalog mirrors components/schemas/ProblemCode
// of the contract, and a test in package httpapi compares the two.
package problem

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// MediaType is the media type of every error document.
const MediaType = "application/problem+json"

// RequestIDHeader carries the request id on every response.
const RequestIDHeader = "Orcon-Request-Id"

// TypePrefix starts the type URI of every problem.
const TypePrefix = "urn:orcon:problem:"

// Code is a stable kebab-case problem code.
type Code string

// The problem codes of the catalog (ERR-6). Codes are added only for new
// conditions, in the same change as their entry in the contract.
const (
	ValidationFailed         Code = "validation-failed"
	InvalidPageToken         Code = "invalid-page-token"
	ExperimentalNotEnabled   Code = "experimental-not-enabled"
	Unauthenticated          Code = "unauthenticated"
	Forbidden                Code = "forbidden"
	StepUpRequired           Code = "step-up-required"
	PersonalKeysDisabled     Code = "personal-keys-disabled"
	NotFound                 Code = "not-found"
	MethodNotAllowed         Code = "method-not-allowed"
	Conflict                 Code = "conflict"
	IdempotencyInProgress    Code = "idempotency-in-progress"
	SecretAlreadyIssued      Code = "secret-already-issued"
	AttentionAlreadyResolved Code = "attention-already-resolved"
	ProjectManagedByGit      Code = "project-managed-by-git"
	DeprecatedRemoved        Code = "deprecated-removed"
	PreconditionFailed       Code = "precondition-failed"
	PayloadTooLarge          Code = "payload-too-large"
	UnsupportedMediaType     Code = "unsupported-media-type"
	IdempotencyKeyReused     Code = "idempotency-key-reused"
	PreconditionRequired     Code = "precondition-required"
	RateLimited              Code = "rate-limited"
	InternalError            Code = "internal-error"
	ServiceUnavailable       Code = "service-unavailable"
	ConfirmationRequired     Code = "confirmation-required"
	FederatedTokenRejected   Code = "federated-token-rejected"
	CSRFCheckFailed          Code = "csrf-check-failed"
)

// Spec describes one entry of the catalog.
type Spec struct {
	// Code is the stable identifier.
	Code Code
	// Status is the HTTP status of the code.
	Status int
	// Title is the short summary of the problem type.
	Title string
	// When says when the code applies.
	When string
	// Retryable reports whether repeating the request can succeed.
	Retryable bool
}

var catalog = []Spec{
	{ValidationFailed, http.StatusBadRequest, "Validation failed", "A parameter or the body is invalid.", false},
	{InvalidPageToken, http.StatusBadRequest, "Invalid page token", "The page token is unknown or was issued for other parameters.", false},
	{ExperimentalNotEnabled, http.StatusBadRequest, "Experimental feature not enabled", "The operation is experimental and Orcon-Experimental does not name it.", false},
	{Unauthenticated, http.StatusUnauthorized, "Unauthenticated", "No valid credential was sent.", false},
	{Forbidden, http.StatusForbidden, "Forbidden", "The caller lacks the required permission.", false},
	{StepUpRequired, http.StatusForbidden, "Step-up required", "The operation needs a recent second factor.", false},
	{PersonalKeysDisabled, http.StatusForbidden, "Personal keys disabled", "The organization does not allow personal API keys.", false},
	{NotFound, http.StatusNotFound, "Not found", "The resource does not exist or is not visible to the caller.", false},
	{MethodNotAllowed, http.StatusMethodNotAllowed, "Method not allowed", "The path exists but the method is not one of its operations; the Allow header lists the methods that are.", false},
	{Conflict, http.StatusConflict, "Conflict", "The resource state does not allow the operation.", false},
	{IdempotencyInProgress, http.StatusConflict, "Idempotent request in progress", "A request with the same key is still running.", true},
	{SecretAlreadyIssued, http.StatusConflict, "Secret already issued", "A replay asked again for a secret that is shown only once.", false},
	{AttentionAlreadyResolved, http.StatusConflict, "Attention already resolved", "The human-attention item was already handled.", false},
	{ProjectManagedByGit, http.StatusConflict, "Project managed by Git", "The project is bound to a Git repository so its flows change only through a Git sync.", false},
	{DeprecatedRemoved, http.StatusGone, "Removed", "The operation was removed after its sunset date.", false},
	{PreconditionFailed, http.StatusPreconditionFailed, "Precondition failed", "If-Match does not match the current ETag.", false},
	{PayloadTooLarge, http.StatusRequestEntityTooLarge, "Payload too large", "The body exceeds the size limit.", false},
	{UnsupportedMediaType, http.StatusUnsupportedMediaType, "Unsupported media type", "The body is not in a media type the operation accepts.", false},
	{IdempotencyKeyReused, http.StatusUnprocessableEntity, "Idempotency key reused", "The key was used with a different request.", false},
	{PreconditionRequired, http.StatusPreconditionRequired, "Precondition required", "If-Match is missing.", false},
	{RateLimited, http.StatusTooManyRequests, "Rate limited", "The rate class quota is exhausted.", true},
	{InternalError, http.StatusInternalServerError, "Internal error", "An unexpected server error happened.", true},
	{ServiceUnavailable, http.StatusServiceUnavailable, "Service unavailable", "The server is starting, stopping or overloaded.", true},
	{ConfirmationRequired, http.StatusForbidden, "Confirmation required", "The request was stored as a confirmation that a person must approve before it runs.", false},
	{FederatedTokenRejected, http.StatusBadRequest, "Federated token rejected", "The workload identity token was rejected by the trust relationship; protocol endpoints send it as invalid_grant.", false},
	{CSRFCheckFailed, http.StatusForbidden, "CSRF check failed", "The request did not carry a valid CSRF token or came from another origin.", false},
}

// Catalog returns a copy of the catalog in contract order.
func Catalog() []Spec { return slices.Clone(catalog) }

// Lookup returns the catalog entry of code.
func Lookup(code Code) (Spec, bool) {
	for _, s := range catalog {
		if s.Code == code {
			return s, true
		}
	}
	return Spec{}, false
}

// FieldError is one error of one request field.
type FieldError struct {
	// Pointer is a JSON Pointer into the request.
	Pointer string `json:"pointer"`
	// Code is the stable code of the field error.
	Code string `json:"code"`
	// Message explains the error to a person.
	Message string `json:"message"`
}

// Problem is the error document (RFC 9457 plus the Orcon members).
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	Code      Code         `json:"code"`
	RequestID string       `json:"requestId"`
	Errors    []FieldError `json:"errors,omitempty"`
	// ConfirmationID and ExpiresAt are the extension members of confirmation-required:
	// the stored confirmation and when it lapses.
	ConfirmationID *string `json:"confirmationId,omitempty"`
	ExpiresAt      *string `json:"expiresAt,omitempty"`
}

// Option adjusts one problem response.
type Option func(*response)

type response struct {
	detail       string
	fields       []FieldError
	header       http.Header
	confirmation *string
	expiresAt    *string
}

// Detail sets the explanation of this occurrence. It must never contain secret
// values or data of another organization (ERR-5).
func Detail(text string) Option { return func(r *response) { r.detail = text } }

// Fields sets the per-field errors; every invalid field goes in one response (API-4.2).
func Fields(errs ...FieldError) Option {
	return func(r *response) { r.fields = append(r.fields, errs...) }
}

// Confirmation adds the extension members of confirmation-required: the id of the
// stored confirmation and the instant it lapses.
func Confirmation(id string, expiresAt time.Time) Option {
	return func(r *response) {
		stamp := expiresAt.UTC().Format(time.RFC3339)
		r.confirmation, r.expiresAt = &id, &stamp
	}
}

// RetryAfter adds the Retry-After header, rounded up to whole seconds.
func RetryAfter(d time.Duration) Option {
	return func(r *response) {
		seconds := int64((d + time.Second - 1) / time.Second)
		seconds = max(seconds, 1)
		r.header.Set("Retry-After", strconv.FormatInt(seconds, 10))
	}
}

// Header adds a response header.
func Header(name, value string) Option {
	return func(r *response) { r.header.Set(name, value) }
}

// New builds the problem document of code for request r. The request id is
// the one the response already carries; an unknown code becomes internal-error.
func New(w http.ResponseWriter, r *http.Request, code Code, opts ...Option) (Problem, http.Header) {
	spec, ok := Lookup(code)
	if !ok {
		spec, _ = Lookup(InternalError)
	}
	resp := response{header: http.Header{}}
	for _, opt := range opts {
		opt(&resp)
	}
	p := Problem{
		Type:      TypePrefix + string(spec.Code),
		Title:     spec.Title,
		Status:    spec.Status,
		Detail:    resp.detail,
		Code:      spec.Code,
		RequestID: w.Header().Get(RequestIDHeader),
		Errors:    resp.fields,

		ConfirmationID: resp.confirmation,
		ExpiresAt:      resp.expiresAt,
	}
	if r != nil && r.URL != nil {
		p.Instance = r.URL.Path
	}
	return p, resp.header
}

// Write sends the problem document of code. It is the only way an error
// leaves the API. A failed write to a client that went away is not reported.
func Write(w http.ResponseWriter, r *http.Request, code Code, opts ...Option) {
	p, header := New(w, r, code, opts...)
	body, err := json.Marshal(p)
	if err != nil {
		// A Problem holds only strings and integers: this cannot happen.
		body = []byte(`{"type":"urn:orcon:problem:internal-error","title":"Internal error","status":500,"code":"internal-error","requestId":""}`)
		p.Status = http.StatusInternalServerError
	}
	for name, values := range header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.Header().Set("Content-Type", MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(p.Status)
	if r != nil && r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}
