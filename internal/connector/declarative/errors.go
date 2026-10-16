// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Class is the outcome class of a failed call (ADR-0030 §5, ADR-0079 §4).
type Class string

// Classes. UNCERTAIN means the effect may or may not have happened and a
// person or a verification decides; it is only ever produced for EFFECT
// operations.
const (
	ClassTransient Class = "TRANSIENT"
	ClassPermanent Class = "PERMANENT"
	ClassUncertain Class = "UNCERTAIN"
)

// Classes returns every valid [Class].
func Classes() []Class { return []Class{ClassTransient, ClassPermanent, ClassUncertain} }

// Codes is the closed list of fourteen provider error codes (ADR-0079 §5).
// Host-raised codes such as EgressDenied are not in this list.
var Codes = []string{
	"AuthInvalid", "AuthExpired", "PermissionDenied", "NotFound", "Conflict",
	"ValidationFailed", "RateLimited", "QuotaExceeded", "PayloadTooLarge",
	"ServiceUnavailable", "Timeout", "NetworkError", "Unsupported", "ProviderError",
}

// CodeEgressDenied is the host-raised code of a refused destination.
const CodeEgressDenied = "EgressDenied"

// IsCode reports whether s is one of the fourteen codes of [Codes].
func IsCode(s string) bool { return slices.Contains(Codes, s) }

// Action tells the caller what must happen next after a failure.
type Action string

// Actions. ActionNone: the caller may apply its normal policy. ActionVerify:
// the operation is RECONCILE and must be verified before anything is sent
// again. ActionSuspend: the outcome is UNCERTAIN and a person decides; nothing
// is resent.
const (
	ActionNone    Action = "NONE"
	ActionVerify  Action = "VERIFY"
	ActionSuspend Action = "SUSPEND"
)

// CallError is the error of a call that did not succeed. It carries no secret:
// Message is a fixed phrase, and the status and code are provider metadata.
type CallError struct {
	Class  Class
	Code   string
	Action Action
	// Ambiguous is true when the request may have been applied.
	Ambiguous bool
	// Status is the HTTP status, or zero when no response arrived.
	Status int
	// RetryAfter is the delay the provider asked for (429 and 503), or zero.
	RetryAfter time.Duration
	Message    string
}

// Error implements error.
func (f *CallError) Error() string {
	msg := fmt.Sprintf("%s %s", f.Class, f.Code)
	if f.Status != 0 {
		msg += fmt.Sprintf(" (HTTP %d)", f.Status)
	}
	if f.Message != "" {
		msg += ": " + f.Message
	}
	return msg
}

// ErrNotSent wraps a Sender error when the request was certainly not
// transmitted (for example a name that did not resolve). Any other Sender error
// means the request may have reached the provider.
var ErrNotSent = errors.New("declarative: request not sent")

// ErrEgressDenied wraps a Sender error caused by the network policy. The
// request was not sent.
var ErrEgressDenied = errors.New("declarative: egress denied")
