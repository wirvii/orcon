// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"fmt"
	"time"
)

// Class says whether a provider failure may succeed on a later attempt.
// Transient failures can fall back or retry; permanent ones fail the step
// (ADR-0061 §3, ADR-0011).
type Class string

// The failure classes.
const (
	ClassTransient Class = "TRANSIENT"
	ClassPermanent Class = "PERMANENT"
)

// ErrUnsupported reports a request that uses a feature the adapter does not
// implement for the chosen model. It is permanent: retrying changes nothing.
var ErrUnsupported = errors.New("provider feature not supported")

// ErrToolNameCollision reports two declared tools whose sanitized names are
// equal, so the model could not tell them apart.
var ErrToolNameCollision = errors.New("tool names collide after sanitizing")

// Error is a failure the provider answered with, or a transport failure the
// adapter classified. Message never carries the request content or a
// credential; it is the provider's own short text, truncated by the adapter.
type Error struct {
	Class  Class
	Status int
	// Code is the provider's error code or type, for example "rate_limit_exceeded".
	Code string
	// RetryAfter is the delay the provider asked for, zero when it asked for none.
	RetryAfter time.Duration
	Message    string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("model provider %s (status %d, code %q): %s", e.Class, e.Status, e.Code, e.Message)
}

// Classify maps an HTTP status to a failure class. Rate limits, server errors
// and the 529 overload status are transient; every other status is permanent,
// including validation, refusal and credential errors (ADR-0061 §5).
func Classify(status int) Class {
	switch {
	case status == 429, status == 529, status >= 500 && status <= 599:
		return ClassTransient
	default:
		return ClassPermanent
	}
}

// IsTransient reports whether err is a [*Error] of the transient class.
func IsTransient(err error) bool {
	var pe *Error
	return errors.As(err, &pe) && pe.Class == ClassTransient
}
