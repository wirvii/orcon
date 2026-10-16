// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ambiguousWrite reports whether a failure after the request was sent may have
// applied the effect and cannot be repeated blindly: a write with no
// idempotency key (ADR-0079 §4). Writes that carry a key are repeated with the
// same key, and reads are repeated freely.
func ambiguousWrite(op Operation) bool {
	return op.Purity == PurityEffect && (op.Safety == SafetyReconcile || op.Safety == SafetyUnsafe)
}

// actionFor is what the caller must do after an ambiguous or uncertain outcome.
func actionFor(op Operation, class Class) Action {
	switch {
	case class == ClassUncertain:
		return ActionSuspend
	case op.Safety == SafetyReconcile:
		return ActionVerify
	default:
		return ActionNone
	}
}

// classifyTransport maps an error of [Sender.Send]. Errors that prove the
// request never left the host are safe to repeat; anything else may have
// reached the provider.
func classifyTransport(op Operation, err error) *CallError {
	switch {
	case errors.Is(err, ErrEgressDenied):
		return &CallError{Class: ClassPermanent, Code: CodeEgressDenied, Action: ActionNone, Message: "the network policy refused the destination"}
	case errors.Is(err, ErrNotSent):
		return &CallError{Class: ClassTransient, Code: "NetworkError", Action: ActionNone, Message: "the request was not sent"}
	}
	code := "NetworkError"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "Timeout"
	}
	if ambiguousWrite(op) {
		class := ClassTransient
		if op.Safety == SafetyUnsafe {
			class = ClassUncertain
		}
		return &CallError{Class: class, Code: code, Action: actionFor(op, class), Ambiguous: true, Message: "the outcome of the request is unknown"}
	}
	return &CallError{Class: ClassTransient, Code: code, Action: ActionNone, Message: "the request failed before a response"}
}

// classifyResponse returns nil for a 2xx response, and the failure otherwise.
// Explicit error rules of the operation refine the code; the ambiguity of a
// write without key is decided before them, so a rule can never turn an
// unknown outcome of an UNSAFE write into a definite failure.
func classifyResponse(op Operation, resp HTTPResponse) *CallError {
	if resp.Status >= 200 && resp.Status <= 299 {
		return nil
	}
	retryAfter := retryAfterOf(resp)
	if resp.Status >= 500 && ambiguousWrite(op) {
		class := ClassTransient
		if op.Safety == SafetyUnsafe {
			class = ClassUncertain
		}
		return &CallError{
			Class: class, Code: defaultCode(resp.Status), Action: actionFor(op, class),
			Ambiguous: true, Status: resp.Status, RetryAfter: retryAfter,
			Message: "the provider failed after the request was sent",
		}
	}
	for _, rule := range op.Response.Errors {
		if !containsStatus(rule.Status, resp.Status) {
			continue
		}
		code := rule.Code
		if code == "" {
			code = defaultCode(resp.Status)
		}
		class := Class(rule.Class)
		return &CallError{Class: class, Code: code, Action: actionFor(op, class), Status: resp.Status, RetryAfter: retryAfter}
	}
	switch {
	case resp.Status == 429:
		// The request was not applied: transient at every safety level.
		return &CallError{Class: ClassTransient, Code: "RateLimited", Action: ActionNone, Status: resp.Status, RetryAfter: retryAfter}
	case resp.Status >= 500:
		return &CallError{Class: ClassTransient, Code: defaultCode(resp.Status), Action: ActionNone, Status: resp.Status, RetryAfter: retryAfter}
	default:
		return &CallError{Class: ClassPermanent, Code: defaultCode(resp.Status), Action: ActionNone, Status: resp.Status}
	}
}

// defaultCode maps a status without an explicit rule to one of the fourteen
// codes (ADR-0079 §5).
func defaultCode(status int) string {
	switch status {
	case 400, 422:
		return "ValidationFailed"
	case 401:
		return "AuthInvalid"
	case 403:
		return "PermissionDenied"
	case 404:
		return "NotFound"
	case 408, 504:
		return "Timeout"
	case 409:
		return "Conflict"
	case 413:
		return "PayloadTooLarge"
	case 429:
		return "RateLimited"
	case 402:
		return "QuotaExceeded"
	case 503:
		return "ServiceUnavailable"
	default:
		return "ProviderError"
	}
}

func containsStatus(list []int, status int) bool {
	for _, s := range list {
		if s == status {
			return true
		}
	}
	return false
}

// retryAfterOf reads a Retry-After header given in seconds. HTTP dates are not
// read in this cut; the caller then falls back to its own backoff.
func retryAfterOf(resp HTTPResponse) time.Duration {
	for k, vals := range resp.Header {
		if !strings.EqualFold(k, "retry-after") || len(vals) == 0 {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(vals[0])); err == nil && n >= 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 0
}
