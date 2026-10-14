// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/identity"
)

// unavailableRetry is the Retry-After of an operation that has no handler yet.
const unavailableRetry = 30 * time.Second

type operationKey struct{}

// OperationFrom returns the operation a request is being served for.
func OperationFrom(ctx context.Context) (Operation, bool) {
	op, ok := ctx.Value(operationKey{}).(Operation)
	return op, ok
}

// withOperation records the operation in the request context for the rest of
// the pipeline, the handlers and the audit step.
func withOperation(op *Operation) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), operationKey{}, *op)))
		})
	}
}

// errorHandler is the one table that maps the failures of the generated code
// to problem documents (API-4). It is the only place that knows the HTTP status
// of a request that could not be bound.
type errorHandler struct {
	logger *slog.Logger
}

// request handles a request the generated code could not bind: a missing or
// malformed parameter, a body that does not decode, a body over the limit.
func (h errorHandler) request(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		problem.Write(w, r, problem.PayloadTooLarge)
		return
	}
	var missingHeader *apigen.RequiredHeaderError
	if errors.As(err, &missingHeader) && strings.EqualFold(missingHeader.ParamName, "If-Match") {
		problem.Write(w, r, problem.PreconditionRequired)
		return
	}
	if field, ok := invalidField(err); ok {
		problem.Write(w, r, problem.ValidationFailed, problem.Fields(field))
		return
	}
	// Anything else in the binding is a malformed request, not a server fault.
	problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{
		Pointer: "", Code: "malformed-request", Message: "The request could not be read.",
	}))
}

// response handles an error returned by a strict handler. The error is not
// in the table of area errors yet, so it is an internal error: logged once
// with the request id, never shown to the client (API-4.1).
func (h errorHandler) response(w http.ResponseWriter, r *http.Request, err error) {
	if answered := knownError(w, r, err); answered {
		return
	}
	var refused *identity.SignInError
	switch {
	case errors.As(err, &refused) && refused.Reason == identity.ReasonThrottled:
		// The wait is the same whether or not the account exists (SPEC-004 R-AUTH-02).
		problem.Write(w, r, problem.RateLimited, problem.RetryAfter(refused.RetryAfter))
	case errors.Is(err, identity.ErrSignInError), errors.Is(err, identity.ErrUnauthenticated):
		// Every other refusal of a sign-in looks like this one: no reason reaches the client.
		w.Header().Set("WWW-Authenticate", "Bearer")
		problem.Write(w, r, problem.Unauthenticated)
	case errors.Is(err, identity.ErrCSRFCheckFailed):
		problem.Write(w, r, problem.CSRFCheckFailed)
	case errors.Is(err, identity.ErrNotFound):
		problem.Write(w, r, problem.NotFound)
	case errors.Is(err, identity.ErrInvalidInput):
		problem.Write(w, r, problem.ValidationFailed)
	case errors.Is(err, errInvalidPageToken):
		problem.Write(w, r, problem.InvalidPageToken)
	default:
		h.logger.ErrorContext(r.Context(), "request failed",
			"request_id", middleware.RequestIDFrom(r.Context()), "error", err)
		problem.Write(w, r, problem.InternalError)
	}
}

// knownError answers the errors that handlers share: an operation without a
// handler, the pagination errors and the failed preconditions. It reports
// whether it answered.
func knownError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, errUnavailable):
		problem.Write(w, r, problem.ServiceUnavailable, problem.RetryAfter(unavailableRetry))
	case errors.Is(err, page.ErrInvalidToken):
		problem.Write(w, r, problem.InvalidPageToken)
	case errors.Is(err, page.ErrInvalidLimit):
		problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{Pointer: "/limit", Code: "out-of-range", Message: "The limit is an integer from 1 to 200."}))
	case errors.Is(err, page.ErrInvalidSort):
		problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{Pointer: "/sort", Code: "not-allowed", Message: "The sort expression names a field this operation does not sort by."}))
	case errors.Is(err, conditional.ErrRequired):
		problem.Write(w, r, problem.PreconditionRequired)
	case errors.Is(err, conditional.ErrMismatch), errors.Is(err, conditional.ErrMalformed):
		problem.Write(w, r, problem.PreconditionFailed)
	default:
		return false
	}
	return true
}

// invalidField describes the parameter the generated code rejected. Messages
// name the parameter and the rule, never the value sent.
func invalidField(err error) (problem.FieldError, bool) {
	var required *apigen.RequiredParamError
	var requiredHeader *apigen.RequiredHeaderError
	var format *apigen.InvalidParamFormatError
	var unmarshal *apigen.UnmarshalingParamError
	var many *apigen.TooManyValuesForParamError
	switch {
	case errors.As(err, &required):
		return problem.FieldError{Pointer: "/" + required.ParamName, Code: "required", Message: "The parameter is required."}, true
	case errors.As(err, &requiredHeader):
		return problem.FieldError{Pointer: "/" + requiredHeader.ParamName, Code: "required", Message: "The header is required."}, true
	case errors.As(err, &format):
		return problem.FieldError{Pointer: "/" + format.ParamName, Code: "invalid-format", Message: "The parameter has an invalid format."}, true
	case errors.As(err, &unmarshal):
		return problem.FieldError{Pointer: "/" + unmarshal.ParamName, Code: "invalid-format", Message: "The parameter has an invalid format."}, true
	case errors.As(err, &many):
		return problem.FieldError{Pointer: "/" + many.ParamName, Code: "too-many-values", Message: "The parameter must be sent once."}, true
	default:
		return problem.FieldError{}, false
	}
}
